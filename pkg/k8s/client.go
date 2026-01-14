package k8s

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
	"github.com/linskybing/platform-go/internal/config"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/util/homedir"
)

// WebSocketIO handles the conversion between WebSocket and io.Reader/Writer
// It implements remotecommand.TerminalSizeQueue, io.Reader, and io.Writer
type WebSocketIO struct {
	conn        *websocket.Conn
	stdinPipe   *io.PipeReader
	stdinWriter *io.PipeWriter
	sizeChan    chan remotecommand.TerminalSize
	once        sync.Once
	mu          sync.Mutex // Protects concurrent writes (Ping vs Stdout)
}

type TerminalMessage struct {
	Type string `json:"type"`
	Data string `json:"data,omitempty"` // For stdin/stdout
	Cols int    `json:"cols,omitempty"` // For resize
	Rows int    `json:"rows,omitempty"` // For resize
}

var (
	Config        *rest.Config
	Clientset     kubernetes.Interface
	Dc            *discovery.DiscoveryClient
	Resources     []*restmapper.APIGroupResources
	Mapper        meta.RESTMapper
	DynamicClient *dynamic.DynamicClient
)

func InitTestCluster() {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		log.Println("KUBECONFIG is not set, using fake Kubernetes client for tests")
		Clientset = k8sfake.NewSimpleClientset()
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
	Config.QPS = 50
	Config.Burst = 100
	DynamicClient, err = dynamic.NewForConfig(Config)
	if err != nil {
		log.Fatalf("failed to create dynamic client: %v", err)
	}
}

// NewWebSocketIO creates a new WebSocketIO handler and starts loops
func NewWebSocketIO(conn *websocket.Conn) *WebSocketIO {
	pr, pw := io.Pipe()

	// Context for internal coordination
	// ctx, cancel := context.WithCancel(context.Background())

	handler := &WebSocketIO{
		conn:        conn,
		stdinPipe:   pr,
		stdinWriter: pw,
		sizeChan:    make(chan remotecommand.TerminalSize),
		// cancel:      cancel,
	}

	// Start the main read loop (Standard Input from user)
	go handler.readLoop()
	// Start the ping loop (Heartbeat to client)
	go handler.pingLoop()

	return handler
}

// pingLoop sends periodic pings to keep the connection alive
func (h *WebSocketIO) pingLoop() {
	// Must be shorter than pongWait (60s)
	pingPeriod := 50 * time.Second
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for range ticker.C {
		// Lock before writing to prevent race condition with stdout
		h.mu.Lock()
		// Set write deadline to prevent hanging
		if err := h.conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
			h.mu.Unlock()
			h.Close()
			return
		}
		err := h.conn.WriteMessage(websocket.PingMessage, nil)
		h.mu.Unlock()

		if err != nil {
			// If ping fails, connection is likely dead. Close handler.
			h.Close()
			return
		}
	}
}

// Read reads data from the pipe receiving stdin data (implements io.Reader)
func (h *WebSocketIO) Read(p []byte) (n int, err error) {
	return h.stdinPipe.Read(p)
}

// Write writes data to WebSocket (stdout from Pod)
func (h *WebSocketIO) Write(p []byte) (n int, err error) {
	msg, err := json.Marshal(TerminalMessage{
		Type: "stdout",
		Data: string(p),
	})
	if err != nil {
		return 0, err
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// Update WriteDeadline before writing
	_ = h.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
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
// IMPORTANT: This method does NOT close sizeChan to avoid panics.
// sizeChan is closed by readLoop.
func (h *WebSocketIO) Close() {
	h.once.Do(func() {
		// Close stdinWriter to stop Read() calls if any
		_ = h.stdinWriter.Close()
		// We do NOT close sizeChan here because readLoop might be trying to send to it.
		// We do NOT close conn here immediately, we let readLoop handle the socket closure or wait for error.
	})
}

// readLoop is the core logic, continuously reading WebSocket messages in the background
func (h *WebSocketIO) readLoop() {
	// Cleanup when the loop exits
	defer func() {
		h.Close()          // Close pipes
		close(h.sizeChan)  // Close channel safely (ONLY here)
		_ = h.conn.Close() // Ensure underlying TCP connection is closed
	}()

	const pongWait = 60 * time.Second

	h.conn.SetReadLimit(512 * 1024)
	if err := h.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		return
	}

	h.conn.SetPongHandler(func(string) error {
		return h.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, message, err := h.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			return
		}

		// Refresh deadline on any message
		if err := h.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
			return
		}

		var msg TerminalMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}

		switch msg.Type {
		case "stdin":
			if msg.Data != "" {
				if _, err := h.stdinWriter.Write([]byte(msg.Data)); err != nil {
					log.Printf("Failed to write to stdin: %v", err)
				}
			}
		case "resize":
			// Non-blocking send to avoid hanging if SPDY executor isn't ready
			// and to avoid panic if sizeChan is closed (though with defer structure it should be safe)
			select {
			case h.sizeChan <- remotecommand.TerminalSize{
				Width:  uint16(msg.Cols),
				Height: uint16(msg.Rows),
			}:
			default:
				// Drop resize event if channel buffer is full or no one listening
			}
		}
	}
}

func ExecToPodViaWebSocket(
	conn *websocket.Conn,
	config *rest.Config,
	clientset *kubernetes.Clientset,
	namespace, podName, container string,
	command []string,
	tty bool,
) error {
	wsIO := NewWebSocketIO(conn)

	// DO NOT call defer wsIO.Close() here.
	// Lifecycle is managed by NewWebSocketIO's goroutines.

	execCmd := []string{
		"env",
		"TERM=xterm",
	}
	execCmd = append(execCmd, command...)

	req := clientset.CoreV1().RESTClient().
		Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: container,
			Command:   execCmd,
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       tty,
		}, scheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return err
	}

	// This blocks until the command finishes
	return executor.StreamWithContext(context.Background(), remotecommand.StreamOptions{
		Stdin:             wsIO,
		Stdout:            wsIO,
		Stderr:            wsIO,
		Tty:               tty,
		TerminalSizeQueue: wsIO,
	})
}

// WatchNamespaceResources monitors resources for a specific namespace
func WatchNamespaceResources(ctx context.Context, writeChan chan<- []byte, namespace string) {
	gvrs := []schema.GroupVersionResource{
		{Group: "", Version: "v1", Resource: "pods"},
		{Group: "", Version: "v1", Resource: "services"},
		{Group: "apps", Version: "v1", Resource: "deployments"},
	}

	var wg sync.WaitGroup
	for _, gvr := range gvrs {
		wg.Add(1)
		time.Sleep(50 * time.Millisecond) // Stagger start to be gentle on APIServer

		go func(gvr schema.GroupVersionResource) {
			defer wg.Done()
			watchAndSend(ctx, DynamicClient, gvr, namespace, writeChan)
		}(gvr)
	}

	// Wait for all watchers to finish (via context cancel) then close channel
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

	var wg sync.WaitGroup
	for _, gvr := range gvrs {
		wg.Add(1)
		go func(gvr schema.GroupVersionResource) {
			defer wg.Done()
			watchUserAndSend(ctx, namespace, gvr, writeChan)
		}(gvr)
	}

	// Wait logic handled by caller or context
	wg.Wait()
}

func watchUserAndSend(ctx context.Context, namespace string, gvr schema.GroupVersionResource, writeChan chan<- []byte) {
	// lastSnapshot holds last sent status signature per resource name
	lastSnapshot := make(map[string]string)

	sendObject := func(eventType string, obj *unstructured.Unstructured) error {
		name := obj.GetName()

		// Always send deletes
		if eventType != "DELETED" {
			// Compute compact snapshot to decide whether to send
			snap := statusSnapshotString(obj)
			if prev, ok := lastSnapshot[name]; ok {
				if prev == snap {
					// No meaningful status change, skip sending
					return nil
				}
			}
			lastSnapshot[name] = snap
		} else {
			// remove snapshot on delete
			delete(lastSnapshot, name)
		}

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
		default:
			// Prevent blocking if client is slow
			return fmt.Errorf("client buffer full, dropping message for %s", name)
		}
	}

	list, err := DynamicClient.Resource(gvr).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, item := range list.Items {
			_ = sendObject("ADDED", &item)
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second * 30):
			// Simple reconnection logic
			watcher, err := DynamicClient.Resource(gvr).Namespace(namespace).Watch(ctx, metav1.ListOptions{})
			if err != nil {
				continue
			}

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
							_ = sendObject(string(event.Type), obj)
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
	// lastSnapshot holds last sent status signature per resource name
	lastSnapshot := make(map[string]string)

	sendObject := func(eventType string, obj *unstructured.Unstructured) error {
		name := obj.GetName()

		if eventType != "DELETED" {
			snap := statusSnapshotString(obj)
			if prev, ok := lastSnapshot[name]; ok {
				if prev == snap {
					return nil
				}
			}
			lastSnapshot[name] = snap
		} else {
			delete(lastSnapshot, name)
		}

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

	// Initial List
	list, err := dynClient.Resource(gvr).Namespace(ns).List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, item := range list.Items {
			if err := sendObject("ADDED", &item); err != nil && ctx.Err() != context.Canceled {
				fmt.Printf("Failed to send list item: %v\n", err)
			}
		}
	} else if ctx.Err() == context.Canceled {
		return
	} else {
		fmt.Printf("List error for %s.%s: %v\n", gvr.Resource, gvr.Group, err)
	}

	// Watch Loop
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		watcher, err := dynClient.Resource(gvr).Namespace(ns).Watch(ctx, metav1.ListOptions{})
		if err != nil {
			if ctx.Err() == context.Canceled {
				return
			}
			time.Sleep(5 * time.Second)
			continue
		}

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

					obj, ok := event.Object.(*unstructured.Unstructured)
					if !ok {
						continue
					}

					if err := sendObject(string(event.Type), obj); err != nil && ctx.Err() != context.Canceled {
						fmt.Printf("Failed to send watch event: %v\n", err)
					}
				}
			}
		}()
	}
}

// buildDataMap extracts comprehensive details from K8s resources for the frontend
func buildDataMap(eventType string, obj *unstructured.Unstructured) map[string]interface{} {
	data := map[string]interface{}{
		"type": eventType,
		"kind": obj.GetKind(),
		"name": obj.GetName(),
		"ns":   obj.GetNamespace(),
	}

	metadata := map[string]interface{}{}
	if ts, found, _ := unstructured.NestedString(obj.Object, "metadata", "creationTimestamp"); found {
		metadata["creationTimestamp"] = ts
	}
	if labels, found, _ := unstructured.NestedStringMap(obj.Object, "metadata", "labels"); found {
		metadata["labels"] = labels
	}
	if dts, found, _ := unstructured.NestedString(obj.Object, "metadata", "deletionTimestamp"); found {
		metadata["deletionTimestamp"] = dts
	}
	data["metadata"] = metadata

	for k, v := range extractStatusFields(obj) {
		data[k] = v
	}

	if obj.GetKind() == "Pod" {
		if containers, found, _ := unstructured.NestedSlice(obj.Object, "spec", "containers"); found {
			var containerNames []string
			var images []string
			for _, c := range containers {
				if m, ok := c.(map[string]interface{}); ok {
					if name, ok := m["name"].(string); ok {
						containerNames = append(containerNames, name)
					}
					if image, ok := m["image"].(string); ok {
						images = append(images, image)
					}
				}
			}
			if len(containerNames) > 0 {
				data["containers"] = containerNames
			}
			if len(images) > 0 {
				data["images"] = images
			}
		}

		if containerStatuses, found, _ := unstructured.NestedSlice(obj.Object, "status", "containerStatuses"); found {
			var totalRestarts int64 = 0
			for _, cs := range containerStatuses {
				if m, ok := cs.(map[string]interface{}); ok {
					if rc, ok := m["restartCount"].(int64); ok {
						totalRestarts += rc
					}
				}
			}
			data["restartCount"] = totalRestarts
		}
	}

	if isService(obj) {
		if ips := extractServiceExternalIPs(obj); len(ips) > 0 {
			data["externalIPs"] = ips
		}
		if ports := extractServiceNodePorts(obj); len(ports) > 0 {
			data["nodePorts"] = ports
		}
		if ports := extractServicePorts(obj); len(ports) > 0 {
			data["ports"] = ports
		}
	}

	return data
}

func extractServicePorts(obj *unstructured.Unstructured) []string {
	var servicePorts []string
	ports, found, err := unstructured.NestedSlice(obj.Object, "spec", "ports")
	if !found || err != nil {
		return servicePorts
	}

	for _, port := range ports {
		if m, ok := port.(map[string]interface{}); ok {
			p, okPort := m["port"].(int64)
			proto, okProto := m["protocol"].(string)

			if okPort {
				portStr := fmt.Sprintf("%d", p)
				if okProto {
					portStr = fmt.Sprintf("%d/%s", p, proto)
				}
				servicePorts = append(servicePorts, portStr)
			}
		}
	}
	return servicePorts
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

func extractStatusFields(obj *unstructured.Unstructured) map[string]interface{} {
	kind := obj.GetKind()
	result := map[string]interface{}{}

	switch kind {
	case "Pod":
		if phase, found, _ := unstructured.NestedString(obj.Object, "status", "phase"); found {
			result["status"] = phase
		}

		// Detect CrashLoopBackOff by inspecting containerStatuses.state.waiting.reason
		if containerStatuses, found, _ := unstructured.NestedSlice(obj.Object, "status", "containerStatuses"); found {
			var crashContainers []string
			for _, cs := range containerStatuses {
				m, ok := cs.(map[string]interface{})
				if !ok {
					continue
				}

				// container name
				name := ""
				if n, ok := m["name"].(string); ok {
					name = n
				}

				if state, ok := m["state"].(map[string]interface{}); ok {
					if waiting, ok := state["waiting"].(map[string]interface{}); ok {
						if reason, ok := waiting["reason"].(string); ok {
							if strings.Contains(reason, "CrashLoopBackOff") {
								crashContainers = append(crashContainers, name)
								// prefer reporting CrashLoopBackOff as the pod status for UI clarity
								result["status"] = "CrashLoopBackOff"
								result["statusReason"] = reason
								break
							}
						}
						// also check message if reason not present
						if msg, ok := waiting["message"].(string); ok && strings.Contains(msg, "CrashLoopBackOff") {
							crashContainers = append(crashContainers, name)
							result["status"] = "CrashLoopBackOff"
							result["statusReason"] = msg
							break
						}
					}
				}
			}

			if len(crashContainers) > 0 {
				result["crashLoopContainers"] = crashContainers
			}
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

// statusSnapshotString produces a compact, stable string representing the
// resource's status-related fields used for change detection. Keep this small
// to avoid expensive allocations; it's used to deduplicate frequent identical
// events (e.g. unrelated metadata updates).
func statusSnapshotString(obj *unstructured.Unstructured) string {
	m := map[string]interface{}{}

	// include kind/name for clarity (not strictly necessary for map key)
	m["kind"] = obj.GetKind()
	m["name"] = obj.GetName()

	// metadata.deletionTimestamp if present
	if dts, found, _ := unstructured.NestedString(obj.Object, "metadata", "deletionTimestamp"); found {
		m["deletionTimestamp"] = dts
	}

	// include extractStatusFields output (only status-related keys)
	for k, v := range extractStatusFields(obj) {
		m[k] = v
	}

	// For Pods, also include container restart counts and waiting reasons
	if obj.GetKind() == "Pod" {
		if containerStatuses, found, _ := unstructured.NestedSlice(obj.Object, "status", "containerStatuses"); found {
			var csSnap []map[string]interface{}
			for _, cs := range containerStatuses {
				if cm, ok := cs.(map[string]interface{}); ok {
					entry := map[string]interface{}{}
					if name, ok := cm["name"].(string); ok {
						entry["name"] = name
					}
					if rc, ok := cm["restartCount"].(int64); ok {
						entry["restartCount"] = rc
					} else if rcf, ok := cm["restartCount"].(float64); ok {
						entry["restartCount"] = int64(rcf)
					}
					if state, ok := cm["state"].(map[string]interface{}); ok {
						if waiting, ok := state["waiting"].(map[string]interface{}); ok {
							if reason, ok := waiting["reason"].(string); ok {
								entry["waitingReason"] = reason
							}
							if msg, ok := waiting["message"].(string); ok {
								entry["waitingMessage"] = msg
							}
						}
					}
					csSnap = append(csSnap, entry)
				}
			}
			if len(csSnap) > 0 {
				m["containerStatuses"] = csSnap
			}
		}
	}

	// Marshal into compact JSON string for easy equality checks
	bs, _ := json.Marshal(m)
	return string(bs)
}

func GetFilteredNamespaces(filter string) ([]corev1.Namespace, error) {
	namespaces, err := Clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list namespaces: %v", err)
	}

	var filteredNamespaces []corev1.Namespace
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
	CPURequest        string
	MemoryRequest     string
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

	resources := corev1.ResourceRequirements{
		Limits:   corev1.ResourceList{},
		Requests: corev1.ResourceList{},
	}

	if spec.GPUCount > 0 {
		qty := resource.MustParse(fmt.Sprintf("%d", spec.GPUCount))
		resourceName := corev1.ResourceName("nvidia.com/gpu")
		if spec.GPUType == "shared" {
			resourceName = corev1.ResourceName("nvidia.com/gpu.shared")
		}

		resources.Limits[resourceName] = qty
		resources.Requests[resourceName] = qty
	}

	if spec.CPURequest != "" {
		if q, err := resource.ParseQuantity(spec.CPURequest); err == nil {
			resources.Requests[corev1.ResourceCPU] = q
		}
	}

	if spec.MemoryRequest != "" {
		if q, err := resource.ParseQuantity(spec.MemoryRequest); err == nil {
			resources.Requests[corev1.ResourceMemory] = q
		}
	}

	container.Resources = resources

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

// DeleteJob deletes a Kubernetes Job and its pods.
func DeleteJob(ctx context.Context, namespace, name string) error {
	propagation := metav1.DeletePropagationForeground
	return Clientset.BatchV1().Jobs(namespace).Delete(ctx, name, metav1.DeleteOptions{PropagationPolicy: &propagation})
}

// CreateFileBrowserPod creates a pod running filebrowser with multiple PVC mounts
func CreateFileBrowserPod(ctx context.Context, ns string, pvcNames []string, readOnly bool, baseURL string) (string, error) {
	if len(pvcNames) == 0 {
		return "", fmt.Errorf("no PVCs provided for filebrowser")
	}

	podName := "filebrowser-project"

	existingPod, err := Clientset.CoreV1().Pods(ns).Get(ctx, podName, metav1.GetOptions{})
	if err == nil {
		matches := true
		if len(existingPod.Spec.Containers) > 0 {
			for _, m := range existingPod.Spec.Containers[0].VolumeMounts {
				if m.ReadOnly != readOnly {
					matches = false
					break
				}
			}
		}

		if matches {
			return podName, nil
		}

		grace := int64(0)
		_ = Clientset.CoreV1().Pods(ns).Delete(ctx, podName, metav1.DeleteOptions{GracePeriodSeconds: &grace})
		_ = wait.PollUntilContextTimeout(ctx, 200*time.Millisecond, 5*time.Second, true, func(ctx context.Context) (bool, error) {
			_, err := Clientset.CoreV1().Pods(ns).Get(ctx, podName, metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			return false, nil
		})
	}

	var volumes []corev1.Volume
	var mounts []corev1.VolumeMount
	for idx, pvc := range pvcNames {
		volName := fmt.Sprintf("data-%d", idx)
		volumes = append(volumes, corev1.Volume{
			Name: volName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvc},
			},
		})
		mounts = append(mounts, corev1.VolumeMount{
			Name:      volName,
			MountPath: "/srv",
			ReadOnly:  readOnly,
		})
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: ns,
			Labels: map[string]string{
				"app":  "filebrowser",
				"role": "project-storage",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "filebrowser",
					Image: "filebrowser/filebrowser:latest",
					Args: []string{
						"--noauth",
						"--database", "/tmp/filebrowser.db",
						"--root", "/srv",
						"--port", "80",
						"--address", "0.0.0.0",
						"--baseURL", baseURL,
					},
					Ports:        []corev1.ContainerPort{{ContainerPort: 80}},
					VolumeMounts: mounts,
				},
			},
			Volumes: volumes,
		},
	}

	_, err = Clientset.CoreV1().Pods(ns).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return "", err
	}
	return podName, nil
}

func CreateFileBrowserService(ctx context.Context, ns string) (string, error) {
	svcName := config.ProjectStorageBrowserSVCName

	svc, err := Clientset.CoreV1().Services(ns).Get(ctx, svcName, metav1.GetOptions{})
	if err == nil {
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
				"app":  "filebrowser",
				"role": "project-storage",
			},
			Type: corev1.ServiceTypeClusterIP,
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

func DeleteFileBrowserResources(ctx context.Context, ns string) error {
	podName := "filebrowser-project"
	svcName := config.ProjectStorageBrowserSVCName

	err := Clientset.CoreV1().Services(ns).Delete(ctx, svcName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	err = Clientset.CoreV1().Pods(ns).Delete(ctx, podName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	return nil
}
