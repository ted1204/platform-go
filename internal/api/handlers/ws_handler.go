package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/linskybing/platform-go/internal/application"
	"github.com/linskybing/platform-go/pkg/k8s"
	"github.com/linskybing/platform-go/pkg/response"
	"k8s.io/client-go/kubernetes"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Batching: Maximum number of messages to buffer before forcing a send
	batchSize = 50

	// Batching: Maximum time to wait before sending buffered messages
	flushFrequency = 100 * time.Millisecond
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		log.Println("WebSocket Origin:", r.Header.Get("Origin"))
		log.Println("WebSocket Host:", r.Host)
		return true
	},
}

func ExecWebSocketHandler(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: "websocket upgrade failed: " + err.Error()})
		return
	}

	cs, ok := k8s.Clientset.(*kubernetes.Clientset)
	if !ok || cs == nil {
		c.JSON(http.StatusServiceUnavailable, response.ErrorResponse{Error: "k8s client not available"})
		return
	}

	err = k8s.ExecToPodViaWebSocket(
		conn,
		k8s.Config,
		cs,
		c.Query("namespace"),
		c.Query("pod"),
		c.Query("container"),
		[]string{c.DefaultQuery("command", "/bin/bash")},
		c.DefaultQuery("tty", "true") == "true",
	)
	if err != nil {
		errorMsg := k8s.TerminalMessage{
			Type: "stdout",
			Data: "\r\n\x1b[31m[Error] " + err.Error() + "\x1b[0m\r\n",
		}
		jsonMsg, _ := json.Marshal(errorMsg)
		_ = conn.WriteMessage(websocket.TextMessage, jsonMsg)
		_ = conn.Close()
		return
	}
}

// WatchNamespaceHandler monitors resources for a specific namespace
// Features: Heartbeat, Message Batching, Context Cancellation
func WatchNamespaceHandler(c *gin.Context) {
	// 1. Get Target Namespace
	namespace := c.Param("namespace")
	if namespace == "" {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "namespace parameter is required"})
		return
	}

	// (Optional) Authentication check - keep if you need to ensure user is logged in
	// username, err := utils.GetUserNameFromContext(c)
	// if err != nil { ... }

	// 2. WebSocket Upgrade
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: "websocket upgrade failed: " + err.Error()})
		return
	}

	// 3. Context & Cleanup Setup
	// Create a context that will be canceled when the connection closes
	ctx, cancel := context.WithCancel(context.Background())
	// Ensure cancel is called to stop all K8s watchers when this handler exits
	defer cancel()

	// 4. Configure Heartbeat (Reader Side)
	conn.SetReadLimit(512 * 1024)
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	// Channel to receive raw JSON bytes from K8s watchers
	writeChan := make(chan []byte, 200)

	// 5. Writer Goroutine (Handles Batching & Ping)
	go func() {
		defer func() { _ = conn.Close() }()

		// Ticker for Heartbeat Pings
		pingTicker := time.NewTicker(pingPeriod)
		defer pingTicker.Stop()

		// Ticker for Batch Flushing
		flushTicker := time.NewTicker(flushFrequency)
		defer flushTicker.Stop()

		// Buffer to store messages temporarily
		var buffer []json.RawMessage

		// Helper function to send all buffered messages as a JSON Array
		flush := func() error {
			if len(buffer) == 0 {
				return nil
			}

			// Marshal the slice of RawMessages into a JSON Array: [{}, {}, {}]
			batchData, err := json.Marshal(buffer)
			if err != nil {
				return err
			}

			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.TextMessage, batchData); err != nil {
				return err
			}

			// Clear buffer and keep capacity
			buffer = buffer[:0]
			return nil
		}

		for {
			select {
			case msg, ok := <-writeChan:
				if !ok {
					_ = conn.WriteMessage(websocket.CloseMessage, []byte{})
					return
				}

				// Append message to buffer
				buffer = append(buffer, json.RawMessage(msg))

				// If buffer is full, flush immediately
				if len(buffer) >= batchSize {
					if err := flush(); err != nil {
						cancel()
						return
					}
				}

			case <-flushTicker.C:
				// Time-based flush
				if err := flush(); err != nil {
					cancel()
					return
				}

			case <-pingTicker.C:
				// Send Ping (Heartbeat)
				// Flush data first to ensure latest updates go out before Ping
				if err := flush(); err != nil {
					cancel()
					return
				}

				_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					cancel()
					return
				}

			case <-ctx.Done():
				return
			}
		}
	}()

	// 6. Start Watcher
	// 這裡直接監聽單一 Namespace，不需要 Loop 也不需要 Sleep (Staggering)
	// 因為只發起少量的 List/Watch 請求，不會觸發 K8s Client 限流
	go k8s.WatchNamespaceResources(ctx, writeChan, namespace)

	// 7. Reader Loop (Blocking)
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			// Exit loop -> defer cancel() triggers -> All watchers stop
			break
		}
	}
}

// WatchImagePullHandler monitors a specific image pull job status via WebSocket
func WatchImagePullHandler(c *gin.Context, imageService interface{}) {
	jobID := c.Param("job_id")
	if jobID == "" {
		c.JSON(http.StatusBadRequest, response.ErrorResponse{Error: "job_id parameter is required"})
		return
	}

	// WebSocket Upgrade
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: "websocket upgrade failed: " + err.Error()})
		return
	}
	defer conn.Close()

	// Context for this connection
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Get the image service from context
	service, ok := imageService.(*application.ImageService)
	if !ok {
		_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"service not available"}`))
		return
	}

	// Subscribe to job updates
	statusChan := service.SubscribeToPullJob(jobID)
	if statusChan == nil {
		_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"job not found"}`))
		return
	}

	// Set up ping ticker for heartbeat
	pingTicker := time.NewTicker(pingPeriod)
	defer pingTicker.Stop()

	// Writer loop
	for {
		select {
		case status, ok := <-statusChan:
			if !ok {
				// Channel closed, job monitoring finished
				_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
				_ = conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// Send status update to client
			data, err := json.Marshal(gin.H{
				"job_id":    status.JobID,
				"image":     status.ImageName + ":" + status.ImageTag,
				"status":    status.Status,
				"progress":  status.Progress,
				"message":   status.Message,
				"timestamp": status.UpdatedAt,
			})
			if err != nil {
				continue
			}

			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				cancel()
				return
			}

		case <-pingTicker.C:
			// Send heartbeat
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				cancel()
				return
			}

		case <-ctx.Done():
			return
		}
	}
}

// WatchMultiplePullJobsHandler monitors multiple pull job statuses
func WatchMultiplePullJobsHandler(c *gin.Context, imageService interface{}) {
	// WebSocket Upgrade
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: "websocket upgrade failed: " + err.Error()})
		return
	}
	defer conn.Close()

	// Context for this connection
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Get the image service
	service, ok := imageService.(*application.ImageService)
	if !ok {
		_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"service not available"}`))
		return
	}

	// Map of jobID -> channel subscription
	subscriptions := make(map[string]<-chan *application.PullJobStatus)

	// Set up ping ticker
	pingTicker := time.NewTicker(pingPeriod)
	defer pingTicker.Stop()

	// Channel for receiving job IDs to monitor from client
	jobIDsChan := make(chan string, 10)

	// Reader goroutine - parse incoming subscribe/unsubscribe requests
	go func() {
		for {
			var msg map[string]interface{}
			if err := conn.ReadJSON(&msg); err != nil {
				return
			}

			if action, ok := msg["action"].(string); ok {
				if action == "subscribe" {
					if jobID, ok := msg["job_id"].(string); ok {
						jobIDsChan <- jobID
					}
				}
			}
		}
	}()

	// Writer loop
	for {
		select {
		case jobID := <-jobIDsChan:
			// Subscribe to new job
			if _, exists := subscriptions[jobID]; !exists {
				statusChan := service.SubscribeToPullJob(jobID)
				if statusChan != nil {
					subscriptions[jobID] = statusChan
				}
			}

		case <-pingTicker.C:
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				cancel()
				return
			}

		case <-ctx.Done():
			return
		}

		// Check all subscriptions for updates
		for jobID, statusChan := range subscriptions {
			select {
			case status, ok := <-statusChan:
				if !ok {
					// Channel closed
					delete(subscriptions, jobID)
					continue
				}

				// Send status update
				data, err := json.Marshal(gin.H{
					"job_id":    status.JobID,
					"image":     status.ImageName + ":" + status.ImageTag,
					"status":    status.Status,
					"progress":  status.Progress,
					"message":   status.Message,
					"timestamp": status.UpdatedAt,
				})
				if err != nil {
					continue
				}

				_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
				if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
					cancel()
					return
				}

			default:
				// No update on this channel
			}
		}
	}
}
