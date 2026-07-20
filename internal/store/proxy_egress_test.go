package store

import "testing"

func TestNormalizeProxyEgress(t *testing.T) {
	p := ProxyEgress{Name: "US-1", Type: "socks", Server: "1.2.3.4", Port: 1080}
	if err := NormalizeProxyEgress(&p); err != nil {
		t.Fatal(err)
	}
	if p.Type != "socks5" {
		t.Fatalf("type=%s", p.Type)
	}
	if p.ID == "" {
		t.Fatal("id empty")
	}
	if err := NormalizeProxyEgress(&ProxyEgress{Type: "ftp", Server: "x", Port: 1}); err == nil {
		t.Fatal("expected type error")
	}
}

func TestAllocateProxyTunIndex(t *testing.T) {
	existing := []ProxyEgress{{TunIndex: 0}, {TunIndex: 2}}
	idx, err := AllocateProxyTunIndex(existing)
	if err != nil {
		t.Fatal(err)
	}
	if idx != 1 {
		t.Fatalf("idx=%d want 1", idx)
	}
}

func TestProxyWanLinkUpsertSync(t *testing.T) {
	st := &State{Network: NetworkState{}}
	p := ProxyEgress{ID: "pe-aabbcc", Name: "A", Type: "socks5", Server: "9.9.9.9", Port: 1080, TunIndex: 3, Enabled: true}
	UpsertProxyWanLink(st, p)
	if len(st.Network.WanLinks) != 1 {
		t.Fatalf("links=%d", len(st.Network.WanLinks))
	}
	w := st.Network.WanLinks[0]
	if !IsProxyWanLink(w) || w.Device != "qpe3" || !w.PolicyOnly {
		t.Fatalf("unexpected wan link: %+v", w)
	}
	st.Network.ProxyEgress = []ProxyEgress{p}
	st.Network.WanLinks = append(st.Network.WanLinks, WanLink{ID: "wan-proxy-orphan", ProxyManaged: true, Device: "qpe9"})
	SyncProxyWanLinks(st)
	if len(st.Network.WanLinks) != 1 || st.Network.WanLinks[0].ID != ProxyWanLinkID(p.ID) {
		t.Fatalf("sync result: %+v", st.Network.WanLinks)
	}
	RemoveProxyWanLink(st, p.ID)
	if len(st.Network.WanLinks) != 0 {
		t.Fatalf("remove failed: %+v", st.Network.WanLinks)
	}
}

func TestProxyTunAddress(t *testing.T) {
	if ProxyTunAddress(5) != "198.18.6.1/30" {
		t.Fatal(ProxyTunAddress(5))
	}
	if ProxyTunDevice(5) != "qpe5" {
		t.Fatal(ProxyTunDevice(5))
	}
}
