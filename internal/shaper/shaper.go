package shaper

import (
	"fmt"
	"os/exec"
	"strings"
)

// TeardownDevice 移除单网卡上的 QoS qdisc/filter（保留网卡本身）
func TeardownDevice(dev string) {
	if dev == "" {
		return
	}
	_ = exec.Command("tc", "filter", "del", "dev", dev, "ingress").Run()
	_ = exec.Command("tc", "filter", "del", "dev", dev, "egress").Run()
	_ = exec.Command("tc", "qdisc", "del", "dev", dev, "clsact").Run()
	_ = exec.Command("tc", "qdisc", "del", "dev", dev, "root").Run()
}

func ensureClsact(dev string) error {
	if out, err := exec.Command("tc", "qdisc", "add", "dev", dev, "clsact").CombinedOutput(); err != nil {
		msg := string(out)
		if strings.Contains(msg, "File exists") || strings.Contains(msg, "Exclusivity flag on") {
			return nil
		}
		return fmt.Errorf("tc clsact %s: %s %w", dev, strings.TrimSpace(msg), err)
	}
	return nil
}
