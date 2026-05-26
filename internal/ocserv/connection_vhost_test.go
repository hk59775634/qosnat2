package ocserv

import (
	"testing"

	"github.com/hk59775634/qosnat2/internal/store"
)

func TestBuildVhostConnectionInfo_usesDomain(t *testing.T) {
	global := store.DefaultOCServ()
	global.TCPPort = 443
	v := store.OCServVhost{Domain: "test.example.nip.io"}
	ci := BuildVhostConnectionInfo(v, global, nil)
	if ci.URL != "https://test.example.nip.io" {
		t.Fatalf("url=%q", ci.URL)
	}
}
