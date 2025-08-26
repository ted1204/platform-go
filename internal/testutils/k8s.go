package testutils

import (
	"context"

	"github.com/linskybing/platform-go/config"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func CreateFakeStorageClass() *fake.Clientset {
	client := fake.NewSimpleClientset()

	sc := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: config.DefaultStorageClassName,
		},
		Provisioner: "kubernetes.io/no-provisioner", // for testing
		VolumeBindingMode: func() *storagev1.VolumeBindingMode {
			mode := storagev1.VolumeBindingImmediate
			return &mode
		}(),
	}

	client.StorageV1().StorageClasses().Create(
		context.TODO(),
		sc,
		metav1.CreateOptions{},
	)

	return client
}
