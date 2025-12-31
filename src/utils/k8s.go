package utils

import (
	"context"
	"crypto/sha256"
	applyJson "encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/linskybing/platform-go/src/config"
	"github.com/linskybing/platform-go/src/k8sclient"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// FormatProjectNamespace returns the namespace name for a project (shared PV/PVC, no user suffix)
func FormatProjectNamespace(projectID uint) string {
	return fmt.Sprintf("proj-%d", projectID)
}

var ValidateK8sJSON = func(jsonStr string) (*schema.GroupVersionKind, string, error) {
	decoder := json.NewSerializerWithOptions(
		json.DefaultMetaFactory, config.Scheme, config.Scheme,
		json.SerializerOptions{Yaml: false, Pretty: false, Strict: true},
	)
	obj, gvk, err := decoder.Decode([]byte(jsonStr), nil, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to decode input: %w", err)
	}
	if obj == nil {
		return nil, "", fmt.Errorf("decoded object is nil")
	}

	metaObj, ok := obj.(metav1.Object)
	if !ok {
		return nil, "", fmt.Errorf("object does not implement metav1.Object interface")
	}

	return gvk, metaObj.GetName(), nil
}

var CreateByJson = func(jsonStr []byte, ns string) error {
	if k8sclient.Mapper == nil || k8sclient.DynamicClient == nil {
		fmt.Printf("[MOCK] Created resource by JSON in namespace %s\n", ns)
		return nil
	}
	// decode
	var obj unstructured.Unstructured
	if err := applyJson.Unmarshal(jsonStr, &obj.Object); err != nil {
		return err
	}

	gvk := obj.GroupVersionKind()
	mapping, err := k8sclient.Mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return err
	}

	if ns == "" {
		ns = "default"
	}
	resourceClient := k8sclient.DynamicClient.Resource(mapping.Resource).Namespace(ns)
	result, err := resourceClient.Create(context.TODO(), &obj, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	fmt.Printf("Created %s/%s\n", result.GetKind(), result.GetName())
	return nil
}

var DeleteByJson = func(jsonStr []byte, ns string) error {
	if k8sclient.Mapper == nil || k8sclient.DynamicClient == nil {
		fmt.Printf("[MOCK] Deleted resource by JSON in namespace %s\n", ns)
		return nil
	}
	// decode
	var obj unstructured.Unstructured
	if err := applyJson.Unmarshal(jsonStr, &obj.Object); err != nil {
		return err
	}

	gvk := obj.GroupVersionKind()
	mapping, err := k8sclient.Mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return err
	}

	if ns == "" {
		ns = "default"
	}
	resourceClient := k8sclient.DynamicClient.Resource(mapping.Resource).Namespace(ns)
	err = resourceClient.Delete(context.TODO(), obj.GetName(), metav1.DeleteOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return nil
}

func UpdateByJson(jsonStr []byte, ns string) error {
	if k8sclient.Mapper == nil || k8sclient.DynamicClient == nil {
		fmt.Printf("[MOCK] Updated resource by JSON in namespace %s\n", ns)
		return nil
	}
	// decode
	var obj unstructured.Unstructured
	if err := applyJson.Unmarshal(jsonStr, &obj.Object); err != nil {
		return err
	}

	gvk := obj.GroupVersionKind()
	mapping, err := k8sclient.Mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return err
	}

	if ns == "" {
		ns = "default"
	}
	resourceClient := k8sclient.DynamicClient.Resource(mapping.Resource).Namespace(ns)
	result, err := resourceClient.Update(context.TODO(), &obj, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	fmt.Printf("Updated %s/%s\n", result.GetKind(), result.GetName())
	return nil
}

var CreateNamespace = func(name string) error {
	if k8sclient.Clientset == nil {
		fmt.Printf("[MOCK] create Namespace: %s successfully\n", name)
		return nil
	}
	_, err := k8sclient.Clientset.CoreV1().Namespaces().Get(context.TODO(), name, metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("namespace %s already exist \n", name)
	}

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	_, err = k8sclient.Clientset.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})

	if err != nil {
		return fmt.Errorf("failed create namespace: %v", err)
	}

	fmt.Printf("create Namespace: %s successfully\n", name)
	return nil
}

var DeleteNamespace = func(name string) error {
	if k8sclient.Clientset == nil {
		fmt.Printf("[MOCK] Deleted namespace: %s\n", name)
		return nil
	}
	err := k8sclient.Clientset.CoreV1().Namespaces().Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete namespace %s: %w", name, err)
	}

	fmt.Printf("Deleted namespace: %s\n", name)
	return nil
}

var CheckNamespaceExists = func(name string) (bool, error) {
	if k8sclient.Clientset == nil {
		return false, nil // Mock
	}
	_, err := k8sclient.Clientset.CoreV1().Namespaces().Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func parseResourceQuantity(size string) (resource.Quantity, error) {
	q, err := resource.ParseQuantity(size)
	if err != nil {
		return resource.Quantity{}, fmt.Errorf("invalid PVC size format: %w", err)
	}
	return q, nil
}

func ExpandPVC(ns, pvcName, newSize string) error {
	if k8sclient.Clientset == nil {
		fmt.Printf("[MOCK] PVC %s in namespace %s expanded to %s\n", pvcName, ns, newSize)
		return nil
	}
	if ns == "" {
		ns = "default"
	}

	client := k8sclient.Clientset.CoreV1().PersistentVolumeClaims(ns)

	pvc, err := client.Get(context.TODO(), pvcName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get PVC: %w", err)
	}

	pvc.Spec.Resources.Requests[corev1.ResourceStorage] = resource.MustParse(newSize)

	_, err = client.Update(context.TODO(), pvc, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to expand PVC: %w", err)
	}

	fmt.Printf("PVC %s in namespace %s expanded to %s\n", pvcName, ns, newSize)
	return nil
}

var CreatePVC = func(ns string, name string, storageClassName string, size string) error {
	if k8sclient.Clientset == nil {
		fmt.Printf("[MOCK] PVC %s created in namespace %s\n", name, ns)
		return nil
	}
	if ns == "" {
		ns = "default"
	}

	quntity, err := parseResourceQuantity(size)
	if err != nil {
		return err
	}
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteMany,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: quntity,
				},
			},
			StorageClassName: &storageClassName,
		},
	}

	client := k8sclient.Clientset.CoreV1().PersistentVolumeClaims(ns)

	_, err = client.Get(context.TODO(), name, metav1.GetOptions{})
	if err == nil {
		fmt.Printf("PVC %s already exists in namespace %s\n", name, ns)
		return nil
	}

	result, err := client.Create(context.TODO(), pvc, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create PVC: %w", err)
	}

	fmt.Printf("PVC %s created in namespace %s\n", result.Name, ns)
	return nil
}

func DeletePVC(ns string, pvcName string) error {
	if k8sclient.Clientset == nil {
		fmt.Printf("[MOCK] PVC %s deleted from namespace %s\n", pvcName, ns)
		return nil
	}
	if ns == "" {
		ns = "default"
	}

	client := k8sclient.Clientset.CoreV1().PersistentVolumeClaims(ns)

	policy := metav1.DeletePropagationForeground
	err := client.Delete(context.TODO(), pvcName, metav1.DeleteOptions{
		PropagationPolicy: &policy,
	})
	if err != nil {
		return fmt.Errorf("failed to delete PVC: %w", err)
	}

	fmt.Printf("PVC %s deleted from namespace %s\n", pvcName, ns)
	return nil
}

func GetPVC(ns string, pvcName string) (*corev1.PersistentVolumeClaim, error) {
	if k8sclient.Clientset == nil {
		fmt.Printf("[MOCK] Get PVC %s in namespace %s\n", pvcName, ns)
		return &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{Name: pvcName, Namespace: ns},
			Status:     corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound},
		}, nil
	}
	if ns == "" {
		ns = "default"
	}
	client := k8sclient.Clientset.CoreV1().PersistentVolumeClaims(ns)
	pvc, err := client.Get(context.TODO(), pvcName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get PVC: %w", err)
	}
	fmt.Printf("PVC %s in namespace %s: %s, size: %v\n",
		pvc.Name, ns, pvc.Status.Phase, pvc.Spec.Resources.Requests[corev1.ResourceStorage])

	return pvc, nil
}

func ListPVCs(ns string) ([]corev1.PersistentVolumeClaim, error) {
	if k8sclient.Clientset == nil {
		fmt.Printf("[MOCK] List PVCs in namespace %s\n", ns)
		return []corev1.PersistentVolumeClaim{}, nil
	}
	if ns == "" {
		ns = "default"
	}
	client := k8sclient.Clientset.CoreV1().PersistentVolumeClaims(ns)
	pvcList, err := client.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list PVCs: %w", err)
	}

	for _, pvc := range pvcList.Items {
		q := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
		fmt.Println(q.String())
	}

	return pvcList.Items, nil
}

var CreatePV = func(name string, storageClassName string, size string, path string) error {
	if k8sclient.Clientset == nil {
		fmt.Printf("[MOCK] PV %s created with path %s\n", name, path)
		return nil
	}

	quantity, err := parseResourceQuantity(size)
	if err != nil {
		return err
	}

	pv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: corev1.PersistentVolumeSpec{
			Capacity: corev1.ResourceList{
				corev1.ResourceStorage: quantity,
			},
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteMany,
			},
			StorageClassName: storageClassName,
		},
	}

	// Check if using Longhorn (CSI) or HostPath
	if storageClassName == "longhorn" {
		// Use CSI for Longhorn
		// path argument is treated as volumeHandle
		pv.Spec.PersistentVolumeSource = corev1.PersistentVolumeSource{
			CSI: &corev1.CSIPersistentVolumeSource{
				Driver:       "driver.longhorn.io",
				VolumeHandle: path, // Use the path as the volume handle name
				FSType:       "ext4",
			},
		}
	} else {
		// Default to HostPath
		hostPathType := corev1.HostPathDirectoryOrCreate
		pv.Spec.PersistentVolumeSource = corev1.PersistentVolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: path,
				Type: &hostPathType,
			},
		}
	}

	_, err = k8sclient.Clientset.CoreV1().PersistentVolumes().Create(context.TODO(), pv, metav1.CreateOptions{})
	if err != nil {
		// If PV already exists, we might want to ignore or check if it matches.
		// For now, just return error if it's not "AlreadyExists"
		if apierrors.IsAlreadyExists(err) {
			fmt.Printf("PV %s already exists\n", name)
			return nil
		}
		return fmt.Errorf("failed to create PV: %w", err)
	}
	fmt.Printf("PV %s created\n", name)
	return nil
}

var CreateBoundPVC = func(ns string, name string, storageClassName string, size string, volumeName string) error {
	if k8sclient.Clientset == nil {
		fmt.Printf("[MOCK] Bound PVC %s created in namespace %s bound to %s\n", name, ns, volumeName)
		return nil
	}
	if ns == "" {
		ns = "default"
	}

	quantity, err := parseResourceQuantity(size)
	if err != nil {
		return err
	}
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteMany,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: quantity,
				},
			},
			StorageClassName: &storageClassName,
			VolumeName:       volumeName,
		},
	}

	client := k8sclient.Clientset.CoreV1().PersistentVolumeClaims(ns)

	_, err = client.Get(context.TODO(), name, metav1.GetOptions{})
	if err == nil {
		fmt.Printf("PVC %s already exists in namespace %s\n", name, ns)
		return nil
	}

	result, err := client.Create(context.TODO(), pvc, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create PVC: %w", err)
	}

	fmt.Printf("PVC %s created in namespace %s bound to %s\n", result.Name, ns, volumeName)
	return nil
}

// CreateHubPVC creates the underlying Longhorn volume for the user's storage hub.
// It enforces ReadWriteOnce (RWO) mode, which is optimal for Longhorn block storage performance.
// This volume will be mounted by the NFS server pod.
var CreateHubPVC = func(ns string, name string, storageClassName string, size string) error {
	if k8sclient.Clientset == nil {
		fmt.Printf("[MOCK] Hub PVC %s created in %s\n", name, ns)
		return nil
	}

	quantity, err := parseResourceQuantity(size)
	if err != nil {
		return err
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: corev1.PersistentVolumeClaimSpec{
			// Use ReadWriteOnce for the backing storage to ensure data consistency and performance
			// when accessed by the single NFS server instance.
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: quantity},
			},
			StorageClassName: &storageClassName,
		},
	}

	_, err = k8sclient.Clientset.CoreV1().PersistentVolumeClaims(ns).Create(context.TODO(), pvc, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create Hub PVC: %w", err)
	}
	fmt.Printf("Hub PVC %s created in namespace %s\n", name, ns)
	return nil
}

// CreateNFSDeployment deploys a lightweight NFS server pod.
// This pod mounts the Longhorn RWO volume and exposes it as an NFS share.
// Privileged mode is required for the NFS server to operate correctly.
var CreateNFSDeployment = func(ns string, pvcName string) error {
	// Mock implementation for testing when K8s client is missing
	if k8sclient.Clientset == nil {
		fmt.Printf("[MOCK] NFS Deployment created in %s mounting %s\n", ns, pvcName)
		return nil
	}

	name := "storage-gateway"
	replicas := int32(1) // Must be 1 since the backing PVC is ReadWriteOnce (RWO)
	privileged := true   // NFS server requires privileged security context to access kernel modules

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,

			// [CRITICAL FIX] Use "Recreate" strategy instead of default "RollingUpdate".
			// Since the PVC is RWO (ReadWriteOnce), the old pod must be fully terminated
			// and release the volume before the new pod can attach it.
			// Without this, the deployment will hang in a deadlock.
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RecreateDeploymentStrategyType,
			},

			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "nfs-gateway"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "nfs-gateway"}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "nfs",
							// [CRITICAL FIX] Replaced "itsthenetwork/nfs-server-alpine".
							// The previous Alpine-based image causes "assertion failed" errors
							// due to compatibility issues with file locking (fcntl) on modern kernels.
							// "erichough/nfs-server" is more stable and widely used.
							Image: "erichough/nfs-server:latest",

							// Configuration for erichough/nfs-server
							Env: []corev1.EnvVar{
								{
									Name:  "NFS_EXPORT_0",
									Value: "/exports *(rw,fsid=0,async,no_subtree_check,no_auth_nlm,insecure,no_root_squash)",
								},
							},

							Ports: []corev1.ContainerPort{
								{Name: "nfs", ContainerPort: 2049, Protocol: corev1.ProtocolTCP},
								{Name: "mountd", ContainerPort: 20048, Protocol: corev1.ProtocolTCP},
								{Name: "rpcbind", ContainerPort: 111, Protocol: corev1.ProtocolTCP},
								// [FIX] Add UDP protocol for rpcbind.
								// Some NFS clients prioritize UDP for discovery; missing this can cause mount delays.
								{Name: "rpcbind-udp", ContainerPort: 111, Protocol: corev1.ProtocolUDP},
							},
							SecurityContext: &corev1.SecurityContext{
								Privileged: &privileged, // Required for NFS kernel capabilities
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "hub-storage", MountPath: "/exports"},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "hub-storage",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: pvcName,
								},
							},
						},
					},
				},
			},
		},
	}

	_, err := k8sclient.Clientset.AppsV1().Deployments(ns).Create(context.TODO(), deploy, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create NFS deployment: %w", err)
	}
	fmt.Printf("NFS Deployment created in %s\n", ns)
	return nil
}

// CreateNFSService creates a ClusterIP service to expose the NFS server.
// This service acts as the stable endpoint (Gateway) for project namespaces to mount storage.
var CreateNFSService = func(ns string) error {
	if k8sclient.Clientset == nil {
		fmt.Printf("[MOCK] NFS Service created in %s\n", ns)
		return nil
	}

	name := "storage-svc"
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": "nfs-gateway"},
			Ports: []corev1.ServicePort{
				{Name: "nfs", Port: 2049, TargetPort: intstr.FromInt(2049)},
				{Name: "mountd", Port: 20048, TargetPort: intstr.FromInt(20048)},
				{Name: "rpcbind", Port: 111, TargetPort: intstr.FromInt(111)},
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}

	_, err := k8sclient.Clientset.CoreV1().Services(ns).Create(context.TODO(), svc, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create NFS service: %w", err)
	}
	fmt.Printf("NFS Service created in %s\n", ns)
	return nil
}

// CreateNFSPV creates a PersistentVolume backed by an NFS server.
// This is used by "Spoke" projects to connect to the User's "Hub" storage.
// The server address should be the DNS name of the user's storage service
// (e.g., storage-svc.user-sky-storage.svc.cluster.local).
var CreateNFSPV = func(name string, server string, path string, size string) error {
	if k8sclient.Clientset == nil {
		fmt.Printf("[MOCK] NFS PV %s created pointing to %s:%s\n", name, server, path)
		return nil
	}

	quantity, err := parseResourceQuantity(size)
	if err != nil {
		return err
	}

	pv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: name, // Must be unique cluster-wide
		},
		Spec: corev1.PersistentVolumeSpec{
			Capacity: corev1.ResourceList{
				corev1.ResourceStorage: quantity,
			},
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteMany, // NFS supports RWX
			},
			// Use NFS source instead of HostPath or CSI
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				NFS: &corev1.NFSVolumeSource{
					Server: server,
					Path:   path,
				},
			},
			// Empty StorageClass prevents dynamic provisioners from interfering
			StorageClassName: "",
		},
	}

	_, err = k8sclient.Clientset.CoreV1().PersistentVolumes().Create(context.TODO(), pv, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create NFS PV: %w", err)
	}
	fmt.Printf("NFS PV %s created\n", name)
	return nil
}

var FormatNamespaceName = func(projectID uint, userName string) string {
	return fmt.Sprintf("proj-%d-%s", projectID, userName)
}

// GenerateSafeResourceName generates a unique and K8s-compliant resource name.
// Format: prefix-{sanitized_name}-{short_hash}
// Constraint: Kubernetes names must be max 63 characters, lowercase, alphanumeric, or hyphen.
func GenerateSafeResourceName(prefix string, name string, id uint) string {
	// 1. Sanitize Name: Keep only lowercase alphanumeric characters and hyphens.
	// Replace invalid characters with a hyphen.
	reg := regexp.MustCompile("[^a-z0-9]+")
	safeName := reg.ReplaceAllString(strings.ToLower(name), "-")
	safeName = strings.Trim(safeName, "-") // Remove leading/trailing hyphens

	// 2. Generate Short Hash from ID to ensure uniqueness.
	// Using the ID as a seed ensures that the same Project ID always generates the same namespace name.
	hashInput := fmt.Sprintf("project-%d", id)
	hash := sha256.Sum256([]byte(hashInput))
	shortHash := fmt.Sprintf("%x", hash)[:6] // Take the first 6 characters of the hash

	// 3. Construct the final name.
	// Format: prefix-name-hash
	// We need to ensure the total length does not exceed 63 characters.
	baseName := fmt.Sprintf("%s-%s", prefix, safeName)
	suffix := fmt.Sprintf("-%s", shortHash)

	// Calculate max allowed length for the base name to accommodate the suffix.
	maxLength := 63 - len(suffix)
	if len(baseName) > maxLength {
		baseName = baseName[:maxLength]
		// Ensure we don't end with a hyphen after truncation
		baseName = strings.TrimRight(baseName, "-")
	}

	return baseName + suffix
}

func ToSafeK8sName(rawName string) string {
	safeName := strings.ToLower(rawName)

	reg := regexp.MustCompile(`[^a-z0-9]+`)
	safeName = reg.ReplaceAllString(safeName, "-")

	safeName = strings.Trim(safeName, "-")

	multiHyphenReg := regexp.MustCompile(`-+`)
	safeName = multiHyphenReg.ReplaceAllString(safeName, "-")

	if len(safeName) > 63 {
		safeName = safeName[:63]
		safeName = strings.TrimRight(safeName, "-")
	}

	if safeName == "" {
		safeName = "unnamed"
	}

	return safeName
}
