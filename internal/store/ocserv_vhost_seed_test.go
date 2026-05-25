package store

import "testing"

func TestVhostFromGlobalCopiesNetworkAndAdvanced(t *testing.T) {
	o := DefaultOCServ()
	o.IPv4Network = "10.9.0.0"
	o.DNS = []string{"1.1.1.1"}
	o.Advanced.Compression = true
	o.Advanced.Keepalive = true
	o.Advanced.KeepaliveSec = 3600
	o.AuthMethod = OCServAuthPlain

	v := VhostFromGlobal(o, "vpn.test", "note", "")
	if v.Domain != "vpn.test" || v.Comment != "note" {
		t.Fatalf("domain/comment: %+v", v)
	}
	if v.AuthMethod != OCServAuthPlain {
		t.Fatalf("auth: %s", v.AuthMethod)
	}
	if v.IPv4Network != "10.9.0.0" || len(v.DNS) != 1 || v.DNS[0] != "1.1.1.1" {
		t.Fatalf("network: %+v", v)
	}
	if !v.Compression || v.Keepalive != 3600 {
		t.Fatalf("advanced map: compression=%v keepalive=%d", v.Compression, v.Keepalive)
	}
	if v.Radius != nil {
		t.Fatal("radius should be nil to inherit global")
	}
	if v.PlainPasswdPath != "" {
		t.Fatal("plain passwd path should be empty to inherit global")
	}
}
