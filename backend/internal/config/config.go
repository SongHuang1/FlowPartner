package config

import "os"

// Config 从环境变量读取运行时配置
type Config struct {
	HTTPPort string // HTTP 监听端口，默认 ":8080"
}

// Load 从环境变量读取配置，缺失时使用默认值
func Load() *Config {
	return &Config{
		HTTPPort: getEnv("FP_HTTP_PORT", ":8090"),
	}
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}
