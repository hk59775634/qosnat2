package store

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
)

// LVSState Linux Virtual Server（IPVS）配置。
type LVSState struct {
	Enabled        bool               `json:"enabled"`
	Mode           string             `json:"mode,omitempty"` // nat | dr
	VirtualServers []LVSVirtualServer `json:"virtual_servers,omitempty"`
}

// LVSVirtualServer 虚拟服务 VIP:port → 多台 Real Server。
type LVSVirtualServer struct {
	ID             string          `json:"id"`
	VIP            string          `json:"vip"`
	Port           int             `json:"port"`
	Protocol       string          `json:"protocol"` // tcp | udp
	Scheduler      string          `json:"scheduler,omitempty"`
	PersistenceSec int             `json:"persistence_sec,omitempty"`
	AutoVIP        bool            `json:"auto_vip,omitempty"`
	WANDevice      string          `json:"wan_device,omitempty"`
	RealServers    []LVSRealServer `json:"real_servers"`
	Comment        string          `json:"comment,omitempty"`
}

// LVSRealServer 后端节点。
type LVSRealServer struct {
	IP     string `json:"ip"`
	Port   int    `json:"port,omitempty"`
	Weight int    `json:"weight,omitempty"`
}

func DefaultLVS() LVSState {
	return LVSState{
		Mode:           "nat",
		VirtualServers: []LVSVirtualServer{},
	}
}

var lvsSchedulers = map[string]struct{}{
	"rr": {}, "wrr": {}, "lc": {}, "wlc": {}, "lblc": {}, "lblcr": {},
	"dh": {}, "sh": {}, "sed": {}, "nq": {},
}

// NormalizeLVS 校验并填充默认值。
func NormalizeLVS(l *LVSState, defaultWAN string) error {
	if l == nil {
		return fmt.Errorf("lvs config nil")
	}
	mode := strings.ToLower(strings.TrimSpace(l.Mode))
	if mode == "" {
		mode = "nat"
	}
	if mode != "nat" && mode != "dr" {
		return fmt.Errorf("lvs mode must be nat or dr")
	}
	l.Mode = mode
	if l.VirtualServers == nil {
		l.VirtualServers = []LVSVirtualServer{}
	}
	seen := map[string]struct{}{}
	for i := range l.VirtualServers {
		if err := normalizeLVSVirtualServer(&l.VirtualServers[i], defaultWAN); err != nil {
			return fmt.Errorf("virtual_servers[%d]: %w", i, err)
		}
		key := lvsVSKey(l.VirtualServers[i])
		if _, dup := seen[key]; dup {
			return fmt.Errorf("duplicate virtual server %s", key)
		}
		seen[key] = struct{}{}
	}
	if l.Enabled && len(l.VirtualServers) == 0 {
		return fmt.Errorf("virtual_servers required when lvs enabled")
	}
	return nil
}

func normalizeLVSVirtualServer(v *LVSVirtualServer, defaultWAN string) error {
	if v.ID == "" {
		v.ID = NewLVSID()
	}
	vip := strings.TrimSpace(v.VIP)
	if vip == "" {
		return fmt.Errorf("vip required")
	}
	parsed := net.ParseIP(vip)
	if parsed == nil || parsed.To4() == nil {
		return fmt.Errorf("vip must be ipv4")
	}
	v.VIP = parsed.String()
	if v.Port <= 0 || v.Port > 65535 {
		return fmt.Errorf("port required (1-65535)")
	}
	proto := strings.ToLower(strings.TrimSpace(v.Protocol))
	switch proto {
	case "", "tcp":
		v.Protocol = "tcp"
	case "udp":
		v.Protocol = "udp"
	default:
		return fmt.Errorf("protocol must be tcp or udp")
	}
	sched := strings.ToLower(strings.TrimSpace(v.Scheduler))
	if sched == "" {
		sched = "rr"
	}
	if _, ok := lvsSchedulers[sched]; !ok {
		return fmt.Errorf("unsupported scheduler: %s", sched)
	}
	v.Scheduler = sched
	if v.PersistenceSec < 0 {
		return fmt.Errorf("persistence_sec invalid")
	}
	dev := strings.TrimSpace(v.WANDevice)
	if dev == "" {
		dev = strings.TrimSpace(defaultWAN)
	}
	v.WANDevice = dev
	if len(v.RealServers) == 0 {
		return fmt.Errorf("real_servers required")
	}
	for j := range v.RealServers {
		if err := normalizeLVSRealServer(&v.RealServers[j], v.Port); err != nil {
			return fmt.Errorf("real_servers[%d]: %w", j, err)
		}
	}
	v.Comment = strings.TrimSpace(v.Comment)
	return nil
}

func normalizeLVSRealServer(r *LVSRealServer, vsPort int) error {
	ip := strings.TrimSpace(r.IP)
	if ip == "" {
		return fmt.Errorf("ip required")
	}
	parsed := net.ParseIP(ip)
	if parsed == nil || parsed.To4() == nil {
		return fmt.Errorf("ip must be ipv4")
	}
	r.IP = parsed.String()
	if r.Port <= 0 {
		r.Port = vsPort
	}
	if r.Port <= 0 || r.Port > 65535 {
		return fmt.Errorf("port invalid")
	}
	if r.Weight <= 0 {
		r.Weight = 1
	}
	if r.Weight > 65535 {
		return fmt.Errorf("weight out of range")
	}
	return nil
}

func lvsVSKey(v LVSVirtualServer) string {
	return v.Protocol + "://" + v.VIP + ":" + fmt.Sprintf("%d", v.Port)
}

// NewLVSID 生成虚拟服务 ID。
func NewLVSID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return "lvs-" + hex.EncodeToString(b[:])
}

// MigrateLVS 启动时补 id/默认值。
func MigrateLVS(l *LVSState) {
	if l == nil {
		return
	}
	if l.Mode == "" {
		l.Mode = "nat"
	}
	if l.VirtualServers == nil {
		l.VirtualServers = []LVSVirtualServer{}
	}
	for i := range l.VirtualServers {
		v := &l.VirtualServers[i]
		if v.ID == "" {
			v.ID = NewLVSID()
		}
		if v.Protocol == "" {
			v.Protocol = "tcp"
		}
		if v.Scheduler == "" {
			v.Scheduler = "rr"
		}
		for j := range v.RealServers {
			if v.RealServers[j].Weight <= 0 {
				v.RealServers[j].Weight = 1
			}
			if v.RealServers[j].Port <= 0 {
				v.RealServers[j].Port = v.Port
			}
		}
	}
}

// LVSVSConflictsForward 检查与 WAN 端口转发是否冲突（同 VIP:port）。
func LVSVSConflictsForward(vs LVSVirtualServer, forwards []WanPortForward) bool {
	for _, f := range forwards {
		if f.IPVersion != "ipv4" && f.IPVersion != "" {
			continue
		}
		for _, proto := range ForwardProtos(f.Proto) {
			if proto != vs.Protocol {
				continue
			}
			if f.DstPort != vs.Port {
				continue
			}
			if f.DstAddr == "" || strings.Contains(f.DstAddr, vs.VIP) {
				return true
			}
			if host, _, err := net.ParseCIDR(f.DstAddr); err == nil && host.String() == vs.VIP {
				return true
			}
		}
	}
	return false
}
