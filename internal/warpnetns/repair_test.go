package warpnetns

import (
	"strings"
	"testing"
)

func TestHostVethPeerBrokenRequiresMissingPeer(t *testing.T) {
	if !linkExists(VethHost) {
		t.Skip("no qwp0 on host")
	}
	out, _ := run("ip", "-d", "-o", "link", "show", VethHost)
	if !strings.Contains(string(out), "link-netnsid 0") {
		t.Skip("qwp0 does not show link-netnsid 0 in this environment")
	}
	if netnsUsable() {
		_, err := netnsExec("ip", "link", "show", VethNS)
		if err == nil && hostVethPeerBroken() {
			t.Fatal("hostVethPeerBroken true while qwp1 exists in netns")
		}
	}
}
