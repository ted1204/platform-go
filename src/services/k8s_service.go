package services

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"

	"github.com/linskybing/platform-go/src/config"
	"github.com/linskybing/platform-go/src/dto"
	"github.com/linskybing/platform-go/src/k8sclient"
	"github.com/linskybing/platform-go/src/models"
	"github.com/linskybing/platform-go/src/repositories"
	"github.com/linskybing/platform-go/src/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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
	annotations := make(map[string]string)

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

					// Handle Dedicated on Shared Node (Emulation)
					if requestedType == "dedicated" {
						input.GPUType = "shared"
						input.GPUCount = input.GPUCount * 10

						// Set MPS limits to Max for "dedicated" usage via Annotations
						annotations["mps.nvidia.com/threads"] = "100"
						annotations["mps.nvidia.com/vram"] = "48000M"
					}

					// Inject MPS Annotations if shared
					if requestedType == "shared" {
						if project.MPSLimit > 0 {
							annotations["mps.nvidia.com/threads"] = strconv.Itoa(project.MPSLimit)
						}
						if project.MPSMemory > 0 {
							annotations["mps.nvidia.com/vram"] = fmt.Sprintf("%dM", project.MPSMemory)
						}
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
		Annotations:       annotations,
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
	if err := s.repos.Job.Create(&jobRecord); err != nil {
		// Note: Job is created in K8s but DB record failed.
		// In a real system, we might want to rollback or handle this inconsistency.
		return err
	}

	return nil
}

func (s *K8sService) ListJobs(userID uint, isAdmin bool) ([]models.Job, error) {
	if isAdmin {
		return s.repos.Job.FindAll()
	}
	return s.repos.Job.FindByUserID(userID)
}

func (s *K8sService) GetJob(id uint) (*models.Job, error) {
	return s.repos.Job.FindByID(id)
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

// StartFileBrowser provisions a FileBrowser instance with specific access permissions.
func (s *K8sService) StartFileBrowser(ctx context.Context, ns, pvcName string, readOnly bool, baseURL string) (string, error) {
	// 1. Create Pod with dynamic read-only configuration
	// Note: You need to update CreateFileBrowserPod to accept the readOnly boolean.
	_, err := k8sclient.CreateFileBrowserPod(ctx, ns, pvcName, readOnly, baseURL)
	if err != nil {
		return "", err
	}

	// 2. Create Service
	nodePort, err := k8sclient.CreateFileBrowserService(ctx, ns, pvcName)
	if err != nil {
		return "", err
	}

	return nodePort, nil
}

func (s *K8sService) StopFileBrowser(ctx context.Context, ns, pvcName string) error {
	return k8sclient.DeleteFileBrowserResources(ctx, ns, pvcName)
}

func (s *K8sService) CheckUserStorageExists(ctx context.Context, username string) (bool, error) {
	safeUser := strings.ToLower(username)
	nsName := fmt.Sprintf("user-%s-storage", safeUser)

	return utils.CheckNamespaceExists(nsName)
}

// InitializeUserStorageHub orchestrates the creation of a per-user storage hub.
// Architecture: Hub-and-Spoke
// 1. Namespace: Dedicated namespace for the user's storage infrastructure.
// 2. PVC: A "Real" Longhorn RWO volume (Thin Provisioned).
// 3. Deployment: An NFS Server pod mounting the Longhorn volume.
// 4. Service: A stable ClusterIP to act as the gateway for future projects.
func (s *K8sService) InitializeUserStorageHub(username string) error {
	// 1. Sanitize Username for K8s compliance
	// K8s resources must consist of lowercase alphanumeric characters or '-'.
	// We replace underscores with hyphens and remove other special chars.
	safeUser := strings.ToLower(username)
	reg, err := regexp.Compile("[^a-z0-9-]+")
	if err == nil {
		safeUser = reg.ReplaceAllString(safeUser, "-")
	}

	// Define resource names
	nsName := fmt.Sprintf("user-%s-storage", safeUser)
	pvcName := fmt.Sprintf("user-%s-disk", safeUser)

	log.Printf("[StorageHub] Initializing storage hub for user: %s (ns: %s)", username, nsName)

	// 2. Create Namespace
	// We ignore the error if it implies "AlreadyExists" inside the utils,
	// but here we just check if it failed critically.
	if err := utils.CreateNamespace(nsName); err != nil {
		// In a real scenario, you might want to check if err is "AlreadyExists" and proceed.
		// Assuming utils handles logging, we just log and continue or return based on policy.
		log.Printf("[StorageHub] Namespace creation warning: %v", err)
	}

	// 3. Create the Hub PVC (The "Real" Volume)
	// StorageClass: "longhorn" (Must match your cluster's SC name)
	// Size: "50Gi" (This is a thin-provisioned limit, actual usage grows on demand)
	// Mode: ReadWriteOnce (Handled inside utils.CreateHubPVC)
	if err := utils.CreateHubPVC(nsName, pvcName, config.DefaultStorageClassName, config.UserPVSize); err != nil {
		return fmt.Errorf("failed to create hub pvc: %w", err)
	}

	// 4. Deploy NFS Server
	// This pod mounts the pvcName created above.
	if err := utils.CreateNFSDeployment(nsName, pvcName); err != nil {
		return fmt.Errorf("failed to create nfs deployment: %w", err)
	}

	// 5. Expose NFS Service (Gateway)
	// This creates the DNS entry: storage-svc.user-<safeUser>-storage.svc.cluster.local
	if err := utils.CreateNFSService(nsName); err != nil {
		return fmt.Errorf("failed to create nfs service: %w", err)
	}

	log.Printf("[StorageHub] Successfully initialized storage hub for %s", username)
	return nil
}

func (s *K8sService) ExpandUserStorageHub(username, newSize string) error {
	safeUser := strings.ToLower(username)
	nsName := fmt.Sprintf("user-%s-storage", safeUser)
	pvcName := fmt.Sprintf("user-%s-disk", safeUser)

	return utils.ExpandPVC(nsName, pvcName, newSize)
}

// DeleteUserStorageHub completely removes a user's storage infrastructure.
// It deletes the dedicated namespace, which automatically cleans up the PVC, NFS Server, and Services inside it.
func (s *K8sService) DeleteUserStorageHub(ctx context.Context, username string) error {
	// 1. Sanitize username to match the naming convention used during initialization.
	// Ensure this matches the logic in InitializeUserStorageHub.
	safeUser := strings.ToLower(username)
	// safeUser = regexp.MustCompile("[^a-z0-9-]").ReplaceAllString(safeUser, "-")

	// 2. Define the namespace name.
	nsName := fmt.Sprintf("user-%s-storage", safeUser)

	// 3. Delete the entire namespace.
	// This is the cleanest way to decommission a user's storage hub.
	// It ensures no orphaned resources (like PVCs or Pods) are left behind.
	if err := utils.DeleteNamespace(nsName); err != nil {
		return fmt.Errorf("failed to delete user storage namespace '%s': %w", nsName, err)
	}

	return nil
}

func (s *K8sService) OpenUserGlobalFileBrowser(ctx context.Context, username string) (string, error) {

	safeUser := strings.ToLower(username)
	port, err := utils.StartUserHubBrowser(ctx, safeUser)
	if err != nil {
		return "", err
	}

	return port, nil
}

func (s *K8sService) StopUserGlobalFileBrowser(ctx context.Context, username string) error {
	safeUser := strings.ToLower(username)
	// safeUser = regexp.MustCompile("[^a-z0-9-]").ReplaceAllString(safeUser, "-")
	return utils.StopUserHubBrowser(ctx, safeUser)
}

func (s *K8sService) CreateProjectPVC(ctx context.Context, req dto.CreateProjectStorageRequest) (*corev1.PersistentVolumeClaim, error) {

	// 1. Generate Safe Name (Using Utils)
	// 呼叫工具函式生成唯一且合法的 Namespace 名稱
	targetNamespace := utils.GenerateSafeResourceName("project", req.ProjectName, req.ProjectID)

	// PVC name convention
	pvcName := fmt.Sprintf("pvc-%s", targetNamespace)

	// 2. Prepare Labels
	// 準備 Namespace 標籤
	nsLabels := map[string]string{
		"managed-by":   "nthucscc",
		"type":         "project-space",
		"project-id":   fmt.Sprintf("%d", req.ProjectID),
		"project-name": req.ProjectName,
	}

	// 3. Ensure Namespace Exists
	if err := s.ensureNamespaceWithLabels(ctx, targetNamespace, nsLabels); err != nil {
		return nil, fmt.Errorf("failed to ensure namespace: %v", err)
	}

	// 4. Parse Capacity
	storageQty, err := resource.ParseQuantity(fmt.Sprintf("%dGi", req.Capacity))
	if err != nil {
		return nil, fmt.Errorf("invalid capacity: %v", err)
	}

	// 5. Prepare PVC Labels (Critical for Filtering)
	// 準備 PVC 標籤，這對於 List API 的過濾至關重要
	pvcLabels := map[string]string{
		"app.kubernetes.io/name":       "filebrowser-storage",
		"app.kubernetes.io/managed-by": "nthu-cscc",
		"storage-type":                 "project",                        // 核心過濾鍵
		"project-id":                   fmt.Sprintf("%d", req.ProjectID), // 唯一識別
		"project-name":                 req.ProjectName,
	}

	// Config
	scName := config.DefaultStorageClassName // 根據你的 Cluster 環境修改 (例如: "nfs-client", "longhorn")
	accessMode := corev1.ReadWriteMany

	// 6. Create PVC Object
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: targetNamespace,
			Labels:    pvcLabels,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{accessMode},

			// [FIXED] Updated for newer client-go versions (v1.30+)
			// 使用 VolumeResourceRequirements 取代舊的 ResourceRequirements
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: storageQty,
				},
			},

			StorageClassName: &scName,
		},
	}

	return k8sclient.Clientset.CoreV1().PersistentVolumeClaims(targetNamespace).Create(ctx, pvc, metav1.CreateOptions{})
}

// ensureNamespaceWithLabels checks if a namespace exists, creates it if not.
// It's a private helper method for the service.
func (s *K8sService) ensureNamespaceWithLabels(ctx context.Context, name string, labels map[string]string) error {
	_, err := k8sclient.Clientset.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		// Namespace exists.
		// Future improvement: We could update labels here if they changed.
		return nil
	}

	// Create new Namespace
	newNs := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
	}
	_, err = k8sclient.Clientset.CoreV1().Namespaces().Create(ctx, newNs, metav1.CreateOptions{})
	return err
}

// ListAllProjectStorages retrieves all project-related PVCs using LabelSelectors.
func (s *K8sService) ListAllProjectStorages(ctx context.Context) ([]dto.ProjectPVCOutput, error) {
	// 1. Define Filter Options.
	// We use server-side filtering to only fetch PVCs relevant to projects.
	listOptions := metav1.ListOptions{
		LabelSelector: "storage-type=project,app.kubernetes.io/managed-by=nthu-cscc",
	}

	// 2. List PVCs from ALL namespaces.
	pvcs, err := k8sclient.Clientset.CoreV1().PersistentVolumeClaims(metav1.NamespaceAll).List(ctx, listOptions)
	if err != nil {
		return nil, err
	}

	var result []dto.ProjectPVCOutput

	// 3. Map Kubernetes resources to DTOs.
	for _, pvc := range pvcs.Items {
		projectID := pvc.Labels["project-id"]
		projectName := pvc.Labels["project-name"]

		// Skip if essential labels are missing (integrity check)
		if projectID == "" {
			continue
		}

		capacityStr := pvc.Spec.Resources.Requests[corev1.ResourceStorage]

		// Safe check for AccessModes
		accessMode := ""
		if len(pvc.Spec.AccessModes) > 0 {
			accessMode = string(pvc.Spec.AccessModes[0])
		}

		result = append(result, dto.ProjectPVCOutput{
			ID:          projectID,
			PVCName:     pvc.Name,
			ProjectName: projectName,
			Namespace:   pvc.Namespace,
			Capacity:    capacityStr.String(),
			Status:      string(pvc.Status.Phase),
			AccessMode:  accessMode,
			CreatedAt:   pvc.CreationTimestamp.Time,
		})
	}

	return result, nil
}
