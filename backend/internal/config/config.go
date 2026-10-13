package config

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
)

type Config struct {
	HTTPPort    string
	FrontendDir string
	DevMode     bool
}

func Load() *Config {
	cfg := &Config{
		HTTPPort: getEnv("FP_HTTP_PORT", "8080"),
		DevMode:  getEnvAsBool("FP_DEV_MODE", false),
	}

	if dir := os.Getenv("FP_FRONTEND_DIR"); dir != "" {
		cfg.FrontendDir = resolvePath(dir)
	} else {
		cfg.FrontendDir = resolvePath("../frontend/dist")
	}

	return cfg
}

func resolvePath(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	wd, err := os.Getwd()
	if err != nil {
		log.Printf("[config] os.Getwd() failed: %v, returning original path %q", err, p)
		return p
	}
	abs, err := filepath.Abs(filepath.Join(wd, p))
	if err != nil {
		log.Printf("[config] filepath.Abs(%q) failed: %v, returning original path %q", p, err, p)
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
