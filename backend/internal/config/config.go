package config

import (
	"os"
	"path/filepath"
	"strconv"
)

// Config 从环境变量读取运行时配置
type Config struct {
	HTTPPort    string // HTTP 监听端口，默认 ":8080"
	FrontendDir string // 前端构建产物目录，默认 "../frontend/dist"
	DevMode     bool   // 开发模式，默认 false
}

// Load 从环境变量读取配置，缺失时使用默认值
func Load() *Config {
	cfg := &Config{
		HTTPPort: getEnv("FP_HTTP_PORT", ":8080"),
		DevMode:  getEnvAsBool("FP_DEV_MODE", false),
	}

	// FrontendDir: 优先使用环境变量，否则基于 CWD 计算
	if dir := os.Getenv("FP_FRONTEND_DIR"); dir != "" {
		cfg.FrontendDir = resolvePath(dir)
	} else {
		cfg.FrontendDir = resolvePath("../frontend/dist")
	}

	return cfg
}

// resolvePath 将相对路径解析为绝对路径（基于 CWD），绝对路径直接返回
func resolvePath(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	wd, err := os.Getwd()
	if err != nil {
		return p
	}
	abs, err := filepath.Abs(filepath.Join(wd, p))
	if err != nil {
		return p
	}
	return abs
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}

func getEnvAsBool(key string, fallback bool) bool {
	if val, ok := os.LookupEnv(key); ok {
		b, err := strconv.ParseBool(val)
		if err == nil {
			return b
		}
	}
	return fallback
}
