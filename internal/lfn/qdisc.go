package lfn

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/hk59775634/qosnat2/internal/store"
)

// Plan 对该口要执行的 tc/ip 命令；Skip 时不改根队列。
type Plan struct {
	Skip    bool
	Warning string
	Cmds    [][]string
}

// QdiscPlan 长肥口：无整形则 mq+fq（或多队列 fq）；有 clsact/HTB 只告警。
// enable=false 且 protected 时不删根（保护整形树）。
func QdiscPlan(dev string, enable, protected bool, queues, txqueuelen int) Plan {
	dev = strings.TrimSpace(dev)
	if dev == "" {
		return Plan{Skip: true}
	}
	if protected {
		if enable {
			return Plan{Skip: true, Warning: "shaper/clsact present; skip root qdisc, MSS only"}
		}
		return Plan{Skip: true}
	}
	if txqueuelen <= 0 {
		txqueuelen = store.LFNTxQueueLen
	}
	var cmds [][]string
	if enable {
		if queues > 1 {
			cmds = append(cmds, []string{"tc", "qdisc", "replace", "dev", dev, "root", "handle", "1:", "mq"})
			for i := 1; i <= queues; i++ {
				parent := fmt.Sprintf("1:%x", i)
				handle := fmt.Sprintf("%x0:", i)
				cmds = append(cmds, []string{"tc", "qdisc", "replace", "dev", dev, "parent", parent, "handle", handle, "fq"})
			}
		} else {
			cmds = append(cmds, []string{"tc", "qdisc", "replace", "dev", dev, "root", "fq"})
		}
		cmds = append(cmds, []string{"ip", "link", "set", "dev", dev, "txqueuelen", strconv.Itoa(txqueuelen)})
		return Plan{Cmds: cmds}
	}
	cmds = append(cmds, []string{"tc", "qdisc", "del", "dev", dev, "root"})
	cmds = append(cmds, []string{"ip", "link", "set", "dev", dev, "txqueuelen", strconv.Itoa(txqueuelen)})
	return Plan{Cmds: cmds}
}

// ExecPlan 执行计划；del root 在无根 qdisc 时忽略错误。
func ExecPlan(p Plan) error {
	if p.Skip {
		return nil
	}
	for _, args := range p.Cmds {
		if len(args) == 0 {
			continue
		}
		out, err := exec.Command(args[0], args[1:]...).CombinedOutput()
		if err == nil {
			continue
		}
		msg := strings.TrimSpace(string(out))
		if isBenignQdiscErr(args, msg) {
			continue
		}
		return fmt.Errorf("%s: %s %w", strings.Join(args, " "), msg, err)
	}
	return nil
}

func isBenignQdiscErr(args []string, msg string) bool {
	if len(args) >= 3 && args[0] == "tc" && args[1] == "qdisc" && args[2] == "del" {
		return strings.Contains(msg, "No such file") || strings.Contains(msg, "Invalid argument") ||
			strings.Contains(msg, "Cannot delete") || msg == ""
	}
	return strings.Contains(msg, "File exists")
}

// QdiscShow 现场 qdisc 摘要。
func QdiscShow(dev string) string {
	if strings.TrimSpace(dev) == "" {
		return ""
	}
	out, err := exec.Command("tc", "qdisc", "show", "dev", dev).CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// QueueCount TX 队列数；失败时 1。
func QueueCount(dev string) int {
	dev = strings.TrimSpace(dev)
	if dev == "" {
		return 1
	}
	matches, _ := os.ReadDir("/sys/class/net/" + dev + "/queues")
	n := 0
	for _, e := range matches {
		if strings.HasPrefix(e.Name(), "tx-") {
			n++
		}
	}
	if n <= 0 {
		return 1
	}
	return n
}

// LinkMTU 读取接口 MTU。
func LinkMTU(dev string) int {
	b, err := os.ReadFile("/sys/class/net/" + strings.TrimSpace(dev) + "/mtu")
	if err != nil {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return 0
	}
	return n
}
