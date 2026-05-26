package netutil

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
	"strings"
)

const (
	ListenPortMin = 1
	ListenPortMax = 65535
)

// FindFreeTCPPort 返回当前可 bind 的随机 TCP 端口（用于安装时管理端口）。
func FindFreeTCPPort() (int, error) {
	for i := 0; i < 80; i++ {
		port := randomListenPort()
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err == nil {
			_ = ln.Close()
			return port, nil
		}
	}
	return 0, fmt.Errorf("no free TCP port found")
}

func randomListenPort() int {
	var b [2]byte
	_, _ = rand.Read(b[:])
	span := ListenPortMax - 1024 + 1
	return 1024 + int(binary.BigEndian.Uint16(b[:]))%span
}

// ValidateListenPort 校验监听端口字符串。
func ValidateListenPort(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("port required")
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < ListenPortMin || n > ListenPortMax {
		return "", fmt.Errorf("port must be %d–%d", ListenPortMin, ListenPortMax)
	}
	return strconv.Itoa(n), nil
}
