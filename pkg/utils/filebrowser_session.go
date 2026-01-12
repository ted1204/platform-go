package utils

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/linskybing/platform-go/pkg/k8s" // 假設這是你的 k8s client wrapper
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
)

// Helper function to generate resource names based on PVC name
func getFileBrowserNames(pvcName string) (podName, svcName string) {
	// e.g. fb-pod-mydata, fb-svc-mydata
	return fmt.Sprintf("fb-pod-%s", pvcName), fmt.Sprintf("fb-svc-%s", pvcName)
}

// CreateFileBrowserPod creates a temporary Pod for file browsing
func CreateFileBrowserPod(ctx context.Context, ns string, pvcName string) (*corev1.Pod, error) {
	if k8s.Clientset == nil {
		fmt.Printf("[MOCK] Created FB Pod for %s in %s\n", pvcName, ns)
		return nil, nil
	}

	podName, _ := getFileBrowserNames(pvcName)

	// Define the Pod
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: ns,
			Labels:    map[string]string{"app": podName}, // Unique label for Service selector
		},
		Spec: corev1.PodSpec{
			// Set pod-level security context for consistent file permissions
			SecurityContext: &corev1.PodSecurityContext{
				RunAsUser:  int64Ptr(0), // Non-root user
				RunAsGroup: int64Ptr(0), // Non-root group
				FSGroup:    int64Ptr(0), // All files created will belong to this group
			},
			Containers: []corev1.Container{
				{
					Name:  "filebrowser",
					Image: "filebrowser/filebrowser:v2",
					Args:  []string{"--noauth", "--root=/srv", "--address=0.0.0.0"},
					Ports: []corev1.ContainerPort{{ContainerPort: 80}},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "data",
							MountPath: "/srv",
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "data",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvcName,
						},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyNever, // Ephemeral pod, no restart
		},
	}

	createdPod, err := k8s.Clientset.CoreV1().Pods(ns).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			// If already exists, return current pod (or handle as success)
			return k8s.Clientset.CoreV1().Pods(ns).Get(ctx, podName, metav1.GetOptions{})
		}
		return nil, fmt.Errorf("failed to create FB pod: %w", err)
	}

	return createdPod, nil
}

// CreateFileBrowserService creates a NodePort service and returns the assigned port
func CreateFileBrowserService(ctx context.Context, ns string, pvcName string) (string, error) {
	if k8s.Clientset == nil {
		return "30000", nil // Mock port
	}

	podName, svcName := getFileBrowserNames(pvcName)

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svcName,
			Namespace: ns,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": podName}, // Match the Pod created above
			Ports: []corev1.ServicePort{
				{
					Port:       80,
					TargetPort: intstr.FromInt(80),
					// NodePort will be auto-assigned by K8s if not specified
				},
			},
			Type: corev1.ServiceTypeNodePort,
		},
	}

	createdSvc, err := k8s.Clientset.CoreV1().Services(ns).Create(ctx, svc, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			// If exists, fetch it to get the existing port
			existingSvc, getErr := k8s.Clientset.CoreV1().Services(ns).Get(ctx, svcName, metav1.GetOptions{})
			if getErr != nil {
				return "", getErr
			}
			createdSvc = existingSvc
		} else {
			return "", fmt.Errorf("failed to create FB service: %w", err)
		}
	}

	// Wait for NodePort allocation (usually instant, but good to be safe)
	var nodePort int32
	err = wait.PollUntilContextTimeout(ctx, 100*time.Millisecond, 5*time.Second, true, func(ctx context.Context) (bool, error) {
		if len(createdSvc.Spec.Ports) > 0 && createdSvc.Spec.Ports[0].NodePort > 0 {
			nodePort = createdSvc.Spec.Ports[0].NodePort
			return true, nil
		}
		// Refresh svc just in case
		createdSvc, _ = k8s.Clientset.CoreV1().Services(ns).Get(ctx, svcName, metav1.GetOptions{})
		return false, nil
	})

	if err != nil {
		return "", fmt.Errorf("timeout waiting for NodePort assignment")
	}

	return fmt.Sprintf("%d", nodePort), nil
}

// DeleteFileBrowserResources cleans up both Pod and Service
func DeleteFileBrowserResources(ctx context.Context, ns string, pvcName string) error {
	if k8s.Clientset == nil {
		fmt.Printf("[MOCK] Cleaning up FB resources for %s\n", pvcName)
		return nil
	}

	podName, svcName := getFileBrowserNames(pvcName)
	gracePeriod := int64(0) // Force delete for faster cleanup

	// 1. Delete Service
	errSvc := k8s.Clientset.CoreV1().Services(ns).Delete(ctx, svcName, metav1.DeleteOptions{})
	if errSvc != nil && !apierrors.IsNotFound(errSvc) {
		return fmt.Errorf("failed to delete service: %w", errSvc)
	}

	// 2. Delete Pod
	errPod := k8s.Clientset.CoreV1().Pods(ns).Delete(ctx, podName, metav1.DeleteOptions{
		GracePeriodSeconds: &gracePeriod,
	})
	if errPod != nil && !apierrors.IsNotFound(errPod) {
		return fmt.Errorf("failed to delete pod: %w", errPod)
	}

	fmt.Printf("Stopped FileBrowser for %s/%s\n", ns, pvcName)
	return nil
}

func int64Ptr(i int64) *int64 { return &i }

func StartUserHubBrowser(ctx context.Context, username string) (string, error) {
	if k8s.Clientset == nil {
		return "30000", nil
	}

	ns := fmt.Sprintf("user-%s-storage", username)
	pvcName := fmt.Sprintf("user-%s-disk", username)
	appName := fmt.Sprintf("fb-hub-%s", username)
	svcName := fmt.Sprintf("fb-hub-svc-%s", username)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      appName,
			Namespace: ns,
			Labels:    map[string]string{"app": appName},
		},
		Spec: corev1.PodSpec{
			SecurityContext: &corev1.PodSecurityContext{
				RunAsUser:  int64Ptr(0),
				RunAsGroup: int64Ptr(0),
				FSGroup:    int64Ptr(0),
			},
			Containers: []corev1.Container{
				{
					Name:  "filebrowser",
					Image: "filebrowser/filebrowser:v2",
					Args:  []string{"--noauth", "--root=/srv", "--address=0.0.0.0", "--baseurl=/k8s/users/proxy"},
					Ports: []corev1.ContainerPort{{ContainerPort: 80}},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "user-data",
							MountPath: "/srv",
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "user-data",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvcName,
						},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyOnFailure,
		},
	}

	_, err := k8s.Clientset.CoreV1().Pods(ns).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return "", fmt.Errorf("failed to create Hub FB pod: %w", err)
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svcName,
			Namespace: ns,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": appName},
			Ports: []corev1.ServicePort{
				{
					Port:       80,
					TargetPort: intstr.FromInt(80),
				},
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}

	_, err = k8s.Clientset.CoreV1().Services(ns).Create(ctx, svc, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return "", fmt.Errorf("failed to create Hub FB service: %w", err)
	}

	return "80", nil
}

func StopUserHubBrowser(ctx context.Context, username string) error {
	if k8s.Clientset == nil {
		fmt.Printf("[MOCK] Stopped Hub Browser for %s\n", username)
		return nil
	}

	ns := fmt.Sprintf("user-%s-storage", username)
	appName := fmt.Sprintf("fb-hub-%s", username)
	svcName := fmt.Sprintf("fb-hub-svc-%s", username)

	gracePeriod := int64(0)

	err := k8s.Clientset.CoreV1().Services(ns).Delete(ctx, svcName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete hub service: %w", err)
	}

	err = k8s.Clientset.CoreV1().Pods(ns).Delete(ctx, appName, metav1.DeleteOptions{
		GracePeriodSeconds: &gracePeriod,
	})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete hub pod: %w", err)
	}

	log.Printf("[StorageHub] Browser stopped and disk unmounted for user: %s", username)
	return nil
}
