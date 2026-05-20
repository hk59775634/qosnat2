package main

import (
	"fmt"
	"os"

	"github.com/cilium/ebpf"
)

type bpfObjects struct {
	ConfigMap *ebpf.Map     `ebpf:"config_map"`
	CidrMap   *ebpf.Map     `ebpf:"cidr_map"`
	StateMap  *ebpf.Map     `ebpf:"state_map"`
	TcIngress *ebpf.Program `ebpf:"tc_ingress"`
	TcEgress  *ebpf.Program `ebpf:"tc_egress"`
}

func loadBPFObjects(objPath string) (*bpfObjects, error) {
	spec, err := ebpf.LoadCollectionSpec(objPath)
	if err != nil {
		return nil, err
	}
	var objs bpfObjects
	if err := spec.LoadAndAssign(&objs, nil); err != nil {
		return nil, err
	}
	return &objs, nil
}

func pinBPFObjects(objs *bpfObjects) error {
	if err := os.MkdirAll(pinDir, 0755); err != nil {
		return err
	}
	cidrPin := pinDir + "/cidr_map"
	for _, p := range []string{configPin, cidrPin, statePin, pinDir + "/tc_ingress", pinDir + "/tc_egress"} {
		_ = os.Remove(p)
	}
	if err := objs.ConfigMap.Pin(configPin); err != nil {
		return fmt.Errorf("pin config_map: %w", err)
	}
	if err := objs.CidrMap.Pin(cidrPin); err != nil {
		return fmt.Errorf("pin cidr_map: %w", err)
	}
	if err := objs.StateMap.Pin(statePin); err != nil {
		return fmt.Errorf("pin state_map: %w", err)
	}
	if err := objs.TcIngress.Pin(pinDir + "/tc_ingress"); err != nil {
		return fmt.Errorf("pin tc_ingress: %w", err)
	}
	if err := objs.TcEgress.Pin(pinDir + "/tc_egress"); err != nil {
		return fmt.Errorf("pin tc_egress: %w", err)
	}
	return nil
}

func closeBPFObjects(objs *bpfObjects) {
	if objs == nil {
		return
	}
	objs.TcIngress.Close()
	objs.TcEgress.Close()
	objs.ConfigMap.Close()
	objs.CidrMap.Close()
	objs.StateMap.Close()
}
