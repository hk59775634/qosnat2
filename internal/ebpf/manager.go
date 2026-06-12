package ebpf

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/cilium/ebpf"
	"github.com/hk59775634/qosnat2/internal/store"
	"github.com/hk59775634/qosnat2/internal/webassets"
)

const (
	PinDir            = "/sys/fs/bpf/qosnat2"
	defaultObj        = "/usr/lib/qosnat2/rate_edt.bpf.o"
	mapProfile        = "profile_lpm"
	mapHost           = "host_exact"
	mapThrottle       = "throttle"
	mapTokenBucket    = "token_bucket"
	progIngress       = "rate_limit_ingress"
	progEgress        = "rate_limit_egress"
)

type bpfObjects struct {
	ProfileLpm  *ebpf.Map     `ebpf:"profile_lpm"`
	HostExact   *ebpf.Map     `ebpf:"host_exact"`
	Throttle    *ebpf.Map     `ebpf:"throttle"`
	TokenBucket *ebpf.Map     `ebpf:"token_bucket"`
	Ingress     *ebpf.Program `ebpf:"rate_limit_ingress"`
	Egress      *ebpf.Program `ebpf:"rate_limit_egress"`
}

func (o *bpfObjects) close() {
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

// Manager EDT BPF 生命周期、map CRUD 与 TC attach。
type Manager struct {
	mu          sync.RWMutex
	objs        *bpfObjects
	objPath     string
	attachedDev string
	attached    map[string]struct{}
	loaded      bool
}

func New() *Manager {
	p := os.Getenv("BPF_EDT_OBJ")
	if p == "" {
		if p2 := os.Getenv("BPF_OBJ"); p2 != "" {
			p = p2
		} else {
			p = defaultObj
		}
	}
	return &Manager{objPath: p, attached: map[string]struct{}{}}
}

func (m *Manager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.loaded {
		return nil
	}
	spec, err := m.loadCollectionSpec()
	if err != nil {
		return err
	}
	opts := &ebpf.CollectionOptions{MapReplacements: loadPinnedMapReplacements(spec)}
	var objs bpfObjects
	if err := spec.LoadAndAssign(&objs, opts); err != nil {
		return err
	}
	for _, pm := range opts.MapReplacements {
		pm.Close()
	}
	if err := m.pinMaps(&objs); err != nil {
		objs.close()
		return err
	}
	if err := m.pinPrograms(&objs); err != nil {
		objs.close()
		return err
	}
	m.objs = &objs
	m.loaded = true
	return nil
}

func (m *Manager) loadCollectionSpec() (*ebpf.CollectionSpec, error) {
	if p := os.Getenv("BPF_EDT_OBJ"); p != "" {
		return ebpf.LoadCollectionSpec(p)
	}
	if webassets.Enabled() && len(webassets.BPFEDT) > 0 {
		return ebpf.LoadCollectionSpecFromReader(bytes.NewReader(webassets.BPFEDT))
	}
	if _, err := os.Stat(m.objPath); err != nil {
		return nil, fmt.Errorf("bpf object %s: %w (run: make bpf -C bpf)", m.objPath, err)
	}
	return ebpf.LoadCollectionSpec(m.objPath)
}

func loadPinnedMapReplacements(spec *ebpf.CollectionSpec) map[string]*ebpf.Map {
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

func (m *Manager) pinMaps(objs *bpfObjects) error {
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

func (m *Manager) pinPrograms(objs *bpfObjects) error {
	progs := []struct {
		name string
		prog *ebpf.Program
	}{
		{progIngress, objs.Ingress},
		{progEgress, objs.Egress},
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

func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	_ = m.detachLocked()
	if m.objs != nil {
		m.objs.close()
		m.objs = nil
	}
	m.loaded = false
	return nil
}

func (m *Manager) attachedList() []string {
	out := make([]string, 0, len(m.attached))
	for d := range m.attached {
		out = append(out, d)
	}
	return out
}

func (m *Manager) Ready() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.loaded
}

func (m *Manager) Status() map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()
	st := map[string]any{
		"pin_dir":       PinDir,
		"phase":         "edt",
		"attached":      m.attachedDev,
		"attached_devs": m.attachedList(),
		"obj":           m.objPath,
		"loaded":        m.loaded,
		"maps":          []string{mapProfile, mapHost, mapThrottle, mapTokenBucket},
	}
	if m.loaded && m.objs != nil {
		st["profile_lpm_entries"] = m.objs.ProfileLpm.MaxEntries()
		st["host_exact_entries"] = m.objs.HostExact.MaxEntries()
	}
	return st
}

func rateFromProfile(down, up string) (RateVal, error) {
	d, err := store.MbitToBPS(down)
	if err != nil {
		return RateVal{}, err
	}
	u, err := store.MbitToBPS(up)
	if err != nil {
		return RateVal{}, err
	}
	return RateVal{DownBPS: d, UpBPS: u}, nil
}

func flushMapIter(del func(interface{}) error, iter *ebpf.MapIterator) error {
	var kbuf, vbuf []byte
	for iter.Next(&kbuf, &vbuf) {
		key := append([]byte(nil), kbuf...)
		_ = del(key)
	}
	return iter.Err()
}

func (m *Manager) flushProfileLpm() error {
	if !m.loaded || m.objs == nil {
		return nil
	}
	iter := m.objs.ProfileLpm.Iterate()
	var kbuf, vbuf []byte
	for iter.Next(&kbuf, &vbuf) {
		_ = m.objs.ProfileLpm.Delete(kbuf)
	}
	return iter.Err()
}

// FlushRuntimeMaps 清空运行期限速 map（关闭 QoS 时调用）。
func (m *Manager) FlushRuntimeMaps() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.loaded || m.objs == nil {
		return nil
	}
	if err := m.flushProfileLpm(); err != nil {
		return err
	}
	if err := flushMapIter(m.objs.HostExact.Delete, m.objs.HostExact.Iterate()); err != nil {
		return err
	}
	if err := flushMapIter(m.objs.Throttle.Delete, m.objs.Throttle.Iterate()); err != nil {
		return err
	}
	if err := flushMapIter(m.objs.TokenBucket.Delete, m.objs.TokenBucket.Iterate()); err != nil {
		return err
	}
	return nil
}

// ReplayState 启动时把 state 写入 map。
func (m *Manager) ReplayState(st store.State) error {
	if err := m.Load(); err != nil {
		return err
	}
	if err := m.flushProfileLpm(); err != nil {
		return err
	}
	for _, p := range store.SortProfilesByID(st.Shaper.Profiles) {
		rv, err := rateFromProfile(p.Down, p.Up)
		if err != nil {
			return err
		}
		if err := m.UpdateProfile(p.CIDR, rv); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) UpdateProfile(cidr string, rv RateVal) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.loaded || m.objs == nil {
		return errors.New("ebpf not loaded")
	}
	v4, _, err := profileMapForCIDR(cidr)
	if err != nil {
		return err
	}
	if !v4 {
		return fmt.Errorf("ipv6 profile not supported in edt mode")
	}
	k, err := IPToLPMKey(cidr)
	if err != nil {
		return err
	}
	return m.objs.ProfileLpm.Put(k.Marshal(), rv.Marshal())
}

func (m *Manager) DeleteProfile(cidr string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.loaded || m.objs == nil {
		return errors.New("ebpf not loaded")
	}
	k, err := IPToLPMKey(cidr)
	if err != nil {
		return err
	}
	return m.objs.ProfileLpm.Delete(k.Marshal())
}

func (m *Manager) UpdateHost(ip string, rv RateVal) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.loaded || m.objs == nil {
		return errors.New("ebpf not loaded")
	}
	k, err := IPToHostKey(ip)
	if err != nil {
		return err
	}
	key := make([]byte, 4)
	binary.BigEndian.PutUint32(key, k)
	return m.objs.HostExact.Put(key, rv.Marshal())
}

func (m *Manager) DeleteHost(ip string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.loaded || m.objs == nil {
		return errors.New("ebpf not loaded")
	}
	k, err := IPToHostKey(ip)
	if err != nil {
		return err
	}
	key := make([]byte, 4)
	binary.BigEndian.PutUint32(key, k)
	return m.objs.HostExact.Delete(key)
}

// ProfileEntry API 列表项
type ProfileEntry struct {
	CIDR    string `json:"cidr"`
	DownBPS uint64 `json:"down_bps"`
	UpBPS   uint64 `json:"up_bps"`
}

// HostEntry API 列表项
type HostEntry struct {
	IP      string `json:"ip"`
	DownBPS uint64 `json:"down_bps"`
	UpBPS   uint64 `json:"up_bps"`
}

func (m *Manager) ListProfiles() ([]ProfileEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.loaded || m.objs == nil {
		return nil, errors.New("ebpf not loaded")
	}
	var out []ProfileEntry
	iter := m.objs.ProfileLpm.Iterate()
	var kbuf, vbuf []byte
	for iter.Next(&kbuf, &vbuf) {
		if len(kbuf) < 8 || len(vbuf) < 16 {
			continue
		}
		prefix := binary.LittleEndian.Uint32(kbuf[0:4])
		addr := binary.BigEndian.Uint32(kbuf[4:8])
		down := binary.LittleEndian.Uint64(vbuf[0:8])
		up := binary.LittleEndian.Uint64(vbuf[8:16])
		ip := make(net.IP, 4)
		binary.BigEndian.PutUint32(ip, addr)
		mask := net.CIDRMask(int(prefix), 32)
		cidr := (&net.IPNet{IP: ip.Mask(mask), Mask: mask}).String()
		out = append(out, ProfileEntry{CIDR: cidr, DownBPS: down, UpBPS: up})
	}
	return out, iter.Err()
}

func (m *Manager) ListHosts() ([]HostEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.loaded || m.objs == nil {
		return nil, errors.New("ebpf not loaded")
	}
	var out []HostEntry
	iter := m.objs.HostExact.Iterate()
	var kbuf, vbuf []byte
	for iter.Next(&kbuf, &vbuf) {
		if len(kbuf) < 4 || len(vbuf) < 16 {
			continue
		}
		key := binary.BigEndian.Uint32(kbuf[0:4])
		down := binary.LittleEndian.Uint64(vbuf[0:8])
		up := binary.LittleEndian.Uint64(vbuf[8:16])
		out = append(out, HostEntry{IP: HostKeyToIP(key), DownBPS: down, UpBPS: up})
	}
	return out, iter.Err()
}

// LookupRates 查 host_exact 再 profile_lpm（最长前缀）。
func (m *Manager) LookupRates(ip string) (down, up uint64, ok bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.loaded || m.objs == nil {
		return 0, 0, false
	}
	k, err := IPToHostKey(ip)
	if err != nil {
		return 0, 0, false
	}
	return m.lookupRatesLocked(k)
}

func ipv4PrefixAddr(addr uint32, prefix int) uint32 {
	if prefix <= 0 {
		return 0
	}
	mask := ^uint32(0) << (32 - prefix)
	return addr & mask
}

func (m *Manager) lookupRatesLocked(hostKey uint32) (down, up uint64, ok bool) {
	key := make([]byte, 4)
	binary.BigEndian.PutUint32(key, hostKey)
	var rv []byte
	if err := m.objs.HostExact.Lookup(key, &rv); err == nil && len(rv) >= 16 {
		return binary.LittleEndian.Uint64(rv[0:8]), binary.LittleEndian.Uint64(rv[8:16]), true
	}
	for prefix := 32; prefix >= 0; prefix-- {
		lpmKey := LPMKey{Prefixlen: uint32(prefix), Addr: ipv4PrefixAddr(hostKey, prefix)}.Marshal()
		if err := m.objs.ProfileLpm.Lookup(lpmKey, &rv); err == nil && len(rv) >= 16 {
			return binary.LittleEndian.Uint64(rv[0:8]), binary.LittleEndian.Uint64(rv[8:16]), true
		}
	}
	return 0, 0, false
}
