package store

import "testing"

func TestFilterRuleMutable(t *testing.T) {
	if FilterRuleMutable(FilterRule{ID: "fr-abc", Enabled: true}) != true {
		t.Fatal("user rule should be mutable")
	}
	if FilterRuleMutable(FilterRule{ID: "fr-abc", System: true}) != false {
		t.Fatal("system rule should be immutable")
	}
	if FilterRuleMutable(FilterRule{ID: "auto-admin"}) != false {
		t.Fatal("auto- prefix should be immutable")
	}
	if FilterRuleMutable(FilterRule{ID: ""}) != false {
		t.Fatal("empty id should be immutable")
	}
}
