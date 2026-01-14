package k8s

import (
	"context"
	applyJson "encoding/json"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func ValidateK8sJSON(jsonBytes []byte) (*schema.GroupVersionKind, string, error) {
	obj := &unstructured.Unstructured{}

	if err := obj.UnmarshalJSON(jsonBytes); err != nil {
		return nil, "", fmt.Errorf("failed to unmarshal JSON to Kubernetes object: %w", err)
	}

	gvk := obj.GroupVersionKind()
	if gvk.Kind == "" || gvk.Version == "" {
		return nil, "", fmt.Errorf("object is missing kind or apiVersion")
	}

	name := obj.GetName()
	if name == "" {
		return nil, "", fmt.Errorf("object is missing metadata.name")
	}

	return &gvk, name, nil
}

func CreateByJson(jsonStr []byte, ns string) error {
	if Mapper == nil || DynamicClient == nil {
		fmt.Printf("[MOCK] Created resource by JSON in namespace %s\n", ns)
		return nil
	}
	// decode
	var obj unstructured.Unstructured
	if err := applyJson.Unmarshal(jsonStr, &obj.Object); err != nil {
		return err
	}

	gvk := obj.GroupVersionKind()
	mapping, err := Mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return err
	}

	if ns == "" {
		ns = "default"
	}
	resourceClient := DynamicClient.Resource(mapping.Resource).Namespace(ns)
	result, err := resourceClient.Create(context.TODO(), &obj, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	fmt.Printf("Created %s/%s\n", result.GetKind(), result.GetName())
	return nil
}

func DeleteByJson(jsonStr []byte, ns string) error {
	if Mapper == nil || DynamicClient == nil {
		fmt.Printf("[MOCK] Deleted resource by JSON in namespace %s\n", ns)
		return nil
	}
	// decode
	var obj unstructured.Unstructured
	if err := applyJson.Unmarshal(jsonStr, &obj.Object); err != nil {
		return err
	}

	gvk := obj.GroupVersionKind()
	mapping, err := Mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return err
	}

	if ns == "" {
		ns = "default"
	}
	resourceClient := DynamicClient.Resource(mapping.Resource).Namespace(ns)
	policy := metav1.DeletePropagationBackground
	err = resourceClient.Delete(context.TODO(), obj.GetName(), metav1.DeleteOptions{PropagationPolicy: &policy})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return nil
}

func UpdateByJson(jsonStr []byte, ns string) error {
	if Mapper == nil || DynamicClient == nil {
		fmt.Printf("[MOCK] Updated resource by JSON in namespace %s\n", ns)
		return nil
	}
	// decode
	var obj unstructured.Unstructured
	if err := applyJson.Unmarshal(jsonStr, &obj.Object); err != nil {
		return err
	}

	gvk := obj.GroupVersionKind()
	mapping, err := Mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return err
	}

	if ns == "" {
		ns = "default"
	}
	resourceClient := DynamicClient.Resource(mapping.Resource).Namespace(ns)
	result, err := resourceClient.Update(context.TODO(), &obj, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	fmt.Printf("Updated %s/%s\n", result.GetKind(), result.GetName())
	return nil
}
