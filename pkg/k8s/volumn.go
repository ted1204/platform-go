package k8s

import (
	"context"
	"fmt"

	"github.com/linskybing/platform-go/internal/config"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func parseResourceQuantity(size string) (resource.Quantity, error) {
	q, err := resource.ParseQuantity(size)
	if err != nil {
		return resource.Quantity{}, fmt.Errorf("invalid PVC size format: %w", err)
	}
	return q, nil
}

func ExpandPVC(ns, pvcName, newSize string) error {
	if Clientset == nil {
		fmt.Printf("[MOCK] PVC %s in namespace %s expanded to %s\n", pvcName, ns, newSize)
		return nil
	}
	if ns == "" {
		ns = "default"
	}

	client := Clientset.CoreV1().PersistentVolumeClaims(ns)

	pvc, err := client.Get(context.TODO(), pvcName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get PVC: %w", err)
	}

	newQuantity, err := resource.ParseQuantity(newSize)
	if err != nil {
		return fmt.Errorf("invalid size format: %w", err)
	}

	currentSize := pvc.Spec.Resources.Requests[corev1.ResourceStorage]

	// Check for shrinking
	if newQuantity.Cmp(currentSize) < 0 {
		return fmt.Errorf("cannot shrink PVC: current size %s, requested %s", currentSize.String(), newSize)
	}

	// Update the request
	pvc.Spec.Resources.Requests[corev1.ResourceStorage] = newQuantity

	_, err = client.Update(context.TODO(), pvc, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to expand PVC: %w", err)
	}

	fmt.Printf("PVC %s in namespace %s expanded to %s\n", pvcName, ns, newSize)
	return nil
}

func CreateHubPVC(ns string, name string, storageClassName string, size string) error {
	if Clientset == nil {
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
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: quantity},
			},
			StorageClassName: &storageClassName,
		},
	}

	_, err = Clientset.CoreV1().PersistentVolumeClaims(ns).Create(context.TODO(), pvc, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create Hub PVC: %w", err)
	}
	fmt.Printf("Hub PVC %s created in namespace %s\n", name, ns)
	return nil
}

// CreateStorageHub creates a lightweight Alpine pod to mount a PVC.
// This allows admins or systems to write/debug data in the Longhorn volume via "kubectl cp" or "exec".
func CreateStorageHub(ns string, pvcName string) error {

	hubName := fmt.Sprintf("storage-hub-%s", pvcName)
	replicas := int32(1)

	privileged := false

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hubName,
			Namespace: ns,
			Labels:    map[string]string{"app": "storage-hub", "pvc": pvcName},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Strategy: appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType},
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "storage-hub", "pvc": pvcName}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "storage-hub", "pvc": pvcName}},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: new(int64),
					Containers: []corev1.Container{
						{
							Name:    "hub-client",
							Image:   "alpine:latest",
							Command: []string{"/bin/sh", "-c", "echo 'Storage Hub Running...'; sleep infinity"},

							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "longhorn-vol",
									MountPath: "/data",
								},
							},
							SecurityContext: &corev1.SecurityContext{
								Privileged: &privileged,
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "longhorn-vol",
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

	_, err := Clientset.AppsV1().Deployments(ns).Create(context.TODO(), deploy, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			fmt.Printf("Storage Hub %s already exists in %s.\n", hubName, ns)
			return nil
		}
		return fmt.Errorf("failed to create Storage Hub: %w", err)
	}

	fmt.Printf("Storage Hub created: %s (ns: %s). Mount path: /data\n", hubName, ns)
	return nil
}

func MountExistingVolumeToProject(sourceNs, sourcePvcName, targetNs, targetPvcName string) error {
	ctx := context.TODO()

	_, err := Clientset.CoreV1().PersistentVolumeClaims(targetNs).Get(ctx, targetPvcName, metav1.GetOptions{})
	if err == nil {
		// Already exists, assume it's correct and return.
		// logic: If you want to enforce updates, you'd need to delete and recreate here.
		return nil
	} else if !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to check existing target pvc: %w", err)
	}

	sourcePvc, err := Clientset.CoreV1().PersistentVolumeClaims(sourceNs).Get(ctx, sourcePvcName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to find source pvc %s/%s: %w", sourceNs, sourcePvcName, err)
	}

	pvName := sourcePvc.Spec.VolumeName
	if pvName == "" {
		return fmt.Errorf("source pvc %s/%s is not bound to any PV yet", sourceNs, sourcePvcName)
	}

	sourcePV, err := Clientset.CoreV1().PersistentVolumes().Get(ctx, pvName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get source pv %s: %w", pvName, err)
	}

	if sourcePV.Spec.CSI == nil || sourcePV.Spec.CSI.Driver != "driver.longhorn.io" {
		return fmt.Errorf("source pv %s is not a Longhorn volume (driver mismatch)", pvName)
	}

	volumeHandle := sourcePV.Spec.CSI.VolumeHandle
	storageSize := sourcePV.Spec.Capacity[corev1.ResourceStorage]

	volumeMode := corev1.PersistentVolumeFilesystem
	if sourcePV.Spec.VolumeMode != nil {
		volumeMode = *sourcePV.Spec.VolumeMode
	}

	newPvName := fmt.Sprintf("share-%s-%s", targetNs, targetPvcName)

	newPV := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: newPvName,
			Labels: map[string]string{
				"created-by": "k8s-platform-share",
				"target-ns":  targetNs,
				"source-vol": volumeHandle,
			},
		},
		Spec: corev1.PersistentVolumeSpec{
			Capacity: corev1.ResourceList{
				corev1.ResourceStorage: storageSize,
			},
			VolumeMode:  &volumeMode,
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
			// [CRITICAL] Retain policy ensures that when we delete the Project's PVC,
			// the actual data (Longhorn Volume) is NOT deleted.
			PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimRetain,

			StorageClassName: "longhorn",
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{
					Driver:       "driver.longhorn.io",
					VolumeHandle: volumeHandle,
					FSType:       "ext4",
				},
			},
		},
	}

	_, err = Clientset.CoreV1().PersistentVolumes().Create(ctx, newPV, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create static share PV: %w", err)
	}
	targetPvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      targetPvcName,
			Namespace: targetNs,
			Labels: map[string]string{
				"created-by": "k8s-platform-share",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: storageSize,
				},
			},
			StorageClassName: sourcePvc.Spec.StorageClassName,
			VolumeName:       newPvName,
		},
	}

	_, err = Clientset.CoreV1().PersistentVolumeClaims(targetNs).Create(ctx, targetPvc, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create target pvc: %w", err)
	}

	return nil
}

// DeleteProjectStorageCompletely handles the cleanup for a project.
// It iterates over ALL PVCs in the project namespace to ensure all shared pointers are removed.
func DeleteProjectStorageCompletely(ctx context.Context, projectName string, projectID uint) error {
	nsName := GenerateSafeResourceName("project", projectName, projectID)

	fmt.Printf("[Cleanup] Starting cleanup for project: %s (ns: %s)\n", projectName, nsName)

	// 1. List ALL PVCs in the project namespace
	// We don't guess names like "project-disk", we find whatever exists.
	pvcs, err := Clientset.CoreV1().PersistentVolumeClaims(nsName).List(ctx, metav1.ListOptions{})
	if err != nil {
		// If namespace doesn't exist, we are done.
		return DeleteNamespace(nsName)
	}

	// 2. Iterate and Clean each PVC tree
	for _, pvc := range pvcs.Items {
		fmt.Printf("[Cleanup] Processing project PVC: %s\n", pvc.Name)

		// Call the generic helper for each PVC found
		if err := cleanUpSinglePVCTree(ctx, nsName, pvc.Name); err != nil {
			// Log error but continue to try cleaning other PVCs and the Namespace
			fmt.Printf("[Error] Failed to clean PVC %s: %v\n", pvc.Name, err)
		}
	}

	// 3. Delete the Namespace
	// Now that all "Pointer PVs" pointing to this project's data are severed,
	// we can safely delete the namespace to remove the data physically.
	if err := DeleteNamespace(nsName); err != nil {
		return fmt.Errorf("failed to delete project namespace: %w", err)
	}

	fmt.Printf("[Cleanup] Successfully deleted project resources: %s\n", projectName)
	return nil
}

// DeleteUserStorageCompletely handles the cleanup for a user.
func DeleteUserStorageCompletely(ctx context.Context, username string) error {
	safeUser := ToSafeK8sName(username)
	nsName := fmt.Sprintf(config.UserStorageNs, safeUser)
	pvcName := fmt.Sprintf(config.UserStoragePVC, safeUser)

	fmt.Printf("[Cleanup] Starting cleanup for user: %s\n", username)

	if err := cleanUpSinglePVCTree(ctx, nsName, pvcName); err != nil {
		return fmt.Errorf("failed to clean up user pvc: %w", err)
	}

	if err := DeleteNamespace(nsName); err != nil {
		return fmt.Errorf("failed to delete user namespace: %w", err)
	}

	return nil
}

// cleanUpSinglePVCTree performs the "Reverse Cleanup" for a specific PVC.
// 1. Finds the underlying Longhorn Volume Handle.
// 2. Finds and deletes all "Pointer PVs" (Shadow PVs) referencing this handle.
// 3. Deletes the Source PVC itself.
func cleanUpSinglePVCTree(ctx context.Context, ns string, pvcName string) error {
	// 1. Get Source PVC
	pvc, err := Clientset.CoreV1().PersistentVolumeClaims(ns).Get(ctx, pvcName, metav1.GetOptions{})
	if err != nil {
		// If PVC is gone, we assume this tree is already clean.
		return nil
	}

	pvName := pvc.Spec.VolumeName
	if pvName == "" {
		// PVC exists but unbound. Just delete the PVC.
		return Clientset.CoreV1().PersistentVolumeClaims(ns).Delete(ctx, pvcName, metav1.DeleteOptions{})
	}

	// 2. Get Source PV to find the Handle
	sourcePV, err := Clientset.CoreV1().PersistentVolumes().Get(ctx, pvName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get source pv %s: %w", pvName, err)
	}

	// Check if it's a Longhorn volume
	if sourcePV.Spec.CSI == nil || sourcePV.Spec.CSI.Driver != "driver.longhorn.io" {
		// Not a Longhorn volume (or mock), just delete the PVC
		return Clientset.CoreV1().PersistentVolumeClaims(ns).Delete(ctx, pvcName, metav1.DeleteOptions{})
	}

	volumeHandle := sourcePV.Spec.CSI.VolumeHandle

	// 3. Find all Pointer PVs referencing this Handle
	// LabelSelector must match what we set in MountExistingVolumeToProject
	labelSelector := fmt.Sprintf("source-vol=%s,created-by=k8s-platform-share", volumeHandle)

	pointerPVs, err := Clientset.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return fmt.Errorf("failed to list pointer PVs for %s: %w", pvcName, err)
	}

	// 4. Delete Pointer PVs (The Clones)
	for _, pv := range pointerPVs.Items {
		err := Clientset.CoreV1().PersistentVolumes().Delete(ctx, pv.Name, metav1.DeleteOptions{})
		if err != nil {
			fmt.Printf("[Warning] Failed to delete pointer PV %s: %v\n", pv.Name, err)
		} else {
			fmt.Printf("[Cleanup] Deleted pointer PV: %s (was linked to %s)\n", pv.Name, pvcName)
		}
	}

	// 5. Delete the Source PVC (The Root)
	// We delete PVC specifically here, not the Namespace yet.
	if err := Clientset.CoreV1().PersistentVolumeClaims(ns).Delete(ctx, pvcName, metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("failed to delete source pvc %s: %w", pvcName, err)
	}

	return nil
}
