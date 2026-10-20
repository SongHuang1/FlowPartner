package llm

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// SSEEvent 表示一个 SSE 事件
type SSEEvent struct {
	Data string
	Done bool
}

// SSEScanner 从 io.Reader 中逐行解析 SSE 流
type SSEScanner struct {
	scanner *bufio.Scanner
}

// NewSSEScanner 创建 SSE 解析器
func NewSSEScanner(r io.Reader) *SSEScanner {
	// 单条事件上限 1MB：够容纳超长 tool_call 参数，又能让异常巨大的单行尽早
	// 失败，而不是让 bufio 无限扩张内存。
	const maxScanTokenSize = 1024 * 1024

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), maxScanTokenSize)
	return &SSEScanner{scanner: scanner}
}

// Scan 读取下一个 SSE 事件
// 返回: event, done, error
// - done=true 表示流结束（[DONE] 或连接关闭）
func (s *SSEScanner) Scan() (SSEEvent, bool, error) {
	for s.scanner.Scan() {
		line := s.scanner.Text()

		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}

		line = strings.TrimSpace(line)
		data := strings.TrimPrefix(line, "data:")
		data = strings.TrimSpace(data)

		if strings.TrimSpace(data) == "[DONE]" {
			return SSEEvent{Done: true}, true, nil
		}

		return SSEEvent{Data: data}, false, nil
	}

	if err := s.scanner.Err(); err != nil {
		return SSEEvent{}, true, fmt.Errorf("sse scan error: %w", err)
	}

	return SSEEvent{Done: true}, true, nil
}
