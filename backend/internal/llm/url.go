package llm

import (
	"fmt"
	"net/url"
	"path"
)

// NormalizeChatCompletionsURL 规范化 BaseURL 并拼接 /chat/completions 路径
func NormalizeChatCompletionsURL(baseURL string) (string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid base_url: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid base_url: missing scheme or host")
	}
	parsed.Path = path.Join(parsed.Path, "chat/completions")
	return parsed.String(), nil
}
