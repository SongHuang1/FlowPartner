package server

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"testing"
)

func TestFindAvailablePort_Success(t *testing.T) {
	listener, port, err := FindAvailablePort(":1024", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if port < 1024 || port > 65535 {
		t.Fatalf("port out of range: %d", port)
	}
	if listener == nil {
		t.Fatal("listener should not be nil")
	}
	listener.Close()
}

// TestFindAvailablePort_HTTPAndGRPCDistinct 验证 HTTP 与 gRPC 端口探索结果不同（A15）
func TestFindAvailablePort_HTTPAndGRPCDistinct(t *testing.T) {
	httpListener, httpPort, err := FindAvailablePort(":8080", nil)
	if err != nil {
		t.Fatalf("HTTP port discovery failed: %v", err)
	}
	defer httpListener.Close()

	exclude := map[string]bool{fmt.Sprintf("127.0.0.1:%d", httpPort): true}
	grpcListener, grpcPort, err := FindAvailablePort(":50051", exclude)
	if err != nil {
		t.Fatalf("gRPC port discovery failed: %v", err)
	}
	defer grpcListener.Close()

	if grpcPort == httpPort {
		t.Fatalf("HTTP and gRPC ports must differ, both are %d", httpPort)
	}
}

func TestFindAvailablePort_ExcludeHTTPPort(t *testing.T) {
	// 占用一个端口
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer lis.Close()

	_, portStr, _ := net.SplitHostPort(lis.Addr().String())

	// 从被占用端口开始探索，同时排除该端口，应跳到下一个可用端口
	exclude := map[string]bool{"127.0.0.1:" + portStr: true}
	_, port2, err := FindAvailablePort(":"+portStr, exclude)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if port2 == 0 {
		t.Fatal("should find an available port")
	}
}

// TestFindAvailablePort_SkipsExcludedDuringIncrement 验证递增路径中遇到 exclude 端口时跳过
// 场景：起始端口被占用且下一个端口被排除，应落在再下一个端口
func TestFindAvailablePort_SkipsExcludedDuringIncrement(t *testing.T) {
	lis1, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer lis1.Close()
	_, portStr1, _ := net.SplitHostPort(lis1.Addr().String())
	p1, _ := strconv.Atoi(portStr1)

	// 排除 p1+1（即使它没有被占用）
	exclude := map[string]bool{fmt.Sprintf("127.0.0.1:%d", p1+1): true}

	listener, foundPort, err := FindAvailablePort(":"+portStr1, exclude)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer listener.Close()

	if foundPort == p1 {
		t.Fatalf("should skip occupied port %d", p1)
	}
	if foundPort == p1+1 {
		t.Fatalf("should skip excluded port %d", p1+1)
	}
}

func TestFindAvailablePort_InvalidPort(t *testing.T) {
	_, _, err := FindAvailablePort(":abc", nil)
	if err == nil {
		t.Fatal("expected error for invalid port string")
	}
}

func TestFindAvailablePort_OutOfRange(t *testing.T) {
	_, _, err := FindAvailablePort(":80", nil)
	if err == nil {
		t.Fatal("expected error for port < 1024")
	}
	if !errors.Is(err, ErrPortOutOfRange) {
		t.Fatalf("expected ErrPortOutOfRange, got: %v", err)
	}
}

// TestFindAvailablePort_AboveMax 验证起始端口超过 65535 时直接返回 ErrPortOutOfRange
func TestFindAvailablePort_AboveMax(t *testing.T) {
	_, _, err := FindAvailablePort(":65536", nil)
	if err == nil {
		t.Fatal("expected error for port > 65535")
	}
	if !errors.Is(err, ErrPortOutOfRange) {
		t.Fatalf("expected ErrPortOutOfRange, got: %v", err)
	}
}

func TestFindAvailablePort_PortOccupied(t *testing.T) {
	// 占用一个具体端口（绑定 127.0.0.1，与 FindAvailablePort 的回环绑定冲突，触发 EADDRINUSE）
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer lis.Close()

	_, portStr, _ := net.SplitHostPort(lis.Addr().String())

	// 从被占用端口开始探索，应跳过并找到下一个可用端口
	_, foundPort, err := FindAvailablePort(":"+portStr, nil)
	if err != nil {
		t.Fatalf("should find next available port, got error: %v", err)
	}
	if foundPort == 0 {
		t.Fatal("should find an available port")
	}
}

// TestFindAvailablePort_MaxPortOccupied 验证起始端口为 65535 且被占用时返回 ErrMaxAttemptsReached
func TestFindAvailablePort_MaxPortOccupied(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:65535")
	if err != nil {
		t.Skip("port 65535 unavailable on this machine")
	}
	defer lis.Close()

	_, _, err = FindAvailablePort(":65535", nil)
	if !errors.Is(err, ErrMaxAttemptsReached) {
		t.Fatalf("expected ErrMaxAttemptsReached, got: %v", err)
	}
}

// TestFindAvailablePort_NonPortInUseError 验证非端口占用错误直接返回（不重试）
func TestFindAvailablePort_NonPortInUseError(t *testing.T) {
	original := listenFn
	defer func() { listenFn = original }()
	listenFn = func(network, address string) (net.Listener, error) {
		return nil, errors.New("permission denied")
	}

	_, _, err := FindAvailablePort(":1024", nil)
	if err == nil {
		t.Fatal("expected error for non-port-in-use bind failure")
	}
	if !errors.Is(err, ErrNonPortInUseError) {
		t.Fatalf("expected ErrNonPortInUseError, got: %v", err)
	}
}

func TestParsePort(t *testing.T) {
	tests := []struct {
		input    string
		expected int
		wantErr  bool
	}{
		{":8080", 8080, false},
		{"8080", 8080, false},
		{":50051", 50051, false},
		{"invalid", 0, true},
		{"", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parsePort(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parsePort(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("parsePort(%q) = %d, want %d", tt.input, got, tt.expected)
			}
		})
	}
}
