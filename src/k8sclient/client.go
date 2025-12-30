package k8sclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/util/homedir"
)

type WebSocketIO struct {
	conn        *websocket.Conn
	stdinPipe   *io.PipeReader // Data from websocket -> stdin
	stdinWriter *io.PipeWriter
	sizeChan    chan remotecommand.TerminalSize
	once        sync.Once
}

type TerminalMessage struct {
	Type string `json:"type"`
	Data string `json:"data,omitempty"` // For stdin/stdout
	Cols int    `json:"cols,omitempty"` // For resize
	Rows int    `json:"rows,omitempty"` // For resize
}

var (
	Config        *rest.Config
	Clientset     *kubernetes.Clientset
	Dc            *discovery.DiscoveryClient
	Resources     []*restmapper.APIGroupResources
	Mapper        meta.RESTMapper
	DynamicClient *dynamic.DynamicClient
)

func InitTestCluster() {
	// In test environment, we might not have a real K8s cluster.
	// If KUBECONFIG is not set, we skip initialization or use a fake client if possible.
	// For integration tests that don't strictly require K8s, we can make this optional.
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		log.Println("KUBECONFIG is not set, skipping K8s cluster initialization")
		return
	}

	var err error
	Config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		log.Fatalf("failed to load kubeconfig: %v", err)
	}

	Clientset, err = kubernetes.NewForConfig(Config)
	if err != nil {
		log.Fatalf("failed to create clientset: %v", err)
	}

	server := Config.Host
	if !strings.Contains(server, "127.0.0.1") && !strings.Contains(server, "localhost") {
		log.Fatalf("unsafe cluster detected: %s, abort test", server)
	}

	Dc, err = discovery.NewDiscoveryClientForConfig(Config)
	if err != nil {
		log.Fatalf("failed to create discovery client: %v", err)
	}

	Resources, err = restmapper.GetAPIGroupResources(Dc)
	if err != nil {
		log.Fatalf("failed to get API group resources: %v", err)
	}
	Mapper = restmapper.NewDiscoveryRESTMapper(Resources)

	DynamicClient, err = dynamic.NewForConfig(Config)
	if err != nil {
		log.Fatalf("failed to create dynamic client: %v", err)
	}
}

func Init() {
	var err error
	if configPath := os.Getenv("KUBECONFIG"); configPath != "" {
		Config, err = clientcmd.BuildConfigFromFlags("", configPath)
	} else {
		Config, err = rest.InClusterConfig()
		if err != nil {
			kubeconfig := filepath.Join(homedir.HomeDir(), ".kube", "config")
			Config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		}
	}
	if err != nil {
		log.Fatalf("failed to load kube config: %v", err)
	}
	Clientset, err = kubernetes.NewForConfig(Config)
	if err != nil {
		log.Fatalf("failed to create kubernetes clientset: %v", err)
	}
	Dc, err = discovery.NewDiscoveryClientForConfig(Config)
	if err != nil {
		log.Fatalf("failed to create Discovery client: %v", err)
	}
	Resources, err = restmapper.GetAPIGroupResources(Dc)
	if err != nil {
		log.Fatalf("failed to get api group resources: %v", err)
	}
	Mapper = restmapper.NewDiscoveryRESTMapper(Resources)
	DynamicClient, err = dynamic.NewForConfig(Config)
	if err != nil {
		log.Fatalf("failed to create dynamic client: %v", err)
	}
}

// NewWebSocketIO creates a new WebSocketIO handler
func NewWebSocketIO(conn *websocket.Conn) *WebSocketIO {
	pr, pw := io.Pipe()
	handler := &WebSocketIO{
		conn:        conn,
		stdinPipe:   pr,
		stdinWriter: pw,
		sizeChan:    make(chan remotecommand.TerminalSize),
	}
	// Unchanged here, still start readLoop
	go handler.readLoop()
	return handler
}

// Read reads data from the pipe receiving stdin data (implements io.Reader)
func (h *WebSocketIO) Read(p []byte) (n int, err error) {
	return h.stdinPipe.Read(p)
}

// Write writes data to WebSocket (implements io.Writer)
func (h *WebSocketIO) Write(p []byte) (n int, err error) {
	msg, err := json.Marshal(TerminalMessage{
		Type: "stdout",
		Data: string(p),
	})
	if err != nil {
		return 0, err
	}
	if err := h.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
		return 0, err
	}
	return len(p), nil
}

// Next is called by executor to wait for a resize event (implements remotecommand.TerminalSizeQueue)
func (h *WebSocketIO) Next() *remotecommand.TerminalSize {
	size, ok := <-h.sizeChan
	if !ok {
		return nil // Channel closed
	}
	return &size
}

// Close cleans up resources
func (h *WebSocketIO) Close() {
	h.once.Do(func() {
		_ = h.stdinWriter.Close()
		close(h.sizeChan)
	})
}

// readLoop is the core logic, continuously reading WebSocket messages in the background and dispatching them
func (h *WebSocketIO) readLoop() {
	// Core fix: readLoop is responsible for cleanup
	// When this goroutine exits (whether normally or due to error),
	// defer ensures Close() is called, safely closing channels.
	defer h.Close()

	for {
		_, message, err := h.conn.ReadMessage()
		if err != nil {
			// When a read error occurs (e.g., WebSocket closed),
			// the for loop terminates, and the defer h.Close() above executes.
			return
		}

		var msg TerminalMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}

		switch msg.Type {
		case "stdin":
			if msg.Data != "" {
				// Here, since the loop is continuing, the channel is definitely open.
				_, _ = h.stdinWriter.Write([]byte(msg.Data))
			}
		case "resize":
			// Same as above, the channel is definitely open.
			h.sizeChan <- remotecommand.TerminalSize{
				Width:  uint16(msg.Cols),
				Height: uint16(msg.Rows),
			}
		}
	}
}

// WebSocketIO's code remains the same, it's correct.
// ... NewWebSocketIO, Read, Write, Next, Close, readLoop ...

func ExecToPodViaWebSocket(
	conn *websocket.Conn,
	config *rest.Config,
	clientset *kubernetes.Clientset,
	namespace, podName, container string,
	command []string,
	tty bool,
) error {
	// Create our handler which implements all necessary interfaces.
	wsIO := NewWebSocketIO(conn)

	// CORE FIX: Remove the defer from the main goroutine.
	// The responsibility of closing the channels is now solely
	// within the readLoop goroutine. This eliminates the race condition.
	// defer wsIO.Close()  <-- REMOVE THIS LINE

	execCmd := []string{
		"env",
		"TERM=xterm",
	}
	execCmd = append(execCmd, command...) // Append the original command (e.g., "/bin/sh")

	req := clientset.CoreV1().RESTClient().
		Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: container,
			// Use the modified command with the TERM variable.
			Command: execCmd,
			Stdin:   true,
			Stdout:  true,
			Stderr:  true,
			TTY:     tty,
		}, scheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return err
	}

	return executor.Stream(remotecommand.StreamOptions{
		Stdin:             wsIO,
		Stdout:            wsIO,
		Stderr:            wsIO,
		Tty:               tty,
		TerminalSizeQueue: wsIO,
	})
}

func WatchNamespaceResources(ctx context.Context, writeChan chan<- []byte, namespace string) {
	gvrs := []schema.GroupVersionResource{
		{Group: "", Version: "v1", Resource: "pods"},
		{Group: "", Version: "v1", Resource: "services"},
		{Group: "apps", Version: "v1", Resource: "deployments"},
	}

	var wg sync.WaitGroup
	for _, gvr := range gvrs {
		wg.Add(1)
		go func(gvr schema.GroupVersionResource) {
			defer wg.Done()
			watchAndSend(ctx, DynamicClient, gvr, namespace, writeChan)
		}(gvr)
	}

	go func() {
		<-ctx.Done()
		wg.Wait()
		close(writeChan)
	}()
}

// WatchNamespaceResources monitors resource changes in a single namespace
func WatchUserNamespaceResources(ctx context.Context, namespace string, writeChan chan<- []byte) {
	gvrs := []schema.GroupVersionResource{
		{Group: "", Version: "v1", Resource: "pods"},
		{Group: "", Version: "v1", Resource: "services"},
		{Group: "apps", Version: "v1", Resource: "deployments"},
	}

	// Wait synchronously for all resource monitoring to finish
	var wg sync.WaitGroup

	// Start a monitoring goroutine for each resource
	for _, gvr := range gvrs {
		wg.Add(1)
		go func(gvr schema.GroupVersionResource) {
			defer wg.Done()
			watchUserAndSend(ctx, namespace, gvr, writeChan)
		}(gvr)
	}

	// Wait for all goroutines to finish
	wg.Wait()
}

func watchUserAndSend(ctx context.Context, namespace string, gvr schema.GroupVersionResource, writeChan chan<- []byte) {
	sendObject := func(eventType string, obj *unstructured.Unstructured) error {
		data := buildDataMap(eventType, obj)
		msg, err := json.Marshal(data)
		if err != nil {
			return err
		}

		select {
		case writeChan <- msg:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second * 10): // Add timeout to avoid deadlock
			return fmt.Errorf("timeout sending message")
		}
	}

	// initial list of resources
	list, err := DynamicClient.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, item := range list.Items {
			if err := sendObject("ADDED", &item); err != nil {
				fmt.Printf("Failed to send list item: %v\n", err)
			}
		}
	} else {
		fmt.Printf("List error for %s.%s: %v\n", gvr.Resource, gvr.Group, err)
	}

	// watch loop
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second * 30): // Reconnect every 30 seconds
			// Reconnect watch every 30 seconds
			watcher, err := DynamicClient.Resource(gvr).Namespace(namespace).Watch(ctx, metav1.ListOptions{})
			if err != nil {
				fmt.Printf("Failed to start watch: %v\n", err)
				continue
			}

			// Handle watcher channel
			func() {
				defer watcher.Stop()
				for {
					select {
					case <-ctx.Done():
						return
					case event, ok := <-watcher.ResultChan():
						if !ok {
							return
						}
						if obj, ok := event.Object.(*unstructured.Unstructured); ok {
							if err := sendObject(string(event.Type), obj); err != nil {
								fmt.Printf("Failed to send watch event: %v\n", err)
							}
						}
					}
				}
			}()
		}
	}
}

func watchAndSend(
	ctx context.Context,
	dynClient dynamic.Interface,
	gvr schema.GroupVersionResource,
	ns string,
	writeChan chan<- []byte,
) {
	sendObject := func(eventType string, obj *unstructured.Unstructured) error {
		data := buildDataMap(eventType, obj)
		msg, err := json.Marshal(data)
		if err != nil {
			return err
		}

		select {
		case writeChan <- msg:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	// initial list
	list, err := dynClient.Resource(gvr).Namespace(ns).List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, item := range list.Items {
			if err := sendObject("ADDED", &item); err != nil {
				fmt.Printf("Failed to send list item: %v\n", err)
			}
		}
	} else {
		fmt.Printf("List error for %s.%s: %v\n", gvr.Resource, gvr.Group, err)
	}

	// watch loop
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		watcher, err := dynClient.Resource(gvr).Namespace(ns).Watch(ctx, metav1.ListOptions{})
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}

		for {
			select {
			case <-ctx.Done():
				watcher.Stop()
				return
			case event, ok := <-watcher.ResultChan():
				if !ok {
					return
				}

				obj, ok := event.Object.(*unstructured.Unstructured)
				if !ok {
					continue
				}

				if err := sendObject(string(event.Type), obj); err != nil {
					fmt.Printf("Failed to send watch event: %v\n", err)
				}
			}
		}
	}
}
func buildDataMap(eventType string, obj *unstructured.Unstructured) map[string]interface{} {
	data := map[string]interface{}{
		"type": eventType,
		"kind": obj.GetKind(),
		"name": obj.GetName(),
		"ns":   obj.GetNamespace(),
	}

	for k, v := range extractStatusFields(obj) {
		data[k] = v
	}

	if obj.GetKind() == "Pod" {
		if containers, found, _ := unstructured.NestedSlice(obj.Object, "spec", "containers"); found {
			var containerNames []string
			for _, c := range containers {
				if m, ok := c.(map[string]interface{}); ok {
					if name, ok := m["name"].(string); ok {
						containerNames = append(containerNames, name)
					}
				}
			}
			if len(containerNames) > 0 {
				data["containers"] = containerNames
			}
		}
	}

	if isService(obj) {
		if ips := extractServiceExternalIPs(obj); len(ips) > 0 {
			data["externalIPs"] = ips
		}
		if ports := extractServiceNodePorts(obj); len(ports) > 0 {
			data["nodePorts"] = ports
		}
	}

	return data
}

func isService(obj *unstructured.Unstructured) bool {
	return obj.GetKind() == "Service"
}

func extractServiceExternalIPs(obj *unstructured.Unstructured) []string {
	var externalIPs []string

	specExternalIPs, found, err := unstructured.NestedSlice(obj.Object, "spec", "externalIPs")
	if found && err == nil {
		for _, ip := range specExternalIPs {
			if s, ok := ip.(string); ok {
				externalIPs = append(externalIPs, s)
			}
		}
	}

	ingressList, found, err := unstructured.NestedSlice(obj.Object, "status", "loadBalancer", "ingress")
	if found && err == nil {
		for _, ingress := range ingressList {
			if m, ok := ingress.(map[string]interface{}); ok {
				if ip, ok := m["ip"].(string); ok {
					externalIPs = append(externalIPs, ip)
				}
			}
		}
	}

	return externalIPs
}

func extractServiceNodePorts(obj *unstructured.Unstructured) []int64 {
	var nodePorts []int64

	ports, found, err := unstructured.NestedSlice(obj.Object, "spec", "ports")
	if !found || err != nil {
		return nodePorts
	}

	for _, port := range ports {
		if m, ok := port.(map[string]interface{}); ok {
			if np, ok := m["nodePort"].(int64); ok {
				nodePorts = append(nodePorts, np)
			} else if npf, ok := m["nodePort"].(float64); ok {
				nodePorts = append(nodePorts, int64(npf))
			}
		}
	}

	return nodePorts
}

func getWatchableNamespacedResources(dc *discovery.DiscoveryClient) ([]schema.GroupVersionResource, error) {
	apiResourceLists, err := dc.ServerPreferredNamespacedResources()
	if err != nil {
		return nil, err
	}

	var result []schema.GroupVersionResource
	for _, apiList := range apiResourceLists {
		gv, err := schema.ParseGroupVersion(apiList.GroupVersion)
		if err != nil {
			continue
		}
		for _, r := range apiList.APIResources {
			if r.Namespaced && contains(r.Verbs, "watch") && !strings.Contains(r.Name, "/") {
				result = append(result, schema.GroupVersionResource{
					Group:    gv.Group,
					Version:  gv.Version,
					Resource: r.Name,
				})
			}
		}
	}
	return result, nil
}

func contains(sl []string, s string) bool {
	for _, item := range sl {
		if item == s {
			return true
		}
	}
	return false
}

func extractStatusFields(obj *unstructured.Unstructured) map[string]interface{} {
	kind := obj.GetKind()
	result := map[string]interface{}{}

	switch kind {
	case "Pod":
		if phase, found, _ := unstructured.NestedString(obj.Object, "status", "phase"); found {
			result["status"] = phase
		}
	case "Service":
		if clusterIP, found, _ := unstructured.NestedString(obj.Object, "spec", "clusterIP"); found {
			result["clusterIP"] = clusterIP
		}
		if externalIPs, found, _ := unstructured.NestedSlice(obj.Object, "status", "loadBalancer", "ingress"); found && len(externalIPs) > 0 {
			if ingressMap, ok := externalIPs[0].(map[string]interface{}); ok {
				if ip, ok := ingressMap["ip"].(string); ok {
					result["externalIP"] = ip
				}
				if hostname, ok := ingressMap["hostname"].(string); ok {
					result["externalHostname"] = hostname
				}
			}
		}
	case "Ingress":
		if externalIPs, found, _ := unstructured.NestedSlice(obj.Object, "status", "loadBalancer", "ingress"); found && len(externalIPs) > 0 {
			if ingressMap, ok := externalIPs[0].(map[string]interface{}); ok {
				if ip, ok := ingressMap["ip"].(string); ok {
					result["externalIP"] = ip
				}
				if hostname, ok := ingressMap["hostname"].(string); ok {
					result["externalHostname"] = hostname
				}
			}
		}
	case "Deployment", "ReplicaSet":
		if availableReplicas, found, _ := unstructured.NestedInt64(obj.Object, "status", "availableReplicas"); found {
			result["availableReplicas"] = availableReplicas
		}
	case "Job":
		if succeeded, found, _ := unstructured.NestedInt64(obj.Object, "status", "succeeded"); found {
			result["succeeded"] = succeeded
		}
	}

	return result
}

func GetFilteredNamespaces(filter string) ([]v1.Namespace, error) {
	namespaces, err := Clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list namespaces: %v", err)
	}

	var filteredNamespaces []v1.Namespace
	for _, ns := range namespaces.Items {
		if strings.Contains(ns.Name, filter) {
			filteredNamespaces = append(filteredNamespaces, ns)
		}
	}

	return filteredNamespaces, nil
}

type JobSpec struct {
	Name              string
	Namespace         string
	Image             string
	Command           []string
	PriorityClassName string
	Parallelism       int32
	Completions       int32
	Volumes           []VolumeSpec
	GPUCount          int
	GPUType           string
	EnvVars           map[string]string
	Annotations       map[string]string
}

type VolumeSpec struct {
	Name      string
	PVCName   string
	HostPath  string
	MountPath string
}

// CreateJob creates a Kubernetes Job with flexible configuration
func CreateJob(ctx context.Context, spec JobSpec) error {
	var volumes []corev1.Volume
	var volumeMounts []corev1.VolumeMount

	for _, v := range spec.Volumes {
		var volumeSource corev1.VolumeSource
		if v.PVCName != "" {
			volumeSource = corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: v.PVCName,
				},
			}
		} else if v.HostPath != "" {
			volumeSource = corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: v.HostPath,
				},
			}
		}

		volumes = append(volumes, corev1.Volume{
			Name:         v.Name,
			VolumeSource: volumeSource,
		})
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      v.Name,
			MountPath: v.MountPath,
		})
	}

	var env []corev1.EnvVar
	for k, v := range spec.EnvVars {
		env = append(env, corev1.EnvVar{
			Name:  k,
			Value: v,
		})
	}

	container := corev1.Container{
		Name:         spec.Name,
		Image:        spec.Image,
		Command:      spec.Command,
		VolumeMounts: volumeMounts,
		Env:          env,
	}

	if spec.GPUCount > 0 {
		qty := resource.MustParse(fmt.Sprintf("%d", spec.GPUCount))
		resourceName := corev1.ResourceName("nvidia.com/gpu")
		if spec.GPUType == "shared" {
			resourceName = corev1.ResourceName("nvidia.com/gpu.shared")
		}

		container.Resources = corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				resourceName: qty,
			},
			Requests: corev1.ResourceList{
				resourceName: qty,
			},
		}
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      spec.Name,
			Namespace: spec.Namespace,
		},
		Spec: batchv1.JobSpec{
			Parallelism: &spec.Parallelism,
			Completions: &spec.Completions,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: spec.Annotations,
				},
				Spec: corev1.PodSpec{
					RestartPolicy:     corev1.RestartPolicyOnFailure,
					PriorityClassName: spec.PriorityClassName,
					Volumes:           volumes,
					Containers: []corev1.Container{
						container,
					},
				},
			},
		},
	}

	_, err := Clientset.BatchV1().Jobs(spec.Namespace).Create(ctx, job, metav1.CreateOptions{})
	return err
}

// CreateFileBrowserPod creates a pod running filebrowser with optional read-only access
func CreateFileBrowserPod(ctx context.Context, ns, pvcName string, readOnly bool, baseURL string) (string, error) {
	podName := fmt.Sprintf("filebrowser-%s", pvcName)

	// Check if pod already exists
	_, err := Clientset.CoreV1().Pods(ns).Get(ctx, podName, metav1.GetOptions{})
	if err == nil {
		return podName, nil // Already exists
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: ns,
			Labels: map[string]string{
				"app": "filebrowser",
				"pvc": pvcName,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "filebrowser",
					Image: "filebrowser/filebrowser:latest",
					// Use --baseurl if needed for reverse proxy compatibility
					Args: []string{
						"--noauth",
						"--database", "/tmp/filebrowser.db",
						"--root", "/srv",
						"--port", "80",
						"--address", "0.0.0.0",
						"--baseURL", baseURL,
					},
					Ports: []corev1.ContainerPort{
						{ContainerPort: 8080},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "data",
							MountPath: "/srv",
							ReadOnly:  readOnly, // Set based on user role
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
		},
	}

	_, err = Clientset.CoreV1().Pods(ns).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return "", err
	}
	return podName, nil
}

// CreateFileBrowserService creates a service for filebrowser
func CreateFileBrowserService(ctx context.Context, ns, pvcName string) (string, error) {
	svcName := fmt.Sprintf("filebrowser-%s-svc", pvcName)

	// Check if service already exists
	svc, err := Clientset.CoreV1().Services(ns).Get(ctx, svcName, metav1.GetOptions{})
	if err == nil {
		// Return existing NodePort if available
		if len(svc.Spec.Ports) > 0 {
			return fmt.Sprintf("%d", svc.Spec.Ports[0].NodePort), nil
		}
		return "", nil
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svcName,
			Namespace: ns,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": "filebrowser",
				"pvc": pvcName,
			},
			Type: corev1.ServiceTypeNodePort,
			Ports: []corev1.ServicePort{
				{
					Port:       80,
					TargetPort: intstr.FromInt(80),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}

	createdSvc, err := Clientset.CoreV1().Services(ns).Create(ctx, service, metav1.CreateOptions{})
	if err != nil {
		return "", err
	}

	if len(createdSvc.Spec.Ports) > 0 {
		return fmt.Sprintf("%d", createdSvc.Spec.Ports[0].NodePort), nil
	}
	return "", nil
}

// DeleteFileBrowserResources deletes the pod and service
func DeleteFileBrowserResources(ctx context.Context, ns, pvcName string) error {
	podName := fmt.Sprintf("filebrowser-%s", pvcName)
	svcName := fmt.Sprintf("filebrowser-%s-svc", pvcName)

	// Delete Service
	err := Clientset.CoreV1().Services(ns).Delete(ctx, svcName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	// Delete Pod
	err = Clientset.CoreV1().Pods(ns).Delete(ctx, podName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	return nil
}
