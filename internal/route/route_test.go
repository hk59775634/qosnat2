package route

import (
	"testing"

	"github.com/hk59775634/qosnat2/internal/store"
)

func TestListLiveMissingPolicyTable(t *testing.T) {
	// Table 199 is unlikely to exist in test env; ListLive must not fail apply path.
	list, err := ListLive(199)
	if err != nil {
		t.Fatalf("ListLive(199): %v", err)
	}
	if list == nil {
		list = []LiveRoute{}
	}
	if len(list) != 0 {
		t.Fatalf("expected empty, got %d", len(list))
	}
}

func TestBuildLiveIndexMissingPolicyTable(t *testing.T) {
	const tbl = 250
	routes := []store.RouteEntry{
		{ID: "eg", Dest: "default", Gateway: "1.2.3.4", Device: "eth1", Table: tbl, Enabled: true},
		{ID: "main", Dest: "default", Gateway: "9.9.9.9", Device: "eth0", Table: 254, Enabled: true},
	}
	if _, err := buildLiveIndex(routes); err != nil {
		t.Fatalf("buildLiveIndex should tolerate missing policy table: %v", err)
	}
}
