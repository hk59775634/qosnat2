package shaper

import (
	"testing"
)

func TestHostConfiguredPerDevice(t *testing.T) {
	h := NewHostShaper("ens19", "fq_codel")
	const minor = uint32(0x1ab)
	const down = uint64(1_000_000)
	const up = uint64(500_000)

	if h.HostConfiguredOnDevice("10.0.0.1", "ens19", minor, down, up) {
		t.Fatal("expected not configured initially")
	}

	h.markDevice("10.0.0.1", "ens19", minor, down, up)
	if !h.HostConfiguredOnDevice("10.0.0.1", "ens19", minor, down, up) {
		t.Fatal("expected ens19 configured")
	}
	if h.HostConfiguredOnDevice("10.0.0.1", "wg0", minor, down, up) {
		t.Fatal("wg0 should not be configured when only ens19 was marked")
	}

	h.markDevice("10.0.0.1", "wg0", minor, down, up)
	if !h.HostConfiguredOnDevice("10.0.0.1", "wg0", minor, down, up) {
		t.Fatal("expected wg0 configured after mark")
	}
}

func TestHostFullyConfiguredRequiresExtraDev(t *testing.T) {
	h := NewHostShaper("ens19", "fq_codel")
	h.SetExtraDev("wg0")
	const minor = uint32(0x1ac)
	const down = uint64(2_000_000)
	const up = uint64(1_000_000)

	h.markDevice("10.0.0.2", "ens19", minor, down, up)
	if h.hostFullyConfigured("10.0.0.2", minor, down, up, "wg0") {
		t.Fatal("expected not fully configured without wg0 egress")
	}

	h.markDevice("10.0.0.2", "wg0", minor, down, up)
	if !h.hostFullyConfigured("10.0.0.2", minor, down, up, "wg0") {
		t.Fatal("expected fully configured with lan+wg0+ifb")
	}
}
