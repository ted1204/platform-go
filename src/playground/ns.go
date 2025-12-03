package main

import (
	"context"
	"fmt"
	"log"

	"github.com/linskybing/platform-go/src/config"
	"github.com/linskybing/platform-go/src/db"
	"github.com/linskybing/platform-go/src/k8sclient"
	"github.com/linskybing/platform-go/src/minio"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateNs() {
	nsName := "my-namespace"
	// Step 1: Check if namespace already exists
	_, err := k8sclient.Clientset.CoreV1().Namespaces().Get(context.TODO(), nsName, metav1.GetOptions{})
	if err == nil {
		fmt.Printf("Namespace '%s' already exists, skipping creation\n", nsName)
		return
	}

	if !errors.IsNotFound(err) {
		log.Fatalf("Error querying namespace: %v", err)
	}

	// Step 2: Does not exist, create namespace
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: nsName,
		},
	}

	_, err = k8sclient.Clientset.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
	if err != nil {
		log.Fatalf("Failed to create namespace: %v", err)
	}

	fmt.Printf("Successfully created Namespace: %s\n", nsName)
}

func main() {
	config.LoadConfig()
	config.InitK8sConfig()
	db.Init()
	minio.InitMinio()
	k8sclient.Init()
	CreateNs()
}
