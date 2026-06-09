package api

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/hk59775634/qosnat2/internal/shaper"
)

// verifyUploadPath 应用策略后检查上行 ifb 管道是否就绪
func (srv *Server) verifyUploadPath(mirredCIDRs []string) error {
	if srv.env.DevLAN == "" {
		return nil
	}
	lan := srv.env.DevLAN
	out, _ := exec.Command("tc", "filter", "show", "dev", lan, "ingress").CombinedOutput()
	s := string(out)
	if strings.Contains(s, "classify_ingress") {
		return fmt.Errorf("%s ingress: LAN BPF blocks u32 mirred (use ifb0 ingress only)", lan)
	}
	if len(mirredCIDRs) > 0 {
		if !strings.Contains(s, "mirred") {
			return fmt.Errorf("%s ingress: missing u32 mirred to ifb", lan)
		}
		if strings.Contains(s, "flower") {
			return fmt.Errorf("%s ingress: legacy flower mirred still present (need u32)", lan)
		}
		if !strings.Contains(s, "u32") {
			return fmt.Errorf("%s ingress: missing u32 mirred to ifb", lan)
		}
		if !strings.Contains(s, "pref 5") || !strings.Contains(s, "match ip dst") {
			return fmt.Errorf("%s ingress: missing local dst skip (prio 5, LAN bypass IFB)", lan)
		}
	}
	ifbOut, _ := exec.Command("tc", "qdisc", "show", "dev", shaper.IFBDev).CombinedOutput()
	if !strings.Contains(string(ifbOut), "htb") {
		return fmt.Errorf("%s: missing HTB root", shaper.IFBDev)
	}
	ifbIng, _ := exec.Command("tc", "filter", "show", "dev", shaper.IFBDev, "ingress").CombinedOutput()
	if !strings.Contains(string(ifbIng), "classify_ingress") {
		return fmt.Errorf("%s ingress: missing BPF classifier", shaper.IFBDev)
	}
	u32, _ := exec.Command("tc", "filter", "show", "dev", shaper.IFBDev, "parent", "1:").CombinedOutput()
	if len(mirredCIDRs) > 0 && !strings.Contains(string(u32), "u32") {
		return fmt.Errorf("%s: missing upload u32 filters", shaper.IFBDev)
	}
	return nil
}
