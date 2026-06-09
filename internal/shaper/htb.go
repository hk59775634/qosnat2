package shaper

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"github.com/hk59775634/qosnat2/internal/ebpf"
	"github.com/hk59775634/qosnat2/internal/route"
)

type hostState struct {
	minor uint32
	down  uint64
	up    uint64
}

// hostEntry 按网卡分别记录 egress HTB；ifb 上行每 IP 共享一份。
type hostEntry struct {
	egress map[string]hostState
	ifb    hostState
	ifbSet bool
}

// HostShaper 每 /32 动态 HTB 类（LAN egress 下行 + ifb 上行；extraDev 如 wg0 用于 VPN 隧道）
type HostShaper struct {
	mu       sync.Mutex
	leaf     string
	lan      string
	extraDev string
	known    map[string]*hostEntry // ip -> 各网卡已安装状态
}

func NewHostShaper(devLAN, leaf string) *HostShaper {
	if leaf == "" {
		leaf = "fq_codel"
	}
	return &HostShaper{
		leaf:  leaf,
		lan:   devLAN,
		known: map[string]*hostEntry{},
	}
}

// ResetKnown 重建 HTB 根后清空内存中的 ip→minor（避免以为 u32 仍存在）
func (h *HostShaper) ResetKnown() {
	h.mu.Lock()
	h.known = map[string]*hostEntry{}
	h.mu.Unlock()
}

func (h *HostShaper) getEntryLocked(ip string) *hostEntry {
	if h.known == nil {
		h.known = map[string]*hostEntry{}
	}
	e, ok := h.known[ip]
	if !ok {
		e = &hostEntry{egress: map[string]hostState{}}
		h.known[ip] = e
	}
	if e.egress == nil {
		e.egress = map[string]hostState{}
	}
	return e
}

func egressMatches(st hostState, minor uint32, downBPS uint64) bool {
	return st.minor == minor && st.down == downBPS
}

func ifbMatches(st hostState, minor uint32, upBPS uint64) bool {
	return st.minor == minor && st.up == upBPS
}

// HostConfiguredOnDevice 报告 ip 在指定 egress 网卡 + ifb 上行是否已按速率安装
func (h *HostShaper) HostConfiguredOnDevice(ip, dev string, minor uint32, down, up uint64) bool {
	if dev == "" {
		dev = h.lan
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	e := h.known[ip]
	if e == nil {
		return false
	}
	if dev == IFBDev {
		return e.ifbSet && ifbMatches(e.ifb, minor, up)
	}
	st, ok := e.egress[dev]
	if !ok || !egressMatches(st, minor, down) {
		return false
	}
	return e.ifbSet && ifbMatches(e.ifb, minor, up)
}

func (h *HostShaper) hostFullyConfigured(ip string, minor uint32, down, up uint64, extraDev string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	e := h.known[ip]
	if e == nil || !e.ifbSet || !ifbMatches(e.ifb, minor, up) {
		return false
	}
	st, ok := e.egress[h.lan]
	if !ok || !egressMatches(st, minor, down) {
		return false
	}
	if extraDev != "" {
		st, ok := e.egress[extraDev]
		if !ok || !egressMatches(st, minor, down) {
			return false
		}
	}
	return true
}

func (h *HostShaper) markDevice(ip, dev string, minor uint32, downBPS, upBPS uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	e := h.getEntryLocked(ip)
	if dev != IFBDev {
		e.egress[dev] = hostState{minor: minor, down: downBPS}
		e.ifb = hostState{minor: minor, up: upBPS}
		e.ifbSet = true
	} else {
		e.ifb = hostState{minor: minor, up: upBPS}
		e.ifbSet = true
	}
}

// HostConfigured 兼容旧调用：默认 LAN 设备
func (h *HostShaper) HostConfigured(ip string, minor uint32, down, up uint64) bool {
	return h.HostConfiguredOnDevice(ip, h.lan, minor, down, up)
}

// SetExtraDev 设置附加整形接口（WireGuard wg0 等）
func (h *HostShaper) SetExtraDev(dev string) {
	h.mu.Lock()
	h.extraDev = dev
	h.mu.Unlock()
}

// MinorForIP 与 BPF class_minor_for 一致
func MinorForIP(ip string) (uint32, error) {
	k, err := ebpf.IPToHostKey(ip)
	if err != nil {
		return 0, err
	}
	m := 0x100 | (k & 0xffff)
	if m == 1 {
		m++
	}
	return m, nil
}

func bpsToTC(bps uint64) string {
	if bps == 0 {
		return "1mbit"
	}
	bits := bps * 8
	if bits >= 1_000_000_000 {
		return fmt.Sprintf("%dGbit", bits/1_000_000_000)
	}
	if bits >= 1_000_000 {
		return fmt.Sprintf("%dMbit", bits/1_000_000)
	}
	if bits >= 1_000 {
		return fmt.Sprintf("%dKbit", bits/1_000)
	}
	return fmt.Sprintf("%dbit", bits)
}

// EnsureHostOnDevice 在指定网卡 egress + ifb 上建 HTB 类（单设备策略）
func (h *HostShaper) EnsureHostOnDevice(ip string, downBPS, upBPS uint64, minor uint32, dev string) error {
	if dev == "" {
		dev = h.lan
	}
	if minor == 0 {
		var err error
		minor, err = MinorForIP(ip)
		if err != nil {
			return err
		}
	}
	if h.HostConfiguredOnDevice(ip, dev, minor, downBPS, upBPS) {
		return nil
	}

	down := bpsToTC(downBPS)
	up := bpsToTC(upBPS)
	cid := fmt.Sprintf("1:%x", minor)

	if err := h.ensureClass(dev, cid, down, down); err != nil {
		return fmt.Errorf("%s %s: %w", dev, ip, err)
	}
	if dev != IFBDev {
		if err := h.ensureClass(IFBDev, cid, up, up); err != nil {
			return fmt.Errorf("ifb %s: %w", ip, err)
		}
		if err := installIFBUploadFilter(ip, minor); err != nil {
			return fmt.Errorf("ifb u32 %s: %w", ip, err)
		}
	}
	h.markDevice(ip, dev, minor, downBPS, upBPS)
	return nil
}

// DeleteHostOnDevice 删除指定网卡上的 HTB 类（及 ifb 上行类）
func (h *HostShaper) DeleteHostOnDevice(ip string, dev string) error {
	if dev == "" {
		dev = h.lan
	}
	h.mu.Lock()
	e := h.known[ip]
	var minor uint32
	if e != nil {
		if st, ok := e.egress[dev]; ok {
			minor = st.minor
		} else if e.ifbSet {
			minor = e.ifb.minor
		}
		delete(e.egress, dev)
		if dev != IFBDev {
			e.ifbSet = false
		}
		if len(e.egress) == 0 && !e.ifbSet {
			delete(h.known, ip)
		}
	}
	h.mu.Unlock()

	if minor == 0 {
		var err error
		minor, err = MinorForIP(ip)
		if err != nil {
			return err
		}
	}

	cid := fmt.Sprintf("1:%x", minor)
	_ = removeIFBUploadFilter(ip)
	_ = h.delClass(dev, cid)
	if dev != IFBDev {
		_ = h.delClass(IFBDev, cid)
	}
	return nil
}

func (h *HostShaper) EnsureHost(ip string, downBPS, upBPS uint64, minor uint32) error {
	if minor == 0 {
		var err error
		minor, err = MinorForIP(ip)
		if err != nil {
			return err
		}
	}
	extra := ""
	if h.extraDev != "" && route.LinkExists(h.extraDev) {
		extra = h.extraDev
	}
	if h.hostFullyConfigured(ip, minor, downBPS, upBPS, extra) {
		return nil
	}

	down := bpsToTC(downBPS)
	up := bpsToTC(upBPS)
	cid := fmt.Sprintf("1:%x", minor)

	if !h.HostConfiguredOnDevice(ip, h.lan, minor, downBPS, upBPS) {
		if err := h.ensureClass(h.lan, cid, down, down); err != nil {
			return fmt.Errorf("lan %s: %w", ip, err)
		}
	}
	if extra != "" && !h.HostConfiguredOnDevice(ip, extra, minor, downBPS, upBPS) {
		if err := h.ensureClass(extra, cid, down, down); err != nil {
			return fmt.Errorf("%s %s: %w", extra, ip, err)
		}
	}
	h.mu.Lock()
	e := h.known[ip]
	needIFB := e == nil || !e.ifbSet || !ifbMatches(e.ifb, minor, upBPS)
	h.mu.Unlock()
	if needIFB {
		if err := h.ensureClass(IFBDev, cid, up, up); err != nil {
			return fmt.Errorf("ifb %s: %w", ip, err)
		}
		if err := installIFBUploadFilter(ip, minor); err != nil {
			return fmt.Errorf("ifb u32 %s: %w", ip, err)
		}
	}

	h.mu.Lock()
	e = h.getEntryLocked(ip)
	e.egress[h.lan] = hostState{minor: minor, down: downBPS}
	if extra != "" {
		e.egress[extra] = hostState{minor: minor, down: downBPS}
	}
	e.ifb = hostState{minor: minor, up: upBPS}
	e.ifbSet = true
	h.mu.Unlock()
	return nil
}

func (h *HostShaper) DeleteHost(ip string) error {
	h.mu.Lock()
	e := h.known[ip]
	var devs []string
	var minor uint32
	if e != nil {
		for d := range e.egress {
			devs = append(devs, d)
		}
		if e.ifbSet {
			minor = e.ifb.minor
		} else if len(devs) > 0 {
			minor = e.egress[devs[0]].minor
		}
		delete(h.known, ip)
	}
	extra := h.extraDev
	h.mu.Unlock()

	if minor == 0 {
		var err error
		minor, err = MinorForIP(ip)
		if err != nil {
			return err
		}
	}

	cid := fmt.Sprintf("1:%x", minor)
	_ = removeIFBUploadFilter(ip)
	_ = h.delClass(h.lan, cid)
	for _, d := range devs {
		_ = h.delClass(d, cid)
	}
	if extra != "" {
		_ = h.delClass(extra, cid)
	}
	_ = h.delClass(IFBDev, cid)
	return nil
}

func (h *HostShaper) ensureClass(dev, classid, rate, ceil string) error {
	parent := "1:"
	args := []string{
		"tc", "class", "add", "dev", dev, "parent", parent,
		"classid", classid, "htb", "rate", rate, "ceil", ceil,
	}
	if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err == nil {
		return h.ensureLeaf(dev, classid)
	} else {
		msg := string(out)
		if strings.Contains(msg, "File exists") {
			chg := []string{"tc", "class", "change", "dev", dev, "parent", parent,
				"classid", classid, "htb", "rate", rate, "ceil", ceil}
			if out2, err2 := exec.Command(chg[0], chg[1:]...).CombinedOutput(); err2 != nil {
				return fmt.Errorf("%s %w", strings.TrimSpace(string(out2)), err2)
			}
			return nil
		}
		return fmt.Errorf("%s %w", strings.TrimSpace(msg), err)
	}
}

func (h *HostShaper) ensureLeaf(dev, classid string) error {
	_ = exec.Command("tc", "qdisc", "del", "dev", dev, "parent", classid).Run()
	args := append([]string{"tc", "qdisc", "add", "dev", dev, "parent", classid}, LeafTCArgs(h.leaf, FQOpts{})...)
	if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
		msg := string(out)
		if strings.Contains(msg, "File exists") {
			return nil
		}
		return fmt.Errorf("tc leaf %s parent %s: %s %w", dev, classid, strings.TrimSpace(msg), err)
	}
	return nil
}

func (h *HostShaper) delClass(dev, classid string) error {
	_ = exec.Command("tc", "qdisc", "del", "dev", dev, "parent", classid).Run()
	_ = exec.Command("tc", "class", "del", "dev", dev, "classid", classid).Run()
	return nil
}

// ListClasses 调试用
func (h *HostShaper) ListClasses(dev string) (string, error) {
	out, err := exec.Command("tc", "-s", "class", "show", "dev", dev).CombinedOutput()
	return string(out), err
}

// ParseClassID 从 tc 输出解析 minor（备用）
func ParseClassID(classid string) (uint32, error) {
	parts := strings.Split(classid, ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("bad classid")
	}
	v, err := strconv.ParseUint(parts[1], 16, 32)
	return uint32(v), err
}
