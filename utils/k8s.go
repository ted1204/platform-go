package utils

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/internalversion/scheme"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
)

func ValidateK8sJSON(jsonStr string) (*schema.GroupVersionKind, string, error) {
	decoder := json.NewSerializerWithOptions(
		json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme,
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
