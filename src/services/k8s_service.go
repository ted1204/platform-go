package services

import (
	"context"

	"github.com/linskybing/platform-go/src/db"
	"github.com/linskybing/platform-go/src/dto"
	"github.com/linskybing/platform-go/src/k8sclient"
	"github.com/linskybing/platform-go/src/models"
	"github.com/linskybing/platform-go/src/repositories"
	"github.com/linskybing/platform-go/src/utils"
	corev1 "k8s.io/api/core/v1"
)

type K8sService struct {
	repos *repositories.Repos
}

func NewK8sService(repos *repositories.Repos) *K8sService {
	return &K8sService{repos: repos}
}

func (s *K8sService) CreateJob(ctx context.Context, userID uint, input dto.CreateJobDTO) error {
	var volumes []k8sclient.VolumeSpec
	for _, v := range input.Volumes {
		volumes = append(volumes, k8sclient.VolumeSpec{
			Name:      v.Name,
			PVCName:   v.PVCName,
			MountPath: v.MountPath,
		})
	}

	spec := k8sclient.JobSpec{
		Name:        input.Name,
		Namespace:   input.Namespace,
		Image:       input.Image,
		Command:     input.Command,
		Parallelism: input.Parallelism,
		Completions: input.Completions,
		Volumes:     volumes,
	}

	// Default values if not provided
	if spec.Parallelism == 0 {
		spec.Parallelism = 1
	}
	if spec.Completions == 0 {
		spec.Completions = 1
	}

	if err := k8sclient.CreateJob(ctx, spec); err != nil {
		return err
	}

	// Record job in database
	jobRecord := models.Job{
		UserID:     userID,
		Name:       input.Name,
		Namespace:  input.Namespace,
		Image:      input.Image,
		K8sJobName: input.Name,
		Status:     "Pending",
	}
	if err := db.DB.Create(&jobRecord).Error; err != nil {
		// Note: Job is created in K8s but DB record failed.
		// In a real system, we might want to rollback or handle this inconsistency.
		return err
	}

	return nil
}

func (s *K8sService) GetPVC(ns, name string) (*corev1.PersistentVolumeClaim, error) {
	return utils.GetPVC(ns, name)
}

func (s *K8sService) ListPVCs(ns string) ([]corev1.PersistentVolumeClaim, error) {
	return utils.ListPVCs(ns)
}

func (s *K8sService) CreatePVC(input dto.CreatePVCDTO) error {
	return utils.CreatePVC(input.Namespace, input.Name, input.StorageClassName, input.Size)
}

func (s *K8sService) ExpandPVC(input dto.ExpandPVCDTO) error {
	return utils.ExpandPVC(input.Namespace, input.Name, input.Size)
}

func (s *K8sService) DeletePVC(ns, name string) error {
	return utils.DeletePVC(ns, name)
}

func (s *K8sService) StartFileBrowser(ctx context.Context, ns, pvcName string) (string, error) {
	// 1. Create Pod
	_, err := k8sclient.CreateFileBrowserPod(ctx, ns, pvcName)
	if err != nil {
		return "", err
	}

	// 2. Create Service
	nodePort, err := k8sclient.CreateFileBrowserService(ctx, ns, pvcName)
	if err != nil {
		return "", err
	}

	// Return access URL (assuming node IP is known or handled by frontend)
	return nodePort, nil
}

func (s *K8sService) StopFileBrowser(ctx context.Context, ns, pvcName string) error {
	return k8sclient.DeleteFileBrowserResources(ctx, ns, pvcName)
}
