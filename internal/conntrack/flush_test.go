package conntrack

import "testing"

func TestFlushByCIDRsEmpty(t *testing.T) {
	FlushByCIDRs(nil)
	FlushByCIDRs([]string{"", "0.0.0.0/0"})
}
