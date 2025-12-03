package main

// import (
// 	"context"
// 	"encoding/json"
// 	"fmt"
// 	"log"

// 	"github.com/linskybing/platform-go/src/config"
// 	"github.com/linskybing/platform-go/src/db"
// 	"github.com/linskybing/platform-go/src/k8sclient"
// 	"github.com/linskybing/platform-go/src/minio"
// 	"github.com/linskybing/platform-go/src/repositories"
// 	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
// 	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
// )

// func main() {
// 	config.LoadConfig()
// 	config.InitK8sConfig()
// 	db.Init()
// 	minio.InitMinio()
// 	k8sclient.Init()

// 	data, _ := repositories.ListResourcesByConfigFileID(1)
// 	rawJSON := data[1].ParsedYAML

// 	// decode
// 	var obj unstructured.Unstructured
// 	if err := json.Unmarshal(rawJSON, &obj.Object); err != nil {
// 		log.Fatalf("failed to parse JSON: %v", err)
// 	}

// 	gvk := obj.GroupVersionKind()
// 	mapping, err := k8sclient.Mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
// 	if err != nil {
// 		log.Fatalf("failed to get REST mapping: %v", err)
// 	}

// 	ns := obj.GetNamespace()
// 	if ns == "" {
// 		ns = "default"
// 	}

// 	resourceClient := k8sclient.DynamicClient.Resource(mapping.Resource).Namespace(ns)
// 	result, err := resourceClient.Create(context.Background(), &obj, metav1.CreateOptions{})
// 	//err = resourceClient.Delete(context.Background(), obj.GetName(), metav1.DeleteOptions{})
// 	//result, err := resourceClient.Update(context.Background(), &obj, metav1.UpdateOptions{})
// 	if err != nil {
// 		log.Fatalf("failed to create resource: %v", err)
// 	}
// 	fmt.Printf("Created %s/%s\n", result.GetKind(), result.GetName())
// 	//fmt.Printf("Updated %s/%s\n", result.GetKind(), result.GetName())
// }
