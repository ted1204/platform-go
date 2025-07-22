package utils

import (
	"context"
	applyJson "encoding/json"
	"fmt"
	"log"

	"github.com/linskybing/platform-go/config"
	"github.com/linskybing/platform-go/k8sclient"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
)

func ValidateK8sJSON(jsonStr string) (*schema.GroupVersionKind, string, error) {
	decoder := json.NewSerializerWithOptions(
		json.DefaultMetaFactory, config.Scheme, config.Scheme,
		json.SerializerOptions{Yaml: false, Pretty: false, Strict: true},
	)
	obj, gvk, err := decoder.Decode([]byte(jsonStr), nil, nil)
	if err != nil {
		return nil, "", err
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

func CreateByJson(jsonStr []byte, ns string) error {
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
	fmt.Printf("✅ Created %s/%s\n", result.GetKind(), result.GetName())
	return nil
}

func DeleteByJson(jsonStr []byte, ns string) error {
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
		return err
	}
	return nil
}

func UpdateByJson(jsonStr []byte, ns string) error {
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
	fmt.Printf("✅ Updated %s/%s\n", result.GetKind(), result.GetName())
	return nil
}

func CreateNamespace(name string) {
	_, err := k8sclient.Clientset.CoreV1().Namespaces().Get(context.TODO(), name, metav1.GetOptions{})
	if err == nil {
		fmt.Printf("namespace %s already exist \n", name)
		return
	}

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	_, err = k8sclient.Clientset.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{})
	if err != nil {
		log.Fatalf("failed create namespace: %v", err)
	}

	fmt.Printf("create Namespace: %s successfully\n", name)
}
