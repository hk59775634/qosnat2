package netif

import (
	"strings"
	"testing"
)

func TestValidateIfaceName(t *testing.T) {
	ok := []string{"ens19", "eth0", "wg0", "br-lan", "veth0.1"}
	for _, d := range ok {
		if err := ValidateIfaceName(d); err != nil {
			t.Fatalf("%q: %v", d, err)
		}
	}
	bad := []string{"", "lo/../x", "eth 0", "a/b", strings.Repeat("x", 16)}
	for _, d := range bad {
		if err := ValidateIfaceName(d); err == nil {
			t.Fatalf("expected error for %q", d)
		}
	}
}
