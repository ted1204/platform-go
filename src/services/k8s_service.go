package services

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/linskybing/platform-go/src/db"
	"github.com/linskybing/platform-go/src/dto"
	"github.com/linskybing/platform-go/src/k8sclient"
	"github.com/linskybing/platform-go/src/models"
	"github.com/linskybing/platform-go/src/repositories"
	"github.com/linskybing/platform-go/src/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	envVars := make(map[string]string)

	// Check GPU Quota and Access
	if input.GPUCount > 0 {
		// Parse Project ID from Namespace (format: pid-username)
		parts := strings.Split(input.Namespace, "-")
		if len(parts) >= 2 {
			pidStr := parts[0]
			pid, err := strconv.Atoi(pidStr)
			if err == nil {
				project, err := s.repos.Project.GetProjectByID(uint(pid))
				if err == nil {
					// Check Access Type
					allowedTypes := strings.Split(project.GPUAccess, ",")
					isAllowed := false
					requestedType := input.GPUType
					if requestedType == "" {
						requestedType = "dedicated" // Default to dedicated if not specified
					}

					for _, t := range allowedTypes {
						if strings.TrimSpace(t) == requestedType {
							isAllowed = true
							break
						}
					}

					if !isAllowed {
						return fmt.Errorf("GPU access type '%s' is not allowed for this project. Allowed: %s", requestedType, project.GPUAccess)
					}

					// Check Quota
					currentUsage, err := s.CountProjectGPUUsage(ctx, uint(pid))
					if err != nil {
						return err
					}
					
					// Calculate requested quota units
					requestedUnits := input.GPUCount
					if requestedType == "dedicated" {
						// Assuming 1 dedicated GPU = 10 shared units
						requestedUnits = input.GPUCount * 10
					}

					if currentUsage+requestedUnits > project.GPUQuota {
						return fmt.Errorf("GPU quota exceeded. Current: %d, Requested: %d, Quota: %d", currentUsage, requestedUnits, project.GPUQuota)
					}

					// Inject MPS Env Vars and Volumes if shared
					if requestedType == "shared" {
						if project.MPSLimit > 0 {
							envVars["CUDA_MPS_ACTIVE_THREAD_PERCENTAGE"] = strconv.Itoa(project.MPSLimit)
						}
						if project.MPSMemory > 0 {
							envVars["CUDA_MPS_PINNED_DEVICE_MEM_LIMIT"] = fmt.Sprintf("%dM", project.MPSMemory)
						}
						
						// Set MPS Pipe Directory
						envVars["CUDA_MPS_PIPE_DIRECTORY"] = "/tmp/nvidia-mps"

						// Mount MPS Pipe and SHM
						volumes = append(volumes, k8sclient.VolumeSpec{
							Name:      "nvidia-mps",
							HostPath:  "/run/nvidia/mps",
							MountPath: "/tmp/nvidia-mps",
						})
						volumes = append(volumes, k8sclient.VolumeSpec{
							Name:      "nvidia-mps-shm",
							HostPath:  "/run/nvidia/mps/shm",
							MountPath: "/dev/shm",
						})
					}

					// Handle Dedicated on Shared Node (Emulation)
					if requestedType == "dedicated" {
						input.GPUType = "shared"
						input.GPUCount = input.GPUCount * 10
					}
				}
			}
		}
	}

	// Determine PriorityClassName
	priorityClassName := "low-priority"
	// Force low priority for now as per requirement
	// if input.Priority == "high" {
	// 	priorityClassName = "high-priority"
	// }

	spec := k8sclient.JobSpec{
		Name:              input.Name,
		Namespace:         input.Namespace,
		Image:             input.Image,
		Command:           input.Command,
		PriorityClassName: priorityClassName,
		Parallelism:       input.Parallelism,
		Completions:       input.Completions,
		Volumes:           volumes,
		GPUCount:          input.GPUCount,
		GPUType:           input.GPUType,
		EnvVars:           envVars,
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
		Priority:   "low", // Force low priority in DB record
		Status:     "Pending",
	}
	if err := db.DB.Create(&jobRecord).Error; err != nil {
		// Note: Job is created in K8s but DB record failed.
		// In a real system, we might want to rollback or handle this inconsistency.
		return err
	}

	return nil
}

func (s *K8sService) CountProjectGPUUsage(ctx context.Context, projectID uint) (int, error) {
	namespaces, err := k8sclient.GetFilteredNamespaces(fmt.Sprintf("%d-", projectID))
	if err != nil {
		return 0, err
	}

	totalGPU := 0
	for _, ns := range namespaces {
		pods, err := k8sclient.Clientset.CoreV1().Pods(ns.Name).List(ctx, metav1.ListOptions{})
		if err != nil {
			continue
		}
		for _, pod := range pods.Items {
			// Check if pod is Running or Pending
			if pod.Status.Phase == corev1.PodRunning || pod.Status.Phase == corev1.PodPending {
				for _, container := range pod.Spec.Containers {
					if qty, ok := container.Resources.Requests["nvidia.com/gpu"]; ok {
						val, _ := qty.AsInt64()
						totalGPU += int(val)
					}
					if qty, ok := container.Resources.Requests["nvidia.com/gpu.shared"]; ok {
						val, _ := qty.AsInt64()
						totalGPU += int(val)
					}
				}
			}
		}
	}
	return totalGPU, nil
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
