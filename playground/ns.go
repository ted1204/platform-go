package main

import (
	"context"
	"fmt"
	"log"

	"github.com/linskybing/platform-go/config"
	"github.com/linskybing/platform-go/db"
	"github.com/linskybing/platform-go/k8sclient"
	"github.com/linskybing/platform-go/minio"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateNs() {
	nsName := "my-namespace"
	// Step 1: 檢查 namespace 是否已存在
	_, err := k8sclient.Clientset.CoreV1().Namespaces().Get(context.TODO(), nsName, metav1.GetOptions{})
	if err == nil {
		fmt.Printf("Namespace '%s' 已存在，跳過創建\n", nsName)
		return
	}

	if !errors.IsNotFound(err) {
		log.Fatalf("查詢 namespace 錯誤: %v", err)
	}

	// Step 2: 不存在，創建 namespace
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: nsName,
		},
	}

	_, err = k8sclient.Clientset.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
	if err != nil {
		log.Fatalf("創建 namespace 失敗: %v", err)
	}

	fmt.Printf("成功創建 Namespace: %s\n", nsName)
}

func main() {
	config.LoadConfig()
	config.InitK8sConfig()
	db.Init()
	minio.InitMinio()
	k8sclient.Init()
	CreateNs()
}
