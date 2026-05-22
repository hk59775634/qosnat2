package dnsmasq

import "testing"

func TestParseLeasesIPv4(t *testing.T) {
	raw := "1700000000 aa:bb:cc:dd:ee:ff 192.168.1.50 host *\n"
	list := ParseLeases(raw)
	if len(list) != 1 || list[0].IP != "192.168.1.50" || list[0].Family != "ipv4" {
		t.Fatalf("%+v", list)
	}
}
