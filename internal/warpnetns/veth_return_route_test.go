package warpnetns

import "testing"

func TestEnsureNetnsVethReturnRouteNoNetns(t *testing.T) {
	// 无 root / 无 netns 时必须安全 no-op，不能 panic。
	ensureNetnsVethReturnRoute()
}
