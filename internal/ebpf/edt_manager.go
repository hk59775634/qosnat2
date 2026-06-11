package ebpf

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cilium/ebpf"
	"github.com/hk59775634/qosnat2/internal/store"
	"github.com/hk59775634/qosnat2/internal/webassets"
)

const (
	defaultEDTObj     = "/usr/lib/qosnat2/rate_edt.bpf.o"
	edtProgIngress    = "rate_limit_ingress"
	edtProgEgress     = "rate_limit_egress"
	mapThrottle       = "throttle"
	mapTokenBucket    = "token_bucket"
)

type edtBpfObjects struct {
	ProfileLpm  *ebpf.Map     `ebpf:"profile_lpm"`
	HostExact   *ebpf.Map     `ebpf:"host_exact"`
	Throttle    *ebpf.Map     `ebpf:"throttle"`
	TokenBucket *ebpf.Map     `ebpf:"token_bucket"`
	Ingress     *ebpf.Program `ebpf:"rate_limit_ingress"`
	Egress      *ebpf.Program `ebpf:"rate_limit_egress"`
}

func (o *edtBpfObjects) close() {
	if o == nil {
		return
	}
	if o.Ingress != nil {
		o.Ingress.Close()
	}
	if o.Egress != nil {
		o.Egress.Close()
	}
	if o.ProfileLpm != nil {
		o.ProfileLpm.Close()
	}
	if o.HostExact != nil {
		o.HostExact.Close()
	}
	if o.Throttle != nil {
		o.Throttle.Close()
	}
	if o.TokenBucket != nil {
		o.TokenBucket.Close()
	}
}

func (m *Manager) SetMode(mode string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mode = store.EffectiveShaperMode(store.ShaperState{Mode: mode})
}

// Mode 当前数据面模式
func (m *Manager) Mode() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.mode == "" {
		return store.ShaperModeEDT
	}
	return m.mode
}

func (m *Manager) UsesEDT() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.mode != store.ShaperModeHTB
}

func (m *Manager) edtObjectPath() string {
	if p := os.Getenv("BPF_EDT_OBJ"); p != "" {
		return p
	}
	return defaultEDTObj
}

func (m *Manager) loadEDTCollectionSpec() (*ebpf.CollectionSpec, error) {
	if p := os.Getenv("BPF_EDT_OBJ"); p != "" {
		return ebpf.LoadCollectionSpec(p)
	}
	if webassets.Enabled() && len(webassets.BPFEDT) > 0 {
		return ebpf.LoadCollectionSpecFromReader(bytes.NewReader(webassets.BPFEDT))
	}
	path := m.edtObjectPath()
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("edt bpf object %s: %w (run: make bpf -C bpf)", path, err)
	}
	return ebpf.LoadCollectionSpec(path)
}

func (m *Manager) loadEDT() error {
	spec, err := m.loadEDTCollectionSpec()
	if err != nil {
		return err
	}
	opts := &ebpf.CollectionOptions{MapReplacements: loadPinnedMapReplacementsEDT(spec)}
	var objs edtBpfObjects
	if err := spec.LoadAndAssign(&objs, opts); err != nil {
		return err
	}
	for _, pm := range opts.MapReplacements {
		pm.Close()
	}
	if err := m.pinEDTMaps(&objs); err != nil {
		objs.close()
		return err
	}
	if err := m.pinEDTPrograms(&objs); err != nil {
		objs.close()
		return err
	}
	m.edtObjs = &objs
	m.objs = nil
	m.loaded = true
	return nil
}

func loadPinnedMapReplacementsEDT(spec *ebpf.CollectionSpec) map[string]*ebpf.Map {
	reps := make(map[string]*ebpf.Map)
	for _, name := range []string{mapProfile, mapHost, mapThrottle, mapTokenBucket} {
		if _, ok := spec.Maps[name]; !ok {
			continue
		}
		path := filepath.Join(PinDir, name)
		pm, err := ebpf.LoadPinnedMap(path, nil)
		if err != nil {
			continue
		}
		reps[name] = pm
	}
	return reps
}

func (m *Manager) pinEDTMaps(objs *edtBpfObjects) error {
	if err := os.MkdirAll(PinDir, 0755); err != nil {
		return err
	}
	pins := []struct {
		name string
		pin  func(string) error
	}{
		{mapProfile, objs.ProfileLpm.Pin},
		{mapHost, objs.HostExact.Pin},
		{mapThrottle, objs.Throttle.Pin},
		{mapTokenBucket, objs.TokenBucket.Pin},
	}
	for _, p := range pins {
		path := filepath.Join(PinDir, p.name)
		if _, err := os.Stat(path); err == nil {
			continue
		}
		_ = os.Remove(path)
		if err := p.pin(path); err != nil {
			return fmt.Errorf("pin %s: %w", p.name, err)
		}
	}
	return nil
}

func (m *Manager) pinEDTPrograms(objs *edtBpfObjects) error {
	progs := []struct {
		name string
		prog *ebpf.Program
	}{
		{edtProgIngress, objs.Ingress},
		{edtProgEgress, objs.Egress},
	}
	for _, p := range progs {
		path := filepath.Join(PinDir, p.name)
		_ = os.Remove(path)
		if err := p.prog.Pin(path); err != nil {
			return fmt.Errorf("pin %s: %w", p.name, err)
		}
	}
	return nil
}

func (m *Manager) flushEDTRuntimeMaps() error {
	if m.edtObjs == nil {
		return nil
	}
	if err := m.flushProfileLpmEDT(); err != nil {
		return err
	}
	if err := flushMapIter(m.edtObjs.HostExact.Delete, m.edtObjs.HostExact.Iterate()); err != nil {
		return err
	}
	if err := flushMapIter(m.edtObjs.Throttle.Delete, m.edtObjs.Throttle.Iterate()); err != nil {
		return err
	}
	if err := flushMapIter(m.edtObjs.TokenBucket.Delete, m.edtObjs.TokenBucket.Iterate()); err != nil {
		return err
	}
	return nil
}

func (m *Manager) flushProfileLpmEDT() error {
	if m.edtObjs == nil {
		return nil
	}
	iter := m.edtObjs.ProfileLpm.Iterate()
	var kbuf, vbuf []byte
	for iter.Next(&kbuf, &vbuf) {
		_ = m.edtObjs.ProfileLpm.Delete(kbuf)
	}
	return iter.Err()
}
