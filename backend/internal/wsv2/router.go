package wsv2

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/songhuang/flowpartner/backend/internal/thread"
)

// ConnState represents the connection lifecycle state.
type ConnState int

const (
	StateConnected ConnState = iota
	StateInitialized
	StateActive
)

// HandlerFunc processes a request and returns a result or error.
type HandlerFunc func(conn *Conn, params json.RawMessage) (interface{}, *ErrorPayload)

// Router manages the WS connection lifecycle and method dispatch.
type Router struct {
	mu        sync.Mutex
	conn      *websocket.Conn
	state     ConnState
	methods   map[string]HandlerFunc
	writeMu   sync.Mutex
	globalMu  sync.Mutex
	threadMgr *thread.Manager
	connID    string
	attached  map[string]struct{}

	// inboundCh is the bounded queue for inbound requests (backpressure).
	inboundCh chan *Envelope
	// pendingCount tracks the number of requests currently being processed.
	pendingCount int32
}

// NewRouter creates a new Router for the given WebSocket connection.
func NewRouter(conn *websocket.Conn) *Router {
	r := &Router{
		conn:      conn,
		state:     StateConnected,
		methods:   make(map[string]HandlerFunc),
		connID:    generateConnID(),
		attached:  make(map[string]struct{}),
		inboundCh: make(chan *Envelope, InboundQueueCapacity),
	}
	r.registerDefaultMethods()
	return r
}

// SetThreadManager sets the manager for thread attachment.
func (r *Router) SetThreadManager(m *thread.Manager) {
	r.threadMgr = m
}

// RegisterMethod registers a handler for a method name.
func (r *Router) RegisterMethod(method string, fn HandlerFunc) {
	r.methods[method] = fn
}

// State returns the current connection state.
func (r *Router) State() ConnState {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.state
}

// Methods returns the registered method handlers (for dispatch).
func (r *Router) Methods() map[string]HandlerFunc {
	return r.methods
}

// registerDefaultMethods registers the handshake methods.
func (r *Router) registerDefaultMethods() {
	r.methods["initialize"] = r.handleInitialize
	r.methods["initialized"] = r.handleInitializedNotification
}

// handleInitialize processes the initialize request.
func (r *Router) handleInitialize(conn *Conn, params json.RawMessage) (interface{}, *ErrorPayload) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.state != StateConnected {
		return nil, &ErrorPayload{
			Code:    ErrInvalidParams,
			Message: "initialize 只能调用一次",
		}
	}

	var req struct {
		ClientInfo struct {
			Name    string `json:"name"`
			Title   string `json:"title"`
			Version string `json:"version"`
		} `json:"clientInfo"`
	}
	if err := json.Unmarshal(params, &req); err != nil {
		return nil, &ErrorPayload{
			Code:    ErrInvalidParams,
			Message: "initialize 参数格式错误",
		}
	}

	r.state = StateInitialized

	return map[string]string{
		"userAgent":       "flowpartner-backend",
		"fpHome":          "",
		"platformOs":      getPlatformOS(),
		"protocolVersion": ProtocolVersion,
	}, nil
}

// handleInitializedNotification processes the initialized notification.
func (r *Router) handleInitializedNotification(conn *Conn, params json.RawMessage) (interface{}, *ErrorPayload) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.state != StateInitialized {
		return nil, &ErrorPayload{
			Code:    ErrNotInitialized,
			Message: "initialized 通知必须在 initialize 应答之后发送",
		}
	}

	r.state = StateActive
	return nil, nil
}

// Serve starts reading messages from the connection and dispatching them.
// It uses a bounded inbound queue for backpressure: when the queue is full,
// new requests receive ErrOverload (-32001).
func (r *Router) Serve() {
	// Start the dispatch goroutine that processes requests from the inbound queue.
	dispatchDone := make(chan struct{})
	go r.dispatchLoop(dispatchDone)
	defer close(dispatchDone)

	for {
		var env Envelope
		if err := r.conn.ReadJSON(&env); err != nil {
			if websocket.IsUnexpectedCloseError(err) || websocket.IsCloseError(err) {
				log.Printf("WebSocket closed: %v", err)
			} else {
				log.Printf("WebSocket read error: %v", err)
			}
			return
		}

		r.attachThread(env)

		switch env.Kind() {
		case KindRequest:
			// Backpressure: reject if inbound queue is full.
			select {
			case r.inboundCh <- &env:
				// queued successfully
			default:
				if env.Id != nil {
					r.sendError(*env.Id, ErrOverload, "服务器繁忙，请稍后重试", nil)
				}
			}
		case KindNotification:
			r.handleNotification(env)
		default:
			if env.Id != nil {
				r.sendError(*env.Id, ErrMethodNotFound, "客户端不能发送 response/error 消息", nil)
			}
		}
	}
}

// dispatchLoop processes requests from the inbound queue sequentially.
func (r *Router) dispatchLoop(done <-chan struct{}) {
	for {
		select {
		case <-done:
			return
		case env := <-r.inboundCh:
			r.handleRequest(*env)
		}
	}
}

func (r *Router) attachThread(env Envelope) {
	if r.threadMgr == nil {
		return
	}
	threadID := extractThreadIDFromParams(env.Params)
	if threadID == "" {
		return
	}
	if _, ok := r.attached[threadID]; ok {
		return
	}
	if t, ok := r.threadMgr.GetThread(threadID); ok {
		t.AttachConn(r.connID, r)
		r.attached[threadID] = struct{}{}
	}
}

func extractThreadIDFromParams(params json.RawMessage) string {
	var p struct {
		ThreadID string `json:"threadId"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return ""
	}
	return p.ThreadID
}

func generateConnID() string {
	return fmt.Sprintf("conn_%d_%d", rand.Int63(), rand.Int31())
}

var _ thread.Connection = (*Router)(nil)

// handleRequest processes a request envelope.
func (r *Router) handleRequest(env Envelope) {
	r.mu.Lock()
	state := r.state
	r.mu.Unlock()

	if env.Method == "" {
		r.sendError(*env.Id, ErrMethodNotFound, "请求缺少 method 字段", nil)
		return
	}

	if state == StateConnected && env.Method != "initialize" {
		r.sendError(*env.Id, ErrNotInitialized, "连接尚未完成握手，请先发送 initialize", nil)
		return
	}
	if state == StateInitialized && env.Method != "initialized" && env.Method != "initialize" {
		r.sendError(*env.Id, ErrNotInitialized, "握手未完成，请发送 initialized 通知", nil)
		return
	}

	fn, ok := r.methods[env.Method]
	if !ok {
		r.sendError(*env.Id, ErrMethodNotFound, fmt.Sprintf("未知方法: %s", env.Method), nil)
		return
	}

	result, errPayload := fn(r.connToConn(env), env.Params)
	if errPayload != nil {
		r.sendError(*env.Id, errPayload.Code, errPayload.Message, errPayload.Data)
		return
	}

	resp, err := NewResponse(*env.Id, result)
	if err != nil {
		r.sendError(*env.Id, ErrInternalError, "内部错误：序列化响应失败", nil)
		return
	}
	r.send(resp)
}

// handleNotification processes a notification envelope.
func (r *Router) handleNotification(env Envelope) {
	r.mu.Lock()
	state := r.state
	r.mu.Unlock()

	if env.Method == "" {
		return
	}

	if state == StateConnected {
		return // drop all notifications before initialize
	}

	fn, ok := r.methods[env.Method]
	if !ok {
		return // silently drop unknown notifications
	}

	_, _ = fn(r.connToConn(env), env.Params)
}

// send writes an envelope to the WebSocket connection.
func (r *Router) send(env *Envelope) {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	if err := r.conn.WriteJSON(env); err != nil {
		log.Printf("WebSocket write error: %v", err)
	}
}

// sendError sends an error response.
func (r *Router) sendError(id RequestId, code int, message string, data interface{}) {
	env := NewErrorResponse(id, code, message, data)
	r.send(env)
}

// sendNotification sends a notification to the client.
func (r *Router) SendNotification(method string, params interface{}) error {
	env, err := NewNotification(method, params)
	if err != nil {
		return err
	}
	r.send(env)
	return nil
}

// connToConn wraps the router as a Conn for handler dispatch.
func (r *Router) connToConn(env Envelope) *Conn {
	return &Conn{router: r, env: &env}
}

// State returns the current connection state.

// Conn represents a connection context passed to handlers.
type Conn struct {
	router *Router
	env    *Envelope
}

// State returns the current connection state.
func (c *Conn) State() ConnState {
	return c.router.State()
}

// SendNotification sends a notification on this connection.
func (c *Conn) SendNotification(method string, params interface{}) error {
	return c.router.SendNotification(method, params)
}

// GlobalMutex returns the global mutex for initialize/settings serialization.
func (c *Conn) GlobalMutex() *sync.Mutex {
	return &c.router.globalMu
}

// Upgrade upgrades an HTTP connection to WebSocket and returns a Router.
// Rejects connections with an Origin header to prevent DNS rebinding attacks.
func Upgrade(w http.ResponseWriter, r *http.Request) (*Router, error) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			// Reject any connection with an Origin header (FR-P7 security requirement)
			return r.Header.Get("Origin") == ""
		},
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, err
	}
	conn.SetReadLimit(MaxPayloadSize)
	return NewRouter(conn), nil
}

func getPlatformOS() string {
	// Simplified; in production use runtime.GOOS
	return "windows"
}
