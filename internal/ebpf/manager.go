package ebpf

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/cilium/ebpf"
	"github.com/hk59775634/qosnat2/internal/netif"
	"github.com/hk59775634/qosnat2/internal/store"
)

const (
	PinDir      = "/sys/fs/bpf/qosnat2"
	defaultObj  = "/usr/lib/qosnat2/classify.bpf.o"
	mapProfile  = "profile_lpm"
	mapHost     = "host_exact"
	mapActive   = "active_host"
	mapClassID  = "classid_map"
)

type bpfObjects struct {
	ProfileLpm *ebpf.Map `ebpf:"profile_lpm"`
	HostExact  *ebpf.Map `ebpf:"host_exact"`
	ActiveHost *ebpf.Map `ebpf:"active_host"`
	ClassidMap *ebpf.Map `ebpf:"classid_map"`
	Events     *ebpf.Map `ebpf:"events"`
	Ingress    *ebpf.Program `ebpf:"classify_ingress"`
	Egress     *ebpf.Program `ebpf:"classify_egress"`
}

// Manager cilium/ebpf 生命周期与 Map CRUD + TC attach（P2）
type Manager struct {
	mu          sync.RWMutex
	objs        *bpfObjects
	objPath     string
	attachedDev string
	attached    map[string]struct{}
	loaded      bool
}

func New() *Manager {
	p := os.Getenv("BPF_OBJ")
	if p == "" {
		p = defaultObj
	}
	return &Manager{objPath: p, attached: map[string]struct{}{}}
}

func (m *Manager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.loaded {
		return nil
	}
	if _, err := os.Stat(m.objPath); err != nil {
		return fmt.Errorf("bpf object %s: %w (run: make bpf && deploy)", m.objPath, err)
	}
	if err := netif.EnsureIFB(); err != nil {
		return err
	}
	spec, err := ebpf.LoadCollectionSpec(m.objPath)
	if err != nil {
		return err
	}
	if err := m.rewriteIFBIndex(spec); err != nil {
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
	if err := m.pinAll(&objs); err != nil {
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

// loadPinnedMapReplacements 复用已 pin 的 map，使 TC 程序与 Go 更新同一套 map
func loadPinnedMapReplacements(spec *ebpf.CollectionSpec) map[string]*ebpf.Map {
	reps := make(map[string]*ebpf.Map)
	for name := range spec.Maps {
		path := filepath.Join(PinDir, name)
		pm, err := ebpf.LoadPinnedMap(path, nil)
		if err != nil {
			continue
		}
		reps[name] = pm
	}
	return reps
}

func (m *Manager) pinAll(objs *bpfObjects) error {
	if err := os.MkdirAll(PinDir, 0755); err != nil {
		return err
	}
	pins := []struct {
		name string
		pin  func(string) error
	}{
		{mapProfile, objs.ProfileLpm.Pin},
		{mapHost, objs.HostExact.Pin},
		{mapActive, objs.ActiveHost.Pin},
		{mapClassID, objs.ClassidMap.Pin},
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
	if o.ActiveHost != nil {
		o.ActiveHost.Close()
	}
	if o.ClassidMap != nil {
		o.ClassidMap.Close()
	}
	if o.Events != nil {
		o.Events.Close()
	}
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
		"pin_dir":  PinDir,
		"phase":    "P5",
		"attached":     m.attachedDev,
		"attached_devs": m.attachedList(),
		"obj":      m.objPath,
		"loaded":   m.loaded,
		"maps":     []string{mapProfile, mapHost, mapActive, mapClassID},
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

// ReplayState 启动时把 state 写入 Map
func (m *Manager) ReplayState(st store.State) error {
	if err := m.Load(); err != nil {
		return err
	}
	if err := m.flushProfileLpm(); err != nil {
		return err
	}
	// 默认 profile
	if st.Shaper.PolicyCIDR != "" {
		rv, err := rateFromProfile(st.Shaper.DefaultProfile.Down, st.Shaper.DefaultProfile.Up)
		if err != nil {
			return err
		}
		if err := m.UpdateProfile(st.Shaper.PolicyCIDR, rv); err != nil {
			return err
		}
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
	if !m.loaded {
		return errors.New("ebpf not loaded")
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
	if !m.loaded {
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
	if !m.loaded {
		return errors.New("ebpf not loaded")
	}
	if rv.ClassMinor == 0 {
		if minor, err := classMinorForIP(ip); err == nil {
			rv.ClassMinor = minor
		}
	}
	k, err := IPToHostKey(ip)
	if err != nil {
		return err
	}
	key := make([]byte, 4)
	binary.BigEndian.PutUint32(key, k)
	return m.objs.HostExact.Put(key, rv.Marshal())
}

func classMinorForIP(ip string) (uint32, error) {
	k, err := IPToHostKey(ip)
	if err != nil {
		return 0, err
	}
	m := 0x100 | (k & 0xffff)
	if m == 1 {
		m++
	}
	return m, nil
}

func (m *Manager) DeleteHost(ip string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.loaded {
		return errors.New("ebpf not loaded")
	}
	k, err := IPToHostKey(ip)
	if err != nil {
		return err
	}
	key := make([]byte, 4)
	binary.BigEndian.PutUint32(key, k)
	_ = m.objs.ClassidMap.Delete(key)
	_ = m.objs.ActiveHost.Delete(key)
	return m.objs.HostExact.Delete(key)
}

// ActiveEntry 活跃主机（Iterate active_host）
type ActiveEntry struct {
	IP         string `json:"ip"`
	DownBPS    uint64 `json:"down_bps"`
	UpBPS      uint64 `json:"up_bps"`
	ClassMinor uint32 `json:"class_minor"`
	BytesDown  uint64 `json:"bytes_down"`
	BytesUp    uint64 `json:"bytes_up"`
	LastSeenNS uint64 `json:"last_seen_ns"`
}

func (m *Manager) ListActive() ([]ActiveEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.loaded {
		return nil, errors.New("ebpf not loaded")
	}
	var out []ActiveEntry
	iter := m.objs.ActiveHost.Iterate()
	var kbuf, vbuf []byte
	for iter.Next(&kbuf, &vbuf) {
		if len(kbuf) < 4 || len(vbuf) < 32 {
			continue
		}
		ipBE := binary.BigEndian.Uint32(kbuf[0:4])
		act := struct {
			bytesDown, bytesUp, lastSeen uint64
			classMinor, flags            uint32
		}{}
		act.bytesDown = binary.LittleEndian.Uint64(vbuf[0:8])
		act.bytesUp = binary.LittleEndian.Uint64(vbuf[8:16])
		act.lastSeen = binary.LittleEndian.Uint64(vbuf[16:24])
		act.classMinor = binary.LittleEndian.Uint32(vbuf[24:28])
		ip := HostKeyToIP(ipBE)
		down, up, _ := m.lookupRatesLocked(ipBE)
		out = append(out, ActiveEntry{
			IP: ip, DownBPS: down, UpBPS: up,
			ClassMinor: act.classMinor, BytesDown: act.bytesDown,
			BytesUp: act.bytesUp, LastSeenNS: act.lastSeen,
		})
	}
	return out, iter.Err()
}


// ProfileEntry API 列表项
type ProfileEntry struct {
	CIDR     string `json:"cidr"`
	DownBPS  uint64 `json:"down_bps"`
	UpBPS    uint64 `json:"up_bps"`
}

// HostEntry API 列表项
type HostEntry struct {
	IP       string `json:"ip"`
	DownBPS  uint64 `json:"down_bps"`
	UpBPS    uint64 `json:"up_bps"`
}

func (m *Manager) ListProfiles() ([]ProfileEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.loaded {
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
	if !m.loaded {
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

// PurgeActive 仅删除 active_host/classid_map（保留 host_exact 配置）
func (m *Manager) PurgeActive(ip string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.loaded {
		return errors.New("ebpf not loaded")
	}
	k, err := IPToHostKey(ip)
	if err != nil {
		return err
	}
	key := make([]byte, 4)
	binary.BigEndian.PutUint32(key, k)
	_ = m.objs.ClassidMap.Delete(key)
	return m.objs.ActiveHost.Delete(key)
}

// LookupRates 查 host_exact 再 profile_lpm（最长前缀，与 BPF trie 一致）
func (m *Manager) LookupRates(ip string) (down, up uint64, ok bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.loaded {
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

// EachClassid 遍历 classid_map 中已分类主机
func (m *Manager) EachClassid(fn func(ip string, minor uint32) error) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.loaded {
		return errors.New("ebpf not loaded")
	}
	iter := m.objs.ClassidMap.Iterate()
	var kbuf, vbuf []byte
	for iter.Next(&kbuf, &vbuf) {
		if len(kbuf) < 4 {
			continue
		}
		ipBE := binary.BigEndian.Uint32(kbuf[0:4])
		minor := uint32(0)
		if len(vbuf) >= 4 {
			minor = binary.LittleEndian.Uint32(vbuf[0:4])
		}
		if err := fn(HostKeyToIP(ipBE), minor); err != nil {
			return err
		}
	}
	return iter.Err()
}

// DeleteHostExact 兼容旧名
func (m *Manager) DeleteHostExact(ip string) error { return m.DeleteHost(ip) }

// UpdateHostExact 兼容旧名
func (m *Manager) UpdateHostExact(ip string, downBPS, upBPS uint64) error {
	return m.UpdateHost(ip, RateVal{DownBPS: downBPS, UpBPS: upBPS})
}
