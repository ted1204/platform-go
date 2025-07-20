package k8sclient

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/gorilla/websocket"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/util/homedir"
)

type WebSocketIO struct {
	Conn       *websocket.Conn
	readBuffer chan []byte
	closeCh    chan struct{}

	sizeMu     sync.Mutex
	size       remotecommand.TerminalSize
	notifySize chan struct{}
}

var (
	Config    *rest.Config
	Clientset *kubernetes.Clientset
)

// Init 載入 kubeconfig，初始化 Clientset
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
}


func NewWebSocketIO(conn *websocket.Conn, initialCols, initialRows uint16) *WebSocketIO {
	wsio := &WebSocketIO{
		Conn:       conn,
		readBuffer: make(chan []byte, 10),
		closeCh:    make(chan struct{}),

		size:       remotecommand.TerminalSize{Width: initialCols, Height: initialRows},
		notifySize: make(chan struct{}, 1),
	}

	go func() {
		defer close(wsio.readBuffer)
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}

			var resize struct {
				Type string `json:"type"`
				Cols uint16 `json:"cols"`
				Rows uint16 `json:"rows"`
			}

			if err := json.Unmarshal(msg, &resize); err == nil && resize.Type == "resize" {
				wsio.sizeMu.Lock()
				wsio.size.Width = resize.Cols
				wsio.size.Height = resize.Rows
				wsio.sizeMu.Unlock()

				// 非阻塞通知有 resize
				select {
				case wsio.notifySize <- struct{}{}:
				default:
				}
				continue
			}

			// 不是 resize 就當一般輸入傳給 shell
			wsio.readBuffer <- msg
		}
	}()

	return wsio
}

// Read 實作 io.Reader，提供給 executor stdin
func (w *WebSocketIO) Read(p []byte) (int, error) {
	select {
	case b, ok := <-w.readBuffer:
		if !ok {
			return 0, io.EOF
		}
		return copy(p, b), nil
	case <-w.closeCh:
		return 0, errors.New("connection closed")
	}
}

// Write 實作 io.Writer，提供給 executor stdout/stderr
func (w *WebSocketIO) Write(p []byte) (int, error) {
	err := w.Conn.WriteMessage(websocket.TextMessage, p)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

// Close 關閉連線
func (w *WebSocketIO) Close() error {
	close(w.closeCh)
	return w.Conn.Close()
}

// TerminalSizeQueue 介面實作，executor 呼叫這裡取得最新大小
func (w *WebSocketIO) Next() *remotecommand.TerminalSize {
	<-w.notifySize
	w.sizeMu.Lock()
	defer w.sizeMu.Unlock()
	return &w.size
}

// ExecToPodViaWebSocket 改用 WebSocketIO
func ExecToPodViaWebSocket(
	conn *websocket.Conn,
	config *rest.Config,
	clientset *kubernetes.Clientset,
	namespace, podName, container string,
	command []string,
	tty bool,
) error {
	wsIO := NewWebSocketIO(conn, 80, 24) // 初始 80x24

	req := clientset.CoreV1().RESTClient().
		Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: container,
			Command:   command,
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       tty,
		}, scheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return err
	}

	err = executor.Stream(remotecommand.StreamOptions{
		Stdin:             wsIO,
		Stdout:            wsIO,
		Stderr:            wsIO,
		Tty:               tty,
		TerminalSizeQueue: wsIO,
	})

	wsIO.Close()
	return err
}
