package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const defaultPath = "/var/lib/qosnat2/audit.log"

var (
	mu   sync.Mutex
	path = defaultPath
)

// SetPath 测试或部署时覆盖日志路径
func SetPath(p string) {
	if p != "" {
		path = p
	}
}

// Entry 单条审计记录
type Entry struct {
	Time   string `json:"time"`
	User   string `json:"user"`
	Action string `json:"action"`
	Detail string `json:"detail,omitempty"`
}

// Log 追加 JSON 行审计日志
func Log(user, action, detail string) {
	if user == "" {
		user = "system"
	}
	e := Entry{
		Time:   time.Now().UTC().Format(time.RFC3339),
		User:   user,
		Action: action,
		Detail: detail,
	}
	b, err := json.Marshal(e)
	if err != nil {
		return
	}
	line := append(b, '\n')
	mu.Lock()
	defer mu.Unlock()
	_ = os.MkdirAll(filepath.Dir(path), 0750)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0640)
	if err != nil {
		return
	}
	_, _ = f.Write(line)
	_ = f.Close()
}

// Tail 读取最近 n 行（从新到旧）
func Tail(n int) ([]Entry, error) {
	if n <= 0 {
		n = 50
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Entry{}, nil
		}
		return nil, err
	}
	var lines []string
	start := 0
	for i, c := range b {
		if c == '\n' {
			if i > start {
				lines = append(lines, string(b[start:i]))
			}
			start = i + 1
		}
	}
	if start < len(b) {
		lines = append(lines, string(b[start:]))
	}
	out := make([]Entry, 0, n)
	for i := len(lines) - 1; i >= 0 && len(out) < n; i-- {
		if lines[i] == "" {
			continue
		}
		var e Entry
		if json.Unmarshal([]byte(lines[i]), &e) == nil {
			out = append(out, e)
		}
	}
	return out, nil
}

// Path 当前日志路径
func Path() string { return path }

// FormatDetail 格式化详情
func FormatDetail(args ...any) string {
	return fmt.Sprint(args...)
}
