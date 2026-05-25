package ocserv

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/hk59775634/qosnat2/internal/store"
)

const OcctlPath = "/usr/local/bin/occtl"

// OcctlConfig socket 与 use-occtl 开关（来自持久化配置）
type OcctlConfig struct {
	SocketFile string
	UseOcctl   bool
}

// OcctlFromState 从 ocserv 配置构造 occtl 调用参数
func OcctlFromState(o store.OCServState) OcctlConfig {
	sock := strings.TrimSpace(o.SocketFile)
	if sock == "" {
		sock = "/var/run/ocserv-socket"
	}
	return OcctlConfig{
		SocketFile: sock,
		UseOcctl:   o.Advanced.UseOcctl,
	}
}

func (c OcctlConfig) checkReady() error {
	if !c.UseOcctl {
		return fmt.Errorf("occtl 未启用：请在高级配置中开启 use-occtl 并保存应用")
	}
	st := InstallInfo()
	if !st.Installed {
		return fmt.Errorf("ocserv 未安装")
	}
	if !st.Active {
		return fmt.Errorf("ocserv 未运行")
	}
	return nil
}

func occtlBin() string {
	if p, err := exec.LookPath("occtl"); err == nil {
		return p
	}
	return OcctlPath
}

// occtlSocketArg 仅在配置的 socket-file 路径本身存在时使用 -s。
// isolate-workers 会在 /var/run 下生成带哈希后缀的 socket，该路径对 occtl 往往不可写；
// 此时应省略 -s，由 occtl 自行发现（与命令行直接执行 occtl -j show users 一致）。
func (c OcctlConfig) occtlSocketArg() (extra []string) {
	sock := strings.TrimSpace(c.SocketFile)
	if sock == "" {
		return nil
	}
	fi, err := os.Stat(sock)
	if err != nil || fi.Mode()&os.ModeSocket == 0 {
		return nil
	}
	return []string{"-s", sock}
}

func (c OcctlConfig) run(args ...string) ([]byte, error) {
	if err := c.checkReady(); err != nil {
		return nil, err
	}
	cmdArgs := []string{"-j"}
	cmdArgs = append(cmdArgs, c.occtlSocketArg()...)
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.Command(occtlBin(), cmdArgs...)
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if len(text) > 0 && json.Valid([]byte(text)) {
		return []byte(text), nil
	}
	if err != nil {
		if text == "" {
			text = err.Error()
		}
		return nil, fmt.Errorf("%s", text)
	}
	return []byte(text), nil
}

// ShowStatus occtl show status -j
func (c OcctlConfig) ShowStatus() (map[string]any, error) {
	out, err := c.run("show", "status")
	if err != nil {
		return nil, err
	}
	var st map[string]any
	if err := json.Unmarshal(out, &st); err != nil {
		return nil, fmt.Errorf("parse occtl status: %w", err)
	}
	return st, nil
}

// ShowUsers occtl show users -j
func (c OcctlConfig) ShowUsers() ([]map[string]any, error) {
	out, err := c.run("show", "users")
	if err != nil {
		return nil, err
	}
	var users []map[string]any
	if err := json.Unmarshal(out, &users); err != nil {
		return nil, fmt.Errorf("parse occtl users: %w", err)
	}
	if users == nil {
		users = []map[string]any{}
	}
	return users, nil
}

// ShowUser occtl show user NAME -j（当前连接信息，含 RX/TX）
func (c OcctlConfig) ShowUser(username string) ([]map[string]any, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, fmt.Errorf("username required")
	}
	out, err := c.run("show", "user", username)
	if err != nil {
		return nil, err
	}
	var rows []map[string]any
	if err := json.Unmarshal(out, &rows); err != nil {
		return nil, fmt.Errorf("parse occtl user: %w", err)
	}
	if rows == nil {
		rows = []map[string]any{}
	}
	return rows, nil
}

// DisconnectUser occtl disconnect user NAME
func (c OcctlConfig) DisconnectUser(username string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return fmt.Errorf("username required")
	}
	_, err := c.run("disconnect", "user", username)
	return err
}

// DisconnectID occtl disconnect id ID
func (c OcctlConfig) DisconnectID(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("id required")
	}
	_, err := c.run("disconnect", "id", id)
	return err
}
