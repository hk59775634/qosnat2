package frr

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

var daemonsLine = regexp.MustCompile(`^(bgpd|ospfd)=(yes|no)\s*$`)

// SyncDaemons 按需求启用/禁用 bgpd、ospfd；若 daemons 变更则 restart frr。
func SyncDaemons(bgp, ospf bool) (restarted bool, err error) {
	if !PackageInstalled() {
		return false, nil
	}
	body, err := os.ReadFile(DaemonsPath)
	if err != nil {
		return false, fmt.Errorf("read daemons: %w", err)
	}
	changed := false
	lines := strings.Split(string(body), "\n")
	for i, line := range lines {
		m := daemonsLine.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		want := "no"
		switch m[1] {
		case "bgpd":
			if bgp {
				want = "yes"
			}
		case "ospfd":
			if ospf {
				want = "yes"
			}
		}
		newLine := m[1] + "=" + want
		if strings.TrimSpace(line) != newLine {
			lines[i] = newLine
			changed = true
		}
	}
	if !changed {
		return false, nil
	}
	newBody := strings.Join(lines, "\n")
	if !strings.HasSuffix(newBody, "\n") {
		newBody += "\n"
	}
	if err := os.WriteFile(DaemonsPath, []byte(newBody), 0644); err != nil {
		return false, err
	}
	out, err := exec.Command("systemctl", "restart", "frr").CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("systemctl restart frr: %s %w", strings.TrimSpace(string(out)), err)
	}
	return true, nil
}
