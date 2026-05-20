package capture

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

const defaultDir = "/var/lib/qosnat2/captures"

// Session 抓包任务
type Session struct {
	ID        string    `json:"id"`
	Device    string    `json:"device"`
	Filter    string    `json:"filter,omitempty"`
	PID       int       `json:"pid,omitempty"`
	File      string    `json:"file"`
	Size      int64     `json:"size_bytes"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
	Running   bool      `json:"running"`
}

// Manager tcpdump 会话
type Manager struct {
	mu   sync.Mutex
	dir  string
	jobs map[string]*Session
}

func New(dir string) *Manager {
	if dir == "" {
		dir = defaultDir
	}
	_ = os.MkdirAll(dir, 0750)
	return &Manager{dir: dir, jobs: map[string]*Session{}}
}

func (m *Manager) Dir() string { return m.dir }

// Start 启动 tcpdump（duration 秒后自动结束，0=默认60）
func (m *Manager) Start(dev, filter string, durationSec int) (*Session, error) {
	if _, err := exec.LookPath("tcpdump"); err != nil {
		return nil, fmt.Errorf("tcpdump not installed")
	}
	if durationSec <= 0 {
		durationSec = 60
	}
	if durationSec > 300 {
		durationSec = 300
	}
	id := fmt.Sprintf("%d", time.Now().UnixNano())
	file := filepath.Join(m.dir, id+".pcap")
	args := []string{"-i", dev, "-w", file, "-U"}
	if filter != "" {
		args = append(args, filter)
	}
	cmd := exec.Command("tcpdump", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	s := &Session{
		ID: id, Device: dev, Filter: filter, PID: cmd.Process.Pid,
		File: file, StartedAt: time.Now(), Running: true,
	}
	m.mu.Lock()
	m.jobs[id] = s
	m.mu.Unlock()
	go func() {
		time.Sleep(time.Duration(durationSec) * time.Second)
		_ = m.Stop(id)
	}()
	go func() {
		_ = cmd.Wait()
		m.mu.Lock()
		if j, ok := m.jobs[id]; ok {
			j.Running = false
			now := time.Now()
			j.EndedAt = &now
			if fi, err := os.Stat(j.File); err == nil {
				j.Size = fi.Size()
			}
		}
		m.mu.Unlock()
	}()
	return s, nil
}

// Stop 停止抓包
func (m *Manager) Stop(id string) error {
	m.mu.Lock()
	s, ok := m.jobs[id]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("session not found")
	}
	if s.Running && s.PID > 0 {
		_ = syscall.Kill(-s.PID, syscall.SIGTERM)
		time.Sleep(200 * time.Millisecond)
		_ = syscall.Kill(-s.PID, syscall.SIGKILL)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	s.Running = false
	now := time.Now()
	s.EndedAt = &now
	if fi, err := os.Stat(s.File); err == nil {
		s.Size = fi.Size()
	}
	return nil
}

// List 所有会话
func (m *Manager) List() []Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Session, 0, len(m.jobs))
	for _, s := range m.jobs {
		cp := *s
		if fi, err := os.Stat(cp.File); err == nil {
			cp.Size = fi.Size()
		}
		out = append(out, cp)
	}
	return out
}

// Get 单条
func (m *Manager) Get(id string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.jobs[id]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	cp := *s
	return &cp, nil
}
