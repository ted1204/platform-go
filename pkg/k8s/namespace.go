package k8s

import (
	"context"
	"fmt"

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
