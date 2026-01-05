package application

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"

	"github.com/linskybing/platform-go/internal/config"
	"github.com/linskybing/platform-go/internal/domain/job"
	"github.com/linskybing/platform-go/internal/domain/project"
	"github.com/linskybing/platform-go/internal/repository"
	"github.com/linskybing/platform-go/pkg/k8s"
	"github.com/linskybing/platform-go/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type K8sService struct {
	repos        *repository.Repos
	imageService *ImageService
}

func NewK8sService(repos *repository.Repos) *K8sService {
	return &K8sService{
		repos:        repos,
		imageService: NewImageService(repos.Image),
	}
}

func (s *K8sService) CreateJob(ctx context.Context, userID uint, input job.JobSubmission) error {
	// Get user for admin check
	user, err := s.repos.User.GetUserByID(userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	// Extract image name and tag
	imageParts := strings.Split(input.Image, ":")
	if len(imageParts) != 2 {
		return fmt.Errorf("invalid image format, expected name:tag")
	}
	imageName := imageParts[0]
	imageTag := imageParts[1]

	// Parse Project ID from Namespace (format: pid-username)
	parts := strings.Split(input.Namespace, "-")
	var projectID uint
	if len(parts) >= 2 {
		pidStr := parts[0]
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			return fmt.Errorf("invalid namespace format: %w", err)
		}
		projectID = uint(pid)
	} else {
		return fmt.Errorf("invalid namespace format, expected pid-username")
	}

	// Validate image against allowed list (skip for admin)
	if !user.IsSuperAdmin {
		isAllowed, err := s.imageService.ValidateImageForProject(imageName, imageTag, projectID)
		if err != nil || !isAllowed {
			return fmt.Errorf("image %s:%s is not allowed. Please request approval first", imageName, imageTag)
		}
	}

	// Convert input volumes to k8s.VolumeSpec
	var volumes []k8s.VolumeSpec
	for _, v := range input.Volumes {
		volumes = append(volumes, k8s.VolumeSpec{
			Name:      v.Name,
			PVCName:   v.PVCName,
			MountPath: v.MountPath,
		})
	}

	envVars := make(map[string]string)
	annotations := make(map[string]string)

	// Check GPU Quota and Access
	if input.GPUCount > 0 {
		// Use the already parsed projectID
		project, err := s.repos.Project.GetProjectByID(projectID)
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
			currentUsage, err := s.CountProjectGPUUsage(ctx, projectID)
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
				if project.GPUQuota > 0 {
					// System will auto-inject CUDA_MPS_ACTIVE_THREAD_PERCENTAGE
					// Use GPU quota as a reference for MPS configuration
					annotations["gpu.quota"] = strconv.Itoa(project.GPUQuota)
				}
				if project.MPSMemory > 0 {
					annotations["mps.nvidia.com/vram"] = fmt.Sprintf("%dM", project.MPSMemory)
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

	spec := k8s.JobSpec{
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

	jobRecord := job.Job{
		UserID:     userID,
		Name:       input.Name,
		Namespace:  input.Namespace,
		Image:      input.Image,
		K8sJobName: input.Name,
		Priority:   "low", // Force low priority in DB record
		Status:     "Pending",
	}

	// Skip K8s creation when no client is configured (tests); still record DB entry.
	if k8s.Clientset == nil {
		return s.repos.Job.Create(&jobRecord)
	}

	if err := k8s.CreateJob(ctx, spec); err != nil {
		return err
	}

	// Record job in database
	if err := s.repos.Job.Create(&jobRecord); err != nil {
		// Note: Job is created in K8s but DB record failed.
		// In a real system, we might want to rollback or handle this inconsistency.
		return err
	}

	return nil
}

func (s *K8sService) ListJobs(userID uint, isAdmin bool) ([]job.Job, error) {
	if isAdmin {
		return s.repos.Job.FindAll()
	}
	return s.repos.Job.FindByUserID(userID)
}

func (s *K8sService) GetJob(id uint) (*job.Job, error) {
	return s.repos.Job.FindByID(id)
}

func (s *K8sService) CountProjectGPUUsage(ctx context.Context, projectID uint) (int, error) {
	namespaces, err := k8s.GetFilteredNamespaces(fmt.Sprintf("%d-", projectID))
	if err != nil {
		return 0, err
	}

	totalGPU := 0
	for _, ns := range namespaces {
		pods, err := k8s.Clientset.CoreV1().Pods(ns.Name).List(ctx, metav1.ListOptions{})
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

// GetProjectPVCNames returns PVC names within a namespace that are tagged as project storage.
// Falls back to all PVCs if no labeled ones found for backward compatibility.
func (s *K8sService) GetProjectPVCNames(ctx context.Context, namespace string) ([]string, error) {
	if k8s.Clientset == nil {
		return []string{}, nil
	}

	// First try to find PVCs with the project storage label
	list, err := k8s.Clientset.CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "storage-type=project",
	})
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(list.Items))
	for _, pvc := range list.Items {
		names = append(names, pvc.Name)
	}

	// If no labeled PVCs found, fall back to all PVCs in namespace for backward compatibility
	if len(names) == 0 {
		fallback, err := k8s.Clientset.CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{})
		if err == nil && len(fallback.Items) > 0 {
			names = make([]string, 0, len(fallback.Items))
			for _, pvc := range fallback.Items {
				names = append(names, pvc.Name)
			}
		}
	}

	return names, nil
}

func (s *K8sService) CreatePVC(input job.VolumeSpec) error {
	return utils.CreatePVC(input.Namespace, input.Name, input.StorageClassName, input.Size)
}

func (s *K8sService) ExpandPVC(input job.VolumeSpec) error {
	return utils.ExpandPVC(input.Namespace, input.Name, input.Size)
}

func (s *K8sService) DeletePVC(ns, name string) error {
	return utils.DeletePVC(ns, name)
}

// StartFileBrowser provisions a FileBrowser instance with specific access permissions.
// It mounts all provided PVCs under /srv/<pvcName>.
func (s *K8sService) StartFileBrowser(ctx context.Context, ns string, pvcNames []string, readOnly bool, baseURL string) (string, error) {
	if len(pvcNames) == 0 {
		return "", fmt.Errorf("no PVCs available to start filebrowser")
	}

	// 1. Create Pod with dynamic read-only configuration
	_, err := k8s.CreateFileBrowserPod(ctx, ns, pvcNames, readOnly, baseURL)
	if err != nil {
		return "", err
	}

	// 2. Create Service
	nodePort, err := k8s.CreateFileBrowserService(ctx, ns)
	if err != nil {
		return "", err
	}

	return nodePort, nil
}

// EnsureProjectHub creates/ensures the project-level NFS gateway (namespace, PVC, deployment, service).
func (s *K8sService) EnsureProjectHub(p *project.Project) error {
	// Derive names
	ns := utils.GenerateSafeResourceName("project", p.ProjectName, p.PID)
	pvcName := fmt.Sprintf("project-%d-disk", p.PID)

	// Namespace
	if err := utils.CreateNamespace(ns); err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "already exist") {
			return fmt.Errorf("failed to ensure project namespace: %w", err)
		}
	}

	// NFS Deployment
	if err := utils.CreateNFSDeployment(ns, pvcName); err != nil {
		return fmt.Errorf("failed to ensure project nfs deployment: %w", err)
	}

	// NFS Service (ClusterIP with NFS ports)
	if err := utils.CreateNFSServiceWithName(ns, config.ProjectNfsServiceName); err != nil {
		return fmt.Errorf("failed to ensure project nfs service: %w", err)
	}

	return nil
}

func (s *K8sService) StopFileBrowser(ctx context.Context, ns string) error {
	return k8s.DeleteFileBrowserResources(ctx, ns)
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

func (s *K8sService) CreateProjectPVC(ctx context.Context, req job.VolumeSpec) (*corev1.PersistentVolumeClaim, error) {
	// If no Kubernetes client is configured, short-circuit for tests.
	if k8s.Clientset == nil {
		return &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mock-pvc",
				Namespace: utils.GenerateSafeResourceName("project", req.ProjectName, req.ProjectID),
			},
		}, nil
	}

	// 1. Generate Safe Name (Using Utils)
	targetNamespace := utils.GenerateSafeResourceName("project", req.ProjectName, req.ProjectID)

	// PVC name convention: allow custom name, else default
	pvcName := req.Name
	if pvcName == "" {
		pvcName = fmt.Sprintf("pvc-%s", targetNamespace)
	}

	// 2. Prepare Labels
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

	// 4. Parse Capacity from Size field (already formatted as "XXGi" by handler)
	storageQty, err := resource.ParseQuantity(req.Size)
	if err != nil {
		return nil, fmt.Errorf("invalid capacity: %v", err)
	}

	// 5. Prepare PVC Labels (Critical for Filtering)
	pvcLabels := map[string]string{
		"app.kubernetes.io/name":       "filebrowser-storage",
		"app.kubernetes.io/managed-by": "nthu-cscc",
		"storage-type":                 "project",
		"project-id":                   fmt.Sprintf("%d", req.ProjectID),
		"project-name":                 req.ProjectName,
	}

	// Config
	scName := config.DefaultStorageClassName
	accessMode := corev1.ReadWriteOnce

	// 6. Create PVC Object
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: targetNamespace,
			Labels:    pvcLabels,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{accessMode},

			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: storageQty,
				},
			},

			StorageClassName: &scName,
		},
	}

	pvcClaim, err := k8s.Clientset.CoreV1().PersistentVolumeClaims(targetNamespace).Create(context.TODO(), pvc, metav1.CreateOptions{})

	if err != nil {
		return nil, fmt.Errorf("failed to create pvc: %w", err)
	}

	// Deploy NFS Server
	// This pod mounts the pvcName created above.
	if err := utils.CreateNFSDeployment(targetNamespace, pvcName); err != nil {
		return nil, fmt.Errorf("failed to create nfs deployment: %w", err)
	}

	// Expose NFS Service (Gateway)
	// This creates the DNS entry: storage-svc.project-<project name>-<unique id>.svc.cluster.local
	if err := utils.CreateNFSService(targetNamespace); err != nil {
		return nil, fmt.Errorf("failed to create nfs service: %w", err)
	}

	return pvcClaim, nil
}

// delete project storage
func (s *K8sService) DeleteProjectAllPVC(ctx context.Context, projectName string, projectID uint) error {
	ns := utils.GenerateSafeResourceName("project", projectName, projectID)
	_ = utils.DeleteNamespace(ns)
	return nil
}

// ensureNamespaceWithLabels checks if a namespace exists, creates it if not.
// It's a private helper method for the service.
func (s *K8sService) ensureNamespaceWithLabels(ctx context.Context, name string, labels map[string]string) error {
	_, err := k8s.Clientset.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
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
	_, err = k8s.Clientset.CoreV1().Namespaces().Create(ctx, newNs, metav1.CreateOptions{})
	return err
}

// ListAllProjectStorages retrieves all project-related PVCs using LabelSelectors.
func (s *K8sService) ListAllProjectStorages(ctx context.Context) ([]job.VolumeSpec, error) {
	// Without a Kubernetes client we cannot list PVCs; return empty for tests.
	if k8s.Clientset == nil {
		return []job.VolumeSpec{}, nil
	}

	// 1. Define Filter Options.
	// We use server-side filtering to only fetch PVCs relevant to projects.
	listOptions := metav1.ListOptions{
		LabelSelector: "storage-type=project,app.kubernetes.io/managed-by=nthu-cscc",
	}

	// 2. List PVCs from ALL namespaces.
	pvcs, err := k8s.Clientset.CoreV1().PersistentVolumeClaims(metav1.NamespaceAll).List(ctx, listOptions)
	if err != nil {
		return nil, err
	}

	var result []job.VolumeSpec

	// 3. Map Kubernetes resources to DTOs.
	for _, pvc := range pvcs.Items {
		projectID := pvc.Labels["project-id"]
		projectName := pvc.Labels["project-name"]

		// Skip if essential labels are missing (integrity check)
		if projectID == "" {
			continue
		}

		// Convert projectID string to uint
		projectIDUint, err := strconv.ParseUint(projectID, 10, 32)
		if err != nil {
			projectIDUint = 0
		}

		// Get capacity from PVC spec
		capacityStr := pvc.Spec.Resources.Requests[corev1.ResourceStorage]

		// Convert capacity to int
		capacityValue := int(capacityStr.Value() / 1e9) // Convert to GB
		if capacityValue < 0 {
			capacityValue = 0
		}

		// Safe check for AccessModes
		accessMode := ""
		if len(pvc.Spec.AccessModes) > 0 {
			accessMode = string(pvc.Spec.AccessModes[0])
		}

		result = append(result, job.VolumeSpec{
			ID:          uint(projectIDUint),
			ProjectID:   uint(projectIDUint),
			Name:        pvc.Name, // keep legacy json field populated
			PVCName:     pvc.Name,
			ProjectName: projectName,
			Namespace:   pvc.Namespace,
			Capacity:    capacityValue,
			Size:        fmt.Sprintf("%dGi", capacityValue),
			Status:      string(pvc.Status.Phase),
			AccessMode:  accessMode,
			CreatedAt:   pvc.CreationTimestamp.Time,
		})
	}

	return result, nil
}
