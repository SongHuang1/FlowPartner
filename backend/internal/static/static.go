package static

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Handler 提供前端静态文件服务，支持 SPA fallback
type Handler struct {
	staticDir string
	indexPath string
}

// NewHandler 创建静态文件处理器
func NewHandler(staticDir string) *Handler {
	return &Handler{
		staticDir: staticDir,
		indexPath: filepath.Join(staticDir, "index.html"),
	}
}

// Handle 注册路由到 mux，包括静态文件和 SPA fallback
func (h *Handler) Handle(mux *http.ServeMux) {
	if h.staticDir == "" {
		return
	}

	if _, err := os.Stat(h.staticDir); os.IsNotExist(err) {
		log.Printf("Static directory not found: %s, skipping static file serving", h.staticDir)
		return
	}

	fileServer := http.FileServer(http.Dir(h.staticDir))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// API 和 WS 路由不处理
		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/ws" {
			http.NotFound(w, r)
			return
		}

		// 检查文件是否存在
		requestedPath := filepath.Join(h.staticDir, r.URL.Path)
		if info, err := os.Stat(requestedPath); err == nil && !info.IsDir() {
			fileServer.ServeHTTP(w, r)
			return
		}

		// SPA fallback: 所有未匹配路由返回 index.html
		http.ServeFile(w, r, h.indexPath)
	})
}
