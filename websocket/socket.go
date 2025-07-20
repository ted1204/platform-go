package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"your_project/config"
	"your_project/k8sclient"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true }, // 視需求可加強
}

// SetupWebSocket 設定 http.Server 用的 Upgrade 路由
func SetupWebSocket(mux *http.ServeMux) {
	mux.HandleFunc("/ws/terminal", terminalHandler)
	mux.HandleFunc("/ws/informer", informerHandler)
}

func terminalHandler(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Failed to upgrade websocket:", err)
		return
	}
	defer ws.Close()

	// 解析 query
	q := r.URL.Query()
	tokenStr := q.Get("token")
	podName := q.Get("pod")
	containerName := q.Get("container")

	if tokenStr == "" || podName == "" || containerName == "" {
		ws.WriteControl(websocket.CloseMessage,
			websocket.FormatCloseMessage(4002, "Missing parameters"), 
			// set a reasonable deadline if needed
			// time.Now().Add(time.Second*5),
		)
		return
	}

	// 解析 JWT，取得 username 當 namespace
	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(config.JWTSecret), nil
	})
	if err != nil || !token.Valid {
		ws.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(4003, "Invalid token"), 0)
		return
	}

	namespace, ok := claims["username"].(string)
	if !ok || namespace == "" {
		ws.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(4002, "Missing namespace"), 0)
		return
	}

	log.Printf("[Terminal] Client connected terminal for namespace: %s", namespace)

	// 使用 channel 作為 stdin stream
	stdinChan := make(chan []byte)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 讀取 ws message 寫入 stdinChan
	go func() {
		defer close(stdinChan)
		for {
			mt, msg, err := ws.ReadMessage()
			if err != nil {
				log.Println("Websocket read error:", err)
				return
			}

			if mt == websocket.TextMessage {
				// 嘗試解析 resize 指令 (JSON)
				var resizeMsg struct {
					Type string `json:"type"`
					Cols int    `json:"cols"`
					Rows int    `json:"rows"`
				}
				if err := json.Unmarshal(msg, &resizeMsg); err == nil && resizeMsg.Type == "resize" {
					go func() {
						err := k8sclient.ResizeTerminal(ctx, namespace, podName, containerName, resizeMsg.Cols, resizeMsg.Rows)
						if err != nil {
							log.Println("Resize terminal error:", err)
						}
					}()
					continue
				}
				// 當成輸入傳入 stdinChan
				stdinChan <- msg
			} else if mt == websocket.BinaryMessage {
				stdinChan <- msg
			}
		}
	}()

	// 用 k8sclient.ExecInteractive 執行指令並與 ws 互動 (你需要實作這方法)
	err = k8sclient.ExecInteractive(ctx, k8sclient.ExecOptions{
		Namespace:     namespace,
		PodName:       podName,
		ContainerName: containerName,
		Command:       []string{"/bin/bash"},
		Tty:           true,
		Stdin:         stdinChan,
		OnStdout: func(b []byte) {
			ws.WriteMessage(websocket.BinaryMessage, b)
		},
		OnStderr: func(b []byte) {
			ws.WriteMessage(websocket.BinaryMessage, b)
		},
		OnClose: func(status int) {
			log.Println("Terminal closed with status:", status)
			ws.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "Terminal closed"), 0)
			ws.Close()
		},
	})
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Println("ExecInteractive error:", err)
		ws.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(1011, "Internal error"), 0)
		ws.Close()
	}
}

func informerHandler(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Failed to upgrade websocket:", err)
		return
	}
	defer ws.Close()

	q := r.URL.Query()
	tokenStr := q.Get("token")

	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(config.JWTSecret), nil
	})
	if err != nil || !token.Valid {
		ws.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(4003, "Invalid token"), 0)
		return
	}
	namespace, ok := claims["username"].(string)
	if !ok || namespace == "" {
		ws.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(4002, "Missing namespace"), 0)
		return
	}

	log.Printf("[Informer] Client connected for namespace: %s", namespace)

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 要監聽的資源列表
	resources := []struct {
		Path string
		Name string
	}{
		{Path: fmt.Sprintf("/api/v1/namespaces/%s/pods", namespace), Name: "pod"},
		{Path: fmt.Sprintf("/apis/apps/v1/namespaces/%s/deployments", namespace), Name: "deployment"},
		{Path: fmt.Sprintf("/api/v1/namespaces/%s/services", namespace), Name: "service"},
	}

	for _, res := range resources {
		wg.Add(1)
		go func(resourcePath, resourceName string) {
			defer wg.Done()

			watcher, err := k8sclient.WatchResource(ctx, resourcePath,
				k8sclient.ResourceEventHandlers{
					OnAdded: func(obj interface{}) {
						sendFiltered(ws, resourceName, "ADDED", obj)
					},
					OnModified: func(obj interface{}) {
						sendFiltered(ws, resourceName, "MODIFIED", obj)
					},
					OnDeleted: func(obj interface{}) {
						sendFiltered(ws, resourceName, "DELETED", obj)
					},
					OnError: func(err error) {
						if errors.Is(err, context.Canceled) {
							log.Printf("%s watcher aborted normally", resourceName)
						} else {
							log.Printf("%s watcher error: %v", resourceName, err)
							ws.WriteJSON(map[string]string{"error": fmt.Sprintf("%s watcher error", resourceName)})
							ws.Close()
						}
					},
				})
			if err != nil {
				log.Printf("Failed to watch resource %s: %v", resourceName, err)
				ws.WriteJSON(map[string]string{"error": fmt.Sprintf("Failed to watch %s", resourceName)})
				return
			}

			// 監聽直到 ctx 被取消
			<-ctx.Done()
			watcher.Stop()
		}(res.Path, res.Name)
	}

	// 等待所有 watcher goroutine 結束
	wg.Wait()
}

// sendFiltered 把過濾後的資料送到 websocket
func sendFiltered(ws *websocket.Conn, resourceType, eventType string, obj interface{}) {
	filtered := filterResource(resourceType, obj)
	msg := map[string]interface{}{
		"type":     eventType,
		"resource": resourceType,
		"obj":      filtered,
	}
	ws.WriteJSON(msg)
}

func filterResource(resourceName string, obj interface{}) map[string]interface{} {
	// 請依據你的 obj 結構自行實作，以下為簡單示意
	// obj 應為 map[string]interface{} 或 Kubernetes 結構體
	m, ok := obj.(map[string]interface{})
	if !ok {
		return nil
	}

	metadata, _ := m["metadata"].(map[string]interface{})
	status, _ := m["status"].(map[string]interface{})
	spec, _ := m["spec"].(map[string]interface{})

	base := map[string]interface{}{
		"name":      metadata["name"],
		"namespace": metadata["namespace"],
	}

	switch resourceName {
	case "pod":
		return map[string]interface{}{
			"name":      base["name"],
			"namespace": base["namespace"],
			"phase":     status["phase"],
		}
	case "deployment":
		availableReplicas := status["availableReplicas"]
		replicas := spec["replicas"]
		conditions := []map[string]interface{}{}
		if conds, ok := status["conditions"].([]interface{}); ok {
			for _, c := range conds {
				if cond, ok := c.(map[string]interface{}); ok {
					conditions = append(conditions, map[string]interface{}{
						"type":   cond["type"],
						"status": cond["status"],
					})
				}
			}
		}
		return map[string]interface{}{
			"name":              base["name"],
			"namespace":         base["namespace"],
			"replicas":          replicas,
			"availableReplicas": availableReplicas,
			"conditions":        conditions,
		}
	case "service":
		externalIP := ""
		if lb, ok := status["loadBalancer"].(map[string]interface{}); ok {
			if ingress, ok := lb["ingress"].([]interface{}); ok && len(ingress) > 0 {
				ing0 := ingress[0].(map[string]interface{})
				if ip, ok := ing0["ip"].(string); ok && ip != "" {
					externalIP = ip
				} else if hostname, ok := ing0["hostname"].(string); ok && hostname != "" {
					externalIP = hostname
				}
			}
		}
		if externalIP == "" {
			if extIPs, ok := spec["externalIPs"].([]interface{}); ok && len(extIPs) > 0 {
				externalIP = extIPs[0].(string)
			}
		}
		ports := []map[string]interface{}{}
		if specPorts, ok := spec["ports"].([]interface{}); ok {
			for _, p := range specPorts {
				if portMap, ok := p.(map[string]interface{}); ok {
					port := portMap["port"]
					protocol := portMap["protocol"]
					nodePort := portMap["nodePort"]
					ports = append(ports, map[string]interface{}{
						"port":     port,
						"protocol": protocol,
						"nodePort": nodePort,
					})
				}
			}
		}
		return map[string]interface{}{
			"name":       base["name"],
			"namespace":  base["namespace"],
			"type":       spec["type"],
			"clusterIP":  spec["clusterIP"],
			"externalIP": externalIP,
			"ports":      ports,
		}
	default:
		return map[string]interface{}{
			"name":      base["name"],
			"namespace": base["namespace"],
			"phase":     status["phase"],
		}
	}
}
