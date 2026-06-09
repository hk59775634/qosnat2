package ocserv

import (
	"strings"
	"testing"

	"github.com/hk59775634/qosnat2/internal/store"
)

func TestRenderGroupGlobalsOmitSelectGroup(t *testing.T) {
	o := store.DefaultOCServ()
	o.Groups = []store.OCServGroup{
		{Name: "hk", Label: "HK"},
		{Name: "internal", Label: "Internal", OmitSelectGroup: true},
		{Name: "us", Label: "US"},
	}
	conf := RenderConf(o, nil)
	if !strings.Contains(conf, "select-group = hk[HK]") {
		t.Fatalf("missing hk select-group:\n%s", conf)
	}
	if !strings.Contains(conf, "select-group = us[US]") {
		t.Fatalf("missing us select-group:\n%s", conf)
	}
	if strings.Contains(conf, "select-group = internal") {
		t.Fatalf("internal must not be in select-group:\n%s", conf)
	}
}
