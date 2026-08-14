package server

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"syscall"
)

const (
	minPort     = 1024
	maxPort     = 65535
	maxAttempts = 100
)

// ErrPortOutOfRange 端口号超出合法范围
var ErrPortOutOfRange = errors.New("port number out of valid range (1024-65535)")

// ErrMaxAttemptsReached 达到最大尝试次数仍未找到可用端口
var ErrMaxAttemptsReached = errors.New("max port discovery attempts reached")

// ErrNonPortInUseError 非端口占用错误
var ErrNonPortInUseError = errors.New("non-port-inuse error during binding")

// listenFn 可替换以便测试非端口占用错误路径
var listenFn = net.Listen

// listenAddr 本地桌面应用只应监听回环接口，避免局域网内任意主机访问无鉴权 API
const listenAddr = "127.0.0.1"

// FindAvailablePort 从起始端口开始探索可用端口
// 返回实际绑定的 net.Listener（持有 listener 直到服务启动，避免 TOCTOU 风险）
// 始终绑定 127.0.0.1，Electron 与 Python Agent 均通过 localhost 连接，完全兼容
func FindAvailablePort(startPort string, exclude map[string]bool) (net.Listener, int, error) {
	port, err := parsePort(startPort)
	if err != nil {
		return nil, 0, err
	}

	if port < minPort {
		return nil, 0, fmt.Errorf("%w: %d", ErrPortOutOfRange, port)
	}
	if port > maxPort {
		return nil, 0, fmt.Errorf("%w: %d", ErrPortOutOfRange, port)
	}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		currentPort := port + attempt
		if currentPort > maxPort {
			return nil, 0, fmt.Errorf("%w: start port %d exceeded upper limit after %d increments", ErrMaxAttemptsReached, port, attempt)
		}

		addr := fmt.Sprintf("%s:%d", listenAddr, currentPort)
		if exclude[addr] {
			continue
		}

		listener, err := listenFn("tcp", addr)
		if err != nil {
			if isPortInUseError(err) {
				continue
			}
			return nil, 0, fmt.Errorf("%w: %v", ErrNonPortInUseError, err)
		}

		return listener, currentPort, nil
	}

	return nil, 0, fmt.Errorf("%w: start port %d", ErrMaxAttemptsReached, port)
}

// parsePort 解析端口字符串（如 ":8080"）为整数
func parsePort(s string) (int, error) {
	s = strings.TrimPrefix(s, ":")
	return strconv.Atoi(s)
}

// WSAEADDRINUSE 是 Windows 上的端口占用错误码。
// 注意：Windows 上 syscall.EADDRINUSE 映射为 CRT errno 值（100），
// 与实际 socket 错误 WSAEADDRINUSE (10048) 不一致，必须显式检查。
const WSAEADDRINUSE = syscall.Errno(10048)

// isPortInUseError 判断错误是否为端口占用
func isPortInUseError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, syscall.EADDRINUSE) {
		return true
	}
	return errors.Is(err, WSAEADDRINUSE)
}
