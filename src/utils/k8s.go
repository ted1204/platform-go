package utils

import (
	"context"
	applyJson "encoding/json"
	"fmt"

	"github.com/linskybing/platform-go/src/config"
	"github.com/linskybing/platform-go/src/k8sclient"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
)

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

var FormatNamespaceName = func(projectID uint, userName string) string {
	return fmt.Sprintf("proj-%d-%s", projectID, userName)
}
