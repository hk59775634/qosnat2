package store

import "testing"

func TestWarpWanLinkPolicyOnly(t *testing.T) {
	w := WarpWanLink("CloudflareWARP")
	if w.ID != WanLinkIDWarp || !w.PolicyOnly || !w.WarpManaged {
		t.Fatalf("unexpected warp wan: %+v", w)
	}
}

func TestRemoveWarpWanLinkKeepsEgress(t *testing.T) {
	st := &State{
		Network: NetworkState{
			WanLinks: []WanLink{WarpWanLink("CloudflareWARP"), {ID: "wan-2", Device: "ens19", Enabled: true}},
			EgressPolicies: []EgressPolicy{
				{ID: "eg-1", CIDR: "104.16.0.0/13", WanLinkID: WanLinkIDWarp, Enabled: true},
				{ID: "eg-2", CIDR: "10.0.0.0/8", WanLinkID: "wan-2", Enabled: true},
			},
		},
	}
	RemoveWarpWanLink(st)
	if len(st.Network.WanLinks) != 1 || st.Network.WanLinks[0].ID != "wan-2" {
		t.Fatalf("wan links: %+v", st.Network.WanLinks)
	}
	if len(st.Network.EgressPolicies) != 2 {
		t.Fatalf("egress should be kept: %+v", st.Network.EgressPolicies)
	}
}
