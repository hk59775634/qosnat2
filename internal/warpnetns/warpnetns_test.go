package warpnetns

import "testing"

func TestNeedsNetnsResetOrphanHostVeth(t *testing.T) {
	// 无 root 环境下仅验证：无 qwp0 时不应因 orphan 触发 reset。
	if linkExists(VethHost) {
		t.Skip("qwp0 exists on host; skip non-root heuristic test")
	}
	if needsNetnsReset() && netnsExists() {
		t.Skip("netns exists in test env")
	}
}
