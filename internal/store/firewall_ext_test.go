package store

import (
	"testing"
	"time"
)

func TestParsePortSpec(t *testing.T) {
	parts, err := ParsePortSpec("80,443,8000-8010")
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 3 {
		t.Fatalf("%v", parts)
	}
}

func TestFilterRuleNftLinePortsAndLog(t *testing.T) {
	r := FilterRule{
		ID: "fr-x", Chain: "forward", Action: "drop", Proto: "tcp",
		DstPorts: "80,443", Log: true, Counter: true, Enabled: true,
	}
	line := r.NftRuleLine()
	if !containsAll(line, "dport { 80, 443 }", "counter", `log prefix "qosnat2-fw "`, "drop") {
		t.Fatalf("%s", line)
	}
}

func TestFilterRuleOutputChain(t *testing.T) {
	r := FilterRule{Chain: "output", Action: "drop", Oif: "ens19", Enabled: true}
	if err := NormalizeFilterRule(&r); err != nil {
		t.Fatal(err)
	}
	if r.Iif != "" {
		t.Fatal("output must clear iif")
	}
}

func TestScheduleActive(t *testing.T) {
	s := Schedule{
		ID: "sch-1", Name: "biz", Enabled: true,
		Ranges: []ScheduleRange{{Weekdays: "1,2,3,4,5", Start: "09:00", End: "18:00"}},
	}
	// Monday 10:00
	mon := time.Date(2026, 7, 20, 10, 0, 0, 0, time.Local) // 2026-07-20 is Monday
	if mon.Weekday() != time.Monday {
		t.Fatalf("fixture weekday %v", mon.Weekday())
	}
	if !ScheduleActiveAt(s, mon) {
		t.Fatal("expected active")
	}
	night := time.Date(2026, 7, 20, 20, 0, 0, 0, time.Local)
	if ScheduleActiveAt(s, night) {
		t.Fatal("expected inactive")
	}
}

func TestRuleEffectivelyEnabledSchedule(t *testing.T) {
	sch := []Schedule{{
		ID: "sch-1", Name: "biz", Enabled: true,
		Ranges: []ScheduleRange{{Start: "00:00", End: "23:59"}},
	}}
	r := FilterRule{ID: "fr-1", Enabled: true, ScheduleID: "sch-1", Chain: "forward", Action: "drop"}
	if !RuleEffectivelyEnabled(r, sch, time.Now()) {
		t.Fatal("expected enabled")
	}
	r.ScheduleID = "missing"
	if RuleEffectivelyEnabled(r, sch, time.Now()) {
		t.Fatal("missing schedule should disable")
	}
}
