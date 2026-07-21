package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/songhuang/flowpartner/backend/internal/config"
	"github.com/songhuang/flowpartner/backend/internal/response"
)

func main() {
	cfg := config.Load()

	mux := setupRoutes(cfg)

	server := &http.Server{
		Addr:    cfg.HTTPPort,
		Handler: mux,
	}

	serverErr := make(chan error, 1)

	listener, err := net.Listen("tcp", cfg.HTTPPort)
	if err != nil {
		log.Fatalf("Failed to bind %s: %v", cfg.HTTPPort, err)
	}

	log.Printf("HTTP server starting on %s", cfg.HTTPPort)

	fmt.Fprintln(os.Stderr, "__FP_BACKEND_READY__")

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			serverErr <- err
			return
		}
		close(serverErr)
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		log.Fatalf("HTTP server error: %v", err)
	case sig := <-quit:
		log.Printf("Received signal %v, shutting down server...", sig)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	log.Println("Server exited")
}

// TODO(step4+): 加入 stdin EOF 检测，实现优雅退出（读取 stdin，收到 EOF 时调用 server.Shutdown）

// setupRoutes 配置所有 HTTP 路由，返回 handler
func setupRoutes(cfg *config.Config) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if cfg.DevMode {
			http.NotFound(w, r)
			return
		}
		serveSPA(w, r, cfg.FrontendDir)
	})

	mux.HandleFunc("/api/", notImplementedHandler)

	return mux
}

func serveSPA(w http.ResponseWriter, r *http.Request, frontendDir string) {
	cleanPath := path.Clean(r.URL.Path)

	if strings.HasPrefix(cleanPath, "/api/") {
		notImplementedHandler(w, r)
		return
	}

	if cleanPath == "/health" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
		return
	}

	if strings.HasPrefix(cleanPath, "/assets/") {
		http.FileServer(http.Dir(frontendDir)).ServeHTTP(w, r)
		return
	}

	fullPath := filepath.Join(frontendDir, cleanPath)
	if _, err := os.Stat(fullPath); err == nil {
		http.ServeFile(w, r, fullPath)
		return
	}

	indexPath := filepath.Join(frontendDir, "index.html")
	http.ServeFile(w, r, indexPath)
}

func notImplementedHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	resp := response.Error(response.CodeNotImplemented, "API not implemented yet")
	resp.RequestID = uuid.NewString()
	json.NewEncoder(w).Encode(resp)
}
