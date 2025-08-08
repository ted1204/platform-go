package handlers

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/linskybing/platform-go/k8sclient"
	"github.com/linskybing/platform-go/response"
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

	err = k8sclient.ExecToPodViaWebSocket(
		conn,
		k8sclient.Config,
		k8sclient.Clientset,
		c.Query("namespace"),
		c.Query("pod"),
		c.Query("container"),
		[]string{c.DefaultQuery("command", "/bin/sh")},
		c.DefaultQuery("tty", "true") == "true",
	)
	if err != nil {
		_ = conn.WriteMessage(websocket.TextMessage, []byte("exec error: "+err.Error()))
		conn.Close()
		return
	}
}

func WatchNamespaceHandler(c *gin.Context) {
	namespace := c.Param("namespace")

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: "websocket upgrade failed: " + err.Error()})
		return
	}

	writeChan := make(chan []byte, 100)

	go func() {
		defer conn.Close()
		for msg := range writeChan {
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				break
			}
		}
	}()

	go k8sclient.WatchNamespaceResources(writeChan, namespace)

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			close(writeChan)
			break
		}
	}
}
