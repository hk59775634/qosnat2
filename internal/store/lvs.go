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
	Protocol       string          `json:"protocol"` // tcp | udp | tcp_udp
	Scheduler      string          `json:"scheduler,omitempty"`
	PersistenceSec int             `json:"persistence_sec,omitempty"`
	// PersistenceUDPSec UDP 虚拟服务会话保持（秒）；0=OCServ 集群默认不在 UDP 上单独 persistence。
	PersistenceUDPSec int `json:"persistence_udp_sec,omitempty"`
	AutoVIP           bool            `json:"auto_vip,omitempty"`
	WANDevice      string          `json:"wan_device,omitempty"`
	Service        string          `json:"service,omitempty"` // ocserv | ""
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
	case "tcp_udp", "tcp+udp", "both":
		v.Protocol = "tcp_udp"
	default:
		return fmt.Errorf("protocol must be tcp, udp, or tcp_udp")
	}
	svc := strings.ToLower(strings.TrimSpace(v.Service))
	if svc != "" && svc != "ocserv" {
		return fmt.Errorf("unsupported service: %s", v.Service)
	}
	v.Service = svc
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
	if v.PersistenceUDPSec < 0 {
		return fmt.Errorf("persistence_udp_sec invalid")
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

// LVSProtos 展开 tcp_udp（与 ForwardProtos 语义一致）。
func LVSProtos(proto string) []string {
	return ForwardProtos(proto)
}

// LVSPersistenceSec 返回某协议 ipvsadm -p 秒数；0 表示不启用 persistence。
func LVSPersistenceSec(vs LVSVirtualServer, proto string) int {
	proto = strings.ToLower(strings.TrimSpace(proto))
	if proto == "udp" {
		if vs.PersistenceUDPSec > 0 {
			return vs.PersistenceUDPSec
		}
		// OpenConnect：UDP(DTLS) 不与 TCP 共用 persistence 表项，避免认证割裂；靠 sh 同源选 RS。
		if vs.Service == "ocserv" {
			return 0
		}
	}
	if vs.PersistenceSec > 0 {
		return vs.PersistenceSec
	}
	return 0
}

// BuildLVSOCServCluster 生成 OpenConnect 集群虚拟服务（TCP 会话保持 + UDP 仅 sh，不单独 persistence）。
func BuildLVSOCServCluster(vip string, port int, nodes []string, defaultWAN string, autoVIP bool, persistenceSec int, scheduler string) (LVSVirtualServer, error) {
	vip = strings.TrimSpace(vip)
	if vip == "" {
		return LVSVirtualServer{}, fmt.Errorf("vip required")
	}
	if port <= 0 {
		port = 443
	}
	if persistenceSec <= 0 {
		persistenceSec = 3600
	}
	if strings.TrimSpace(scheduler) == "" {
		scheduler = "sh"
	}
	var rs []LVSRealServer
	for _, n := range nodes {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		rs = append(rs, LVSRealServer{IP: n, Port: port, Weight: 1})
	}
	vs := LVSVirtualServer{
		VIP:            vip,
		Port:           port,
		Protocol:       "tcp_udp",
		Service:        "ocserv",
		Scheduler:      scheduler,
		PersistenceSec: persistenceSec,
		AutoVIP:        autoVIP,
		WANDevice:      defaultWAN,
		RealServers:    rs,
		Comment:        "OpenConnect cluster",
	}
	if err := normalizeLVSVirtualServer(&vs, defaultWAN); err != nil {
		return LVSVirtualServer{}, err
	}
	return vs, nil
}

// LVSOCServClusterHint 供 UI 预填 OCServ 集群参数。
type LVSOCServClusterHint struct {
	DefaultPort              int    `json:"default_port"`
	DefaultPersistenceSec    int    `json:"default_persistence_sec"`
	DefaultUDPPersistenceSec int    `json:"default_udp_persistence_sec"`
	DefaultScheduler         string `json:"default_scheduler"`
	// ProxyProtoOnRS：纯 IPVS 不向 RS 发送 PROXY 头；RS 上 listen-proxy-proto 需 HAProxy 等，LVS 后直接开会失败。
	ProxyProtoOnRS bool   `json:"proxy_proto_on_rs"`
	ProxyProtoNote string `json:"proxy_proto_note,omitempty"`
}

func LVSOCServClusterHintFromState(st State) LVSOCServClusterHint {
	port := 443
	if st.VPN.OCServ.TCPPort > 0 {
		port = st.VPN.OCServ.TCPPort
	}
	return LVSOCServClusterHint{
		DefaultPort:              port,
		DefaultPersistenceSec:    3600,
		DefaultUDPPersistenceSec: 0,
		DefaultScheduler:         "sh",
		ProxyProtoOnRS:           false,
		ProxyProtoNote:           "Plain LVS does not send PROXY protocol; do not enable listen-proxy-proto on RS unless HAProxy emits PROXY v1/v2.",
	}
}

// LVSOCServConflictsLocal 本机已启用 OCServ 且与 LVS 虚拟服务端口冲突。
func LVSOCServConflictsLocal(vs LVSVirtualServer, o OCServState) bool {
	if !o.Enabled {
		return false
	}
	tcp, udp := 443, 443
	if o.TCPPort > 0 {
		tcp = o.TCPPort
	}
	if o.UDPPort > 0 {
		udp = o.UDPPort
	} else {
		udp = tcp
	}
	for _, proto := range LVSProtos(vs.Protocol) {
		switch proto {
		case "tcp":
			if vs.Port == tcp {
				return true
			}
		case "udp":
			if vs.Port == udp {
				return true
			}
		}
	}
	return false
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
			match := false
			for _, vp := range LVSProtos(vs.Protocol) {
				if vp == proto {
					match = true
					break
				}
			}
			if !match {
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

// CollectLVSInputEndpoints 汇总启用 LVS 时需在 WAN input 放行的 VIP:port。
func CollectLVSInputEndpoints(l LVSState) []AutoInputLVSEndpoint {
	if !l.Enabled {
		return nil
	}
	var out []AutoInputLVSEndpoint
	for _, vs := range l.VirtualServers {
		for _, proto := range LVSProtos(vs.Protocol) {
			out = append(out, AutoInputLVSEndpoint{
				VSID:  vs.ID,
				VIP:   vs.VIP,
				Port:  vs.Port,
				Proto: proto,
			})
		}
	}
	return out
}
