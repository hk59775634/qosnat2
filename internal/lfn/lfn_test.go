package lfn

import (
	"strings"
	"testing"

	"github.com/hk59775634/qosnat2/internal/store"
	"github.com/hk59775634/qosnat2/internal/sysctl"
)

func TestOverlayOnOff(t *testing.T) {
	base := map[string]string{"net.ipv4.ip_forward": "1"}
	on := Overlay(base, true)
	if on["net.ipv4.tcp_congestion_control"] != "bbr" {
		t.Fatalf("want bbr, got %v", on)
	}
	if on["net.core.default_qdisc"] != "fq" {
		t.Fatal(on["net.core.default_qdisc"])
	}
	if on["net.ipv4.tcp_rmem"] != "4096 262144 67108864" {
		t.Fatal(on["net.ipv4.tcp_rmem"])
	}
	off := Overlay(base, false)
	if _, ok := off["net.ipv4.tcp_congestion_control"]; ok {
		t.Fatalf("off overlay must not inject bbr: %v", off)
	}
	merged := sysctl.Merge(off, false)
	if merged["net.ipv4.tcp_congestion_control"] != "cubic" {
		t.Fatalf("restore cubic via Defaults, got %q", merged["net.ipv4.tcp_congestion_control"])
	}
	if merged["net.core.default_qdisc"] != "fq_codel" {
		t.Fatalf("restore fq_codel, got %q", merged["net.core.default_qdisc"])
	}
	if base["net.ipv4.tcp_congestion_control"] != "" {
		t.Fatal("must not mutate input")
	}
}

func TestQdiscPlanProtectedSkip(t *testing.T) {
	p := QdiscPlan("ens19", true, true, 8, store.LFNTxQueueLen)
	if !p.Skip || p.Warning == "" {
		t.Fatalf("%+v", p)
	}
	if len(p.Cmds) != 0 {
		t.Fatalf("cmds=%v", p.Cmds)
	}
	off := QdiscPlan("ens19", false, true, 8, 5000)
	if !off.Skip {
		t.Fatal("must not del root on shaper port")
	}
}

func TestQdiscPlanMQAndSingle(t *testing.T) {
	p := QdiscPlan("ens18", true, false, 4, store.LFNTxQueueLen)
	if p.Skip || len(p.Cmds) < 3 {
		t.Fatalf("%+v", p)
	}
	if strings.Join(p.Cmds[0], " ") != "tc qdisc replace dev ens18 root handle 1: mq" {
		t.Fatalf("root=%v", p.Cmds[0])
	}
	if !strings.Contains(strings.Join(p.Cmds[1], " "), "parent 1:1") || !strings.Contains(strings.Join(p.Cmds[1], " "), " fq") {
		t.Fatalf("leaf=%v", p.Cmds[1])
	}
	last := strings.Join(p.Cmds[len(p.Cmds)-1], " ")
	if last != "ip link set dev ens18 txqueuelen 10000" {
		t.Fatalf("txq=%s", last)
	}
	s := QdiscPlan("ens18", true, false, 1, 10000)
	if strings.Join(s.Cmds[0], " ") != "tc qdisc replace dev ens18 root fq" {
		t.Fatalf("single=%v", s.Cmds[0])
	}
	d := QdiscPlan("ens18", false, false, 4, 5000)
	if strings.Join(d.Cmds[0], " ") != "tc qdisc del dev ens18 root" {
		t.Fatalf("del=%v", d.Cmds[0])
	}
}

func TestLiveProtected(t *testing.T) {
	if !LiveProtected("qdisc fq 8001: root\nqdisc clsact ffff:") {
		t.Fatal("clsact")
	}
	if !LiveProtected("qdisc htb 1: root") {
		t.Fatal("htb")
	}
	if LiveProtected("qdisc mq 0: root\nqdisc fq 0: parent 1:1") {
		t.Fatal("plain mq+fq is not protected")
	}
	if !DeviceProtected("ens19", []string{"ens19", "wg0"}, "") {
		t.Fatal("shaper list")
	}
	if DeviceProtected("ens18", []string{"ens19"}, "qdisc fq 1: root") {
		t.Fatal("wan not shaper")
	}
}

func TestTwoIfacesLastOff(t *testing.T) {
	st := store.State{}
	on := true
	store.UpsertIfaceConfig(&st, "ens18", nil, nil, nil, nil, nil, &on, nil)
	store.UpsertIfaceConfig(&st, "ens20", nil, nil, nil, nil, nil, &on, nil)
	if !store.AnyLFNEnabled(st) {
		t.Fatal("two on")
	}
	off := false
	store.UpsertIfaceConfig(&st, "ens20", nil, nil, nil, nil, nil, &off, nil)
	if !store.AnyLFNEnabled(st) {
		t.Fatal("one still on")
	}
	store.UpsertIfaceConfig(&st, "ens18", nil, nil, nil, nil, nil, &off, nil)
	if store.AnyLFNEnabled(st) {
		t.Fatal("last off")
	}
}
