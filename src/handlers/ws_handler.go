package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/linskybing/platform-go/src/k8sclient"
	"github.com/linskybing/platform-go/src/response"
	"github.com/linskybing/platform-go/src/utils"
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
		[]string{c.DefaultQuery("command", "/bin/bash")},
		c.DefaultQuery("tty", "true") == "true",
	)
	if err != nil {
		errorMsg := k8sclient.TerminalMessage{
			Type: "stdout",
			Data: "\r\n\x1b[31m[Error] " + err.Error() + "\x1b[0m\r\n",
		}
		jsonMsg, _ := json.Marshal(errorMsg)
		_ = conn.WriteMessage(websocket.TextMessage, jsonMsg)
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	writeChan := make(chan []byte, 100)

	go func() {
		defer conn.Close()
		for {
			select {
			case msg := <-writeChan:
				if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
					cancel()
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	go k8sclient.WatchNamespaceResources(ctx, writeChan, namespace)

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			cancel()
			break
		}
	}
}

func WatchUserNamespaceHandler(c *gin.Context) {
	// 從 cookie 取得 username
	username, err := utils.GetUserNameFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.ErrorResponse{Error: "missing username cookie"})
		return
	}

	// 升級 websocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: "websocket upgrade failed: " + err.Error()})
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	writeChan := make(chan []byte, 100)

	// 監聽 websocket 寫入
	go func() {
		defer conn.Close()
		for {
			select {
			case msg := <-writeChan:
				if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
					cancel()
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// 取得符合 username 的 namespace 列表
	namespacesList, err := k8sclient.GetFilteredNamespaces(username)
	if err != nil {
		fmt.Printf("Failed to list user namespaces: %v\n", err)
		c.JSON(http.StatusInternalServerError, response.ErrorResponse{Error: "failed to list namespaces"})
		return
	}

	// 只取 namespace 名稱
	var namespaces []string
	for _, ns := range namespacesList {
		namespaces = append(namespaces, ns.Name)
	}

	// 為每個 namespace 啟動監控
	for _, ns := range namespaces {
		go k8sclient.WatchUserNamespaceResources(ctx, ns, writeChan)
	}

	// 監聽 client 關閉
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			cancel()
			break
		}
	}
}
