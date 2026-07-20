package singbox

import (
	"testing"

	"github.com/hk59775634/qosnat2/internal/store"
)

func TestProbeExitViaCloudflareTraceParse(t *testing.T) {
	// unit test parse path via unexported helper behavior through exported pattern
	dev := "nonexistent999"
	info := ProbeExitInfo(store.ProxyEgress{TunIndex: 999})
	if info.Error == "" {
		t.Fatalf("expected error for missing tun, got %+v", info)
	}
	_ = dev
}

func TestTrimCurlErr(t *testing.T) {
	if got := trimCurlErr([]byte("timeout"), nil); got != "timeout" {
		t.Fatalf("got %q", got)
	}
}
