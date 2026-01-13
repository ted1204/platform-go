package handlers

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"strconv" // Ensure this is imported for parameter parsing
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/linskybing/platform-go/pkg/k8s"
	"github.com/linskybing/platform-go/pkg/response"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// PodLogHandler implements functionality similar to "kubectl logs -f"
// Supported Query Parameters:
// - namespace (required): The namespace of the pod
// - pod (required): The name of the pod
// - container (optional): Specific container name (defaults to the first one)
// - tail_lines (optional): Number of lines to show from the end of the logs (default: 100)
// - follow (optional): Whether to stream logs continuously (default: true)
func PodLogHandler(c *gin.Context) {
	namespace := c.Query("namespace")
	podName := c.Query("pod")
	container := c.Query("container")

	if namespace == "" || podName == "" {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "namespace and pod are required"})
		return
	}

	// Upgrade the HTTP connection to a WebSocket connection
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: "websocket upgrade failed: " + err.Error()})
		return
	}
	// Ensure the connection is closed when the function exits and log any error
	defer func() {
		if err := conn.Close(); err != nil {
			fmt.Printf("websocket close error: %v\n", err)
		}
	}()

	// Retrieve the K8s Clientset
	cs, ok := k8s.Clientset.(*kubernetes.Clientset)
	if !ok || cs == nil {
		_ = conn.WriteMessage(websocket.TextMessage, []byte("k8s client not available"))
		return
	}

	// Parse optional parameters
	tailLines := int64(100) // Default to 100 lines
	if val := c.Query("tail_lines"); val != "" {
		if n, err := strconv.ParseInt(val, 10, 64); err == nil {
			tailLines = n
		}
	}

	follow := c.DefaultQuery("follow", "true") == "true"

	// Configure Pod Log Options
	logOptions := &corev1.PodLogOptions{
		Container:  container,
		Follow:     follow, // Stream continuously
		TailLines:  &tailLines,
		Timestamps: false, // Set to true if timestamps are needed
	}

	// Create the K8s Log Request
	req := cs.CoreV1().Pods(namespace).GetLogs(podName, logOptions)
	stream, err := req.Stream(c.Request.Context())
	if err != nil {
		_ = conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("error opening stream: %v", err)))
		return
	}
	// Ensure the stream is closed when the function exits and log any error
	defer func() {
		if err := stream.Close(); err != nil {
			fmt.Printf("stream close error: %v\n", err)
		}
	}()

	// WebSocket Heartbeat Configuration
	// CRITICAL: Prevents connection drops from Load Balancers during idle periods.
	const (
		pongWait   = 60 * time.Second
		pingPeriod = (pongWait * 9) / 10
		writeWait  = 10 * time.Second
	)

	conn.SetReadLimit(512)
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	// Channel to signal the writer goroutine to stop
	done := make(chan struct{})

	// 1. Reader Goroutine: Handles Control Frames (Ping/Pong/Close)
	// If we don't read from the socket, Pong responses won't be processed,
	// and the connection will time out or close unexpectedly.
	go func() {
		defer close(done)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				// Client disconnected or error occurred
				return
			}
		}
	}()

	// 2. Ticker Goroutine: Sends periodic Pings
	go func() {
		ticker := time.NewTicker(pingPeriod)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			case <-done:
				return
			}
		}
	}()

	// 3. Main Loop: Reads from K8s Stream and writes to WebSocket
	reader := bufio.NewReader(stream)
	for {
		// Read line by line using ReadBytes
		// Note: For extremely long lines or high throughput, consider using Read(buffer)
		line, err := reader.ReadBytes('\n')

		if len(line) > 0 {
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			// Write raw bytes directly to avoid string conversion overhead
			if wErr := conn.WriteMessage(websocket.TextMessage, line); wErr != nil {
				// WebSocket write failed (Client likely disconnected)
				break
			}
		}

		if err != nil {
			if err != io.EOF {
				// Optionally send the error to the frontend
				_ = conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("stream error: %v", err)))
			}
			break
		}
	}
}
