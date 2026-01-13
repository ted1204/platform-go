package k8s

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateNamespace(name string) error {
	if Clientset == nil {
		fmt.Printf("[MOCK] create Namespace: %s successfully\n", name)
		return nil
	}
	_, err := Clientset.CoreV1().Namespaces().Get(context.TODO(), name, metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("namespace %s already exists", name)
	}

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	_, err = Clientset.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})

	if err != nil {
		return fmt.Errorf("failed create namespace: %v", err)
	}

	fmt.Printf("create Namespace: %s successfully\n", name)
	return nil
}

func DeleteNamespace(name string) error {
	if Clientset == nil {
		fmt.Printf("[MOCK] Deleted namespace: %s\n", name)
		return nil
	}
	err := Clientset.CoreV1().Namespaces().Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete namespace %s: %w", name, err)
	}

	fmt.Printf("Deleted namespace: %s\n", name)
	return nil
}

func EnsureNamespaceExists(nsName string) error {

	if Clientset == nil {
		// In unit tests or environments without a k8s client, behave as a no-op
		// and assume the namespace exists / can be created.
		fmt.Printf("[MOCK] ensure namespace exists: %s\n", nsName)
		return nil
	}

	_, err := Clientset.CoreV1().Namespaces().Get(context.TODO(), nsName, metav1.GetOptions{})
	if err == nil {
		return nil
	}

	if !apierrors.IsNotFound(err) {
		return err
	}

	newNs := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: nsName,
			Labels: map[string]string{
				"managed-by": "gpu-platform",
				"created-at": time.Now().Format("20060102-150405"),
			},
		},
	}

	_, err = Clientset.CoreV1().Namespaces().Create(context.TODO(), newNs, metav1.CreateOptions{})
	return err
}

func CheckNamespaceExists(name string) (bool, error) {
	if Clientset == nil {
		return false, nil // Mock
	}
	_, err := Clientset.CoreV1().Namespaces().Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func FormatNamespaceName(projectID uint, userName string) string {
	return fmt.Sprintf("proj-%d-%s", projectID, userName)
}
