//go:build integration
// +build integration

package integration

import (
	"context"
	"fmt"
	"time"

	"github.com/linskybing/platform-go/pkg/k8s"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// K8sValidator provides K8s resource validation utilities
type K8sValidator struct{}

// NewK8sValidator creates a new K8s validator
func NewK8sValidator() *K8sValidator {
	return &K8sValidator{}
}

// NamespaceExists checks if a namespace exists
func (v *K8sValidator) NamespaceExists(namespace string) (bool, error) {
	if k8s.Clientset == nil {
		return false, fmt.Errorf("k8s client not initialized")
	}
	_, err := k8s.Clientset.CoreV1().Namespaces().Get(
		context.Background(),
		namespace,
		metav1.GetOptions{},
	)
	if err != nil {
		return false, nil
	}
	return true, nil
}

// PVCExists checks if a PVC exists in a namespace
func (v *K8sValidator) PVCExists(namespace, name string) (bool, error) {
	_, err := k8s.Clientset.CoreV1().PersistentVolumeClaims(namespace).Get(
		context.Background(),
		name,
		metav1.GetOptions{},
	)
	if err != nil {
		return false, nil
	}
	return true, nil
}

// GetPVC gets a PVC
func (v *K8sValidator) GetPVC(namespace, name string) (*corev1.PersistentVolumeClaim, error) {
	return k8s.Clientset.CoreV1().PersistentVolumeClaims(namespace).Get(
		context.Background(),
		name,
		metav1.GetOptions{},
	)
}

// PodExists checks if a pod exists in a namespace
func (v *K8sValidator) PodExists(namespace, name string) (bool, error) {
	_, err := k8s.Clientset.CoreV1().Pods(namespace).Get(
		context.Background(),
		name,
		metav1.GetOptions{},
	)
	if err != nil {
		return false, nil
	}
	return true, nil
}

// GetPod gets a pod
func (v *K8sValidator) GetPod(namespace, name string) (*corev1.Pod, error) {
	return k8s.Clientset.CoreV1().Pods(namespace).Get(
		context.Background(),
		name,
		metav1.GetOptions{},
	)
}

// WaitForPodRunning waits for a pod to be in Running state
func (v *K8sValidator) WaitForPodRunning(namespace, name string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for pod %s/%s to be running", namespace, name)
		case <-ticker.C:
			pod, err := v.GetPod(namespace, name)
			if err != nil {
				continue
			}
			if pod.Status.Phase == corev1.PodRunning {
				return nil
			}
		}
	}
}

// ServiceExists checks if a service exists in a namespace
func (v *K8sValidator) ServiceExists(namespace, name string) (bool, error) {
	_, err := k8s.Clientset.CoreV1().Services(namespace).Get(
		context.Background(),
		name,
		metav1.GetOptions{},
	)
	if err != nil {
		return false, nil
	}
	return true, nil
}

// GetService gets a service
func (v *K8sValidator) GetService(namespace, name string) (*corev1.Service, error) {
	return k8s.Clientset.CoreV1().Services(namespace).Get(
		context.Background(),
		name,
		metav1.GetOptions{},
	)
}

// DeploymentExists checks if a deployment exists in a namespace
func (v *K8sValidator) DeploymentExists(namespace, name string) (bool, error) {
	if k8s.Clientset == nil {
		return false, fmt.Errorf("k8s client not initialized")
	}
	_, err := k8s.Clientset.AppsV1().Deployments(namespace).Get(
		context.Background(),
		name,
		metav1.GetOptions{},
	)
	if err != nil {
		return false, nil
	}
	return true, nil
}

// GetDeployment gets a deployment
func (v *K8sValidator) GetDeployment(namespace, name string) (*appsv1.Deployment, error) {
	return k8s.Clientset.AppsV1().Deployments(namespace).Get(
		context.Background(),
		name,
		metav1.GetOptions{},
	)
}

// WaitForDeploymentReady waits for a deployment to be ready
func (v *K8sValidator) WaitForDeploymentReady(namespace, name string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for deployment %s/%s to be ready", namespace, name)
		case <-ticker.C:
			deployment, err := v.GetDeployment(namespace, name)
			if err != nil {
				continue
			}
			if deployment.Status.ReadyReplicas == *deployment.Spec.Replicas {
				return nil
			}
		}
	}
}

// ListPods lists all pods in a namespace
func (v *K8sValidator) ListPods(namespace string, labelSelector string) (*corev1.PodList, error) {
	return k8s.Clientset.CoreV1().Pods(namespace).List(
		context.Background(),
		metav1.ListOptions{
			LabelSelector: labelSelector,
		},
	)
}

// CreateTestPVC creates a test PVC
func (v *K8sValidator) CreateTestPVC(namespace, name, storageClass, size string) error {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: parseQuantity(size),
				},
			},
		},
	}

	if storageClass != "" {
		pvc.Spec.StorageClassName = &storageClass
	}

	_, err := k8s.Clientset.CoreV1().PersistentVolumeClaims(namespace).Create(
		context.Background(),
		pvc,
		metav1.CreateOptions{},
	)
	return err
}

// DeletePVC deletes a PVC
func (v *K8sValidator) DeletePVC(namespace, name string) error {
	return k8s.Clientset.CoreV1().PersistentVolumeClaims(namespace).Delete(
		context.Background(),
		name,
		metav1.DeleteOptions{},
	)
}

// CreateTestPod creates a test pod
func (v *K8sValidator) CreateTestPod(namespace, name string, labels map[string]string) error {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "test-container",
					Image: "nginx:alpine",
				},
			},
		},
	}

	_, err := k8s.Clientset.CoreV1().Pods(namespace).Create(
		context.Background(),
		pod,
		metav1.CreateOptions{},
	)
	return err
}

// DeletePod deletes a pod
func (v *K8sValidator) DeletePod(namespace, name string) error {
	return k8s.Clientset.CoreV1().Pods(namespace).Delete(
		context.Background(),
		name,
		metav1.DeleteOptions{},
	)
}

// Helper function to parse quantity
func parseQuantity(size string) resource.Quantity {
	qty, _ := resource.ParseQuantity(size)
	return qty
}
