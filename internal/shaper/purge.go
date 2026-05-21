package shaper

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

var tcClassID = regexp.MustCompile(`class htb 1:([0-9a-f]+)`)

// PurgeIFBShaperArtifacts 删除 ifb0 上所有动态 u32 与 HTB 子类（保留默认 1:1）
func PurgeIFBShaperArtifacts() error {
	for i := 0; i < 128; i++ {
		out, _ := exec.Command("tc", "filter", "del", "dev", IFBDev, "parent", "1:",
			"protocol", "ip", "u32").CombinedOutput()
		msg := string(out)
		if strings.Contains(msg, "No such file") || strings.Contains(msg, "Cannot find") {
			break
		}
	}
	out, err := exec.Command("tc", "class", "show", "dev", IFBDev).CombinedOutput()
	if err != nil {
		return fmt.Errorf("tc class show %s: %w", IFBDev, err)
	}
	for _, m := range tcClassID.FindAllStringSubmatch(string(out), -1) {
		if len(m) < 2 || m[1] == "1" {
			continue
		}
		cid := "1:" + m[1]
		_ = exec.Command("tc", "qdisc", "del", "dev", IFBDev, "parent", cid).Run()
		_ = exec.Command("tc", "class", "del", "dev", IFBDev, "classid", cid).Run()
	}
	return nil
}

// PurgeLANShaperArtifacts 删除 LAN/ifb 上 per-host HTB（保留根类 1:1）
func PurgeLANShaperArtifacts(devLAN string) error {
	for _, dev := range []string{devLAN, IFBDev} {
		if dev == "" {
			continue
		}
		out, err := exec.Command("tc", "class", "show", "dev", dev).CombinedOutput()
		if err != nil {
			continue
		}
		for _, m := range tcClassID.FindAllStringSubmatch(string(out), -1) {
			if len(m) < 2 || m[1] == "1" {
				continue
			}
			cid := "1:" + m[1]
			_ = exec.Command("tc", "qdisc", "del", "dev", dev, "parent", cid).Run()
			_ = exec.Command("tc", "class", "del", "dev", dev, "classid", cid).Run()
		}
	}
	return nil
}
