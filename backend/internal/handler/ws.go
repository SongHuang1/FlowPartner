package handler

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/songhuang/flowpartner/backend/internal/bridge"
	"github.com/songhuang/flowpartner/backend/proto"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true }, // 本地开发允许跨域
}

type WebSocketHandler struct {
	manager *bridge.Manager
}

func NewWebSocketHandler(m *bridge.Manager) *WebSocketHandler {
	return &WebSocketHandler{manager: m}
}

func (h *WebSocketHandler) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket 升级失败:", err)
		return
	}
	defer conn.Close()

	log.Println("前端 WebSocket 已连接")

	// 监听前端发来的消息
	for {
		var msg struct {
			Action  string `json:"action"`
			Content string `json:"content"`
		}
		if err := conn.ReadJSON(&msg); err != nil {
			log.Println("WebSocket 读取失败或前端断开:", err)
			break
		}

		if msg.Action == "start_chat" {
			// 1. 生成高随机 Session ID
			b := make([]byte, 16)
			rand.Read(b)
			sessionId := fmt.Sprintf("sess_%d_%s", time.Now().UnixNano(), hex.EncodeToString(b))

			// 2. 注册 Session，将当前 WebSocket 连接绑定
			h.manager.RegisterSession(sessionId, conn)

			// 3. 将指令放入通道，gRPC 协程会把它发给 Python
			h.manager.CmdChan <- &proto.ServerCommand{
				SessionId:   sessionId,
				CommandType: "start_chat",
				Payload:     fmt.Sprintf(`{"user_message": "%s"}`, msg.Content),
			}
			log.Printf("[前端发起聊天] Session: %s | 内容: %s", sessionId, msg.Content)
		}
	}
}
