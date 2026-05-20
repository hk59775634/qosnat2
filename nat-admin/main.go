// nat-admin — NAT-QoS 轻量管理后台（Web UI + API）
package main

import (
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed static/*
var staticFS embed.FS

// --- 配置（/etc/nat-admin/env 或环境变量，由 initConfig 加载）---
var (
	adminUser, adminPass, adminPort string
	devLAN, devWAN, vpnNet          string
	nsGWIP, hostGWIP, ipvlanIF      string
	vethHost, vethNS                string
	vethHostIP, vethNSIP, vethPrefix string
	nsName, stateFile, sessionFile  string
	sessionKey                      = "nat_admin_sess"
	sessionTTL                      = 30 * 24 * time.Hour
)

const adminEnvFile = "/etc/nat-admin/env"

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// loadEnvFile 将 key=value 写入环境（不覆盖已存在的变量）
func loadEnvFile(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		i := strings.IndexByte(line, '=')
		if i <= 0 {
			continue
		}
		k := strings.TrimSpace(line[:i])
		v := strings.TrimSpace(line[i+1:])
		if len(v) >= 2 && ((v[0] == '"' && v[len(v)-1] == '"') || (v[0] == '\'' && v[len(v)-1] == '\'')) {
			v = v[1 : len(v)-1]
		}
		if os.Getenv(k) == "" {
			os.Setenv(k, v)
		}
	}
}

func initConfig() {
	loadEnvFile(adminEnvFile)
	adminUser = env("ADMIN_USER", "admin")
	adminPass = env("ADMIN_PASS", "NatAdmin@2026")
	adminPort = env("ADMIN_PORT", "8080")
	devLAN = env("DEV_LAN", "ens19")
	devWAN = env("DEV_WAN", "ens18")
	vpnNet = env("VPN_NET", "10.0.0.0/8")
	nsGWIP = env("NS_GW_IP", "172.16.99.2")
	hostGWIP = env("HOST_GW_IP", "172.16.99.1")
	ipvlanIF = env("IPVLAN_IF", "ipvl0")
	vethHost = env("VETH_HOST", "veth-nat0")
	vethNS = env("VETH_NS", "veth-nat1")
	vethHostIP = env("VETH_HOST_IP", "172.16.100.1")
	vethNSIP = env("VETH_NS_IP", "172.16.100.2")
	vethPrefix = env("VETH_PREFIX", "30")
	nsName = env("NS_NAME", "natns")
	stateFile = env("STATE_FILE", "/var/lib/nat-admin/state.json")
	sessionFile = env("SESSION_FILE", "/var/lib/nat-admin/sessions.json")
}

// --- 持久化状态 ---
type RateLimit struct {
	Down string `json:"down"`
	Up   string `json:"up"`
}

type State struct {
	PolicyRoutes    []string             `json:"policy_routes"`    // 策略路由网段（共享池 SNAT 兜底范围）
	InternalNet     string               `json:"internal_net,omitempty"` // 已废弃，迁移至 policy_routes
	SharedIPs       []string             `json:"shared_ips"`
	StaticMappings  map[string]string    `json:"static_mappings"`  // 单 IP -> 出口公网 IP
	PrefixMappings  map[string]string    `json:"prefix_mappings"`  // 内网网段 -> 公网网段 (prefix snat)
	DefaultRate     RateLimit            `json:"default_rate"`
	CidrLimits      map[string]RateLimit `json:"cidr_limits"`
	RateLimits      map[string]RateLimit `json:"rate_limits"`
	WanPortForwards []WanPortForward     `json:"wan_port_forwards"` // 公网口 DNAT → 宿主机
	APIKeys         []APIKey             `json:"api_keys"`
}

// WanPortForward 将 natns WAN 口端口映射到宿主机（默认 hostGWIP）
type WanPortForward struct {
	Proto    string `json:"proto"`     // tcp | udp
	WanPort  int    `json:"wan_port"`  // 公网监听端口
	HostIP   string `json:"host_ip"`   // 默认 172.16.99.1
	HostPort int    `json:"host_port"` // 宿主机端口
	Comment  string `json:"comment,omitempty"`
}

type APIKey struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Key       string    `json:"key"`
	CreatedAt time.Time `json:"created_at"`
}

var (
	mu       sync.RWMutex
	state    State
	sessions = map[string]time.Time{}
)

func loadState() error {
	mu.Lock()
	defer mu.Unlock()
	b, err := os.ReadFile(stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := json.Unmarshal(b, &state); err != nil {
		return err
	}
	ensureStateDefaults()
	return nil
}

func ensureStateDefaults() {
	if len(state.PolicyRoutes) == 0 {
		if c := strings.TrimSpace(state.InternalNet); c != "" {
			state.PolicyRoutes = []string{c}
		} else {
			state.PolicyRoutes = []string{vpnNet}
		}
	}
	state.PolicyRoutes = canonicalPolicyRoutes(state.PolicyRoutes)
	if state.StaticMappings == nil {
		state.StaticMappings = map[string]string{}
	}
	if state.PrefixMappings == nil {
		state.PrefixMappings = map[string]string{}
	}
	if state.CidrLimits == nil {
		state.CidrLimits = map[string]RateLimit{}
	}
	if state.RateLimits == nil {
		state.RateLimits = map[string]RateLimit{}
	}
	if state.WanPortForwards == nil {
		state.WanPortForwards = []WanPortForward{}
	}
	migrateWanForwardHostIP()
	seedWanForwardsFromEnv()
}

// interlinkHostIP 宿主机侧互联地址（veth，用于 DNAT / 策略路由回程）
func interlinkHostIP() string {
	return vethHostIP
}

func migrateWanForwardHostIP() {
	target := interlinkHostIP()
	for i := range state.WanPortForwards {
		if state.WanPortForwards[i].HostIP == "" || state.WanPortForwards[i].HostIP == hostGWIP {
			state.WanPortForwards[i].HostIP = target
		}
	}
}

func hasWanForwardLocked(proto string, wanPort int) bool {
	for _, x := range state.WanPortForwards {
		if x.Proto == proto && x.WanPort == wanPort {
			return true
		}
	}
	return false
}

func seedWanForwardsFromEnv() {
	if os.Getenv("WAN_SSH_DNAT") == "1" && !hasWanForwardLocked("tcp", 22) {
		upsertWanForwardLocked(WanPortForward{Proto: "tcp", WanPort: 22, HostIP: interlinkHostIP(), HostPort: 22, Comment: "SSH"})
	}
	if os.Getenv("WAN_ADMIN_DNAT") == "1" {
		p, _ := strconv.Atoi(adminPort)
		if p <= 0 {
			p = 8080
		}
		if !hasWanForwardLocked("tcp", p) {
			upsertWanForwardLocked(WanPortForward{Proto: "tcp", WanPort: p, HostIP: interlinkHostIP(), HostPort: p, Comment: "nat-admin"})
		}
	}
}

func upsertWanForwardLocked(f WanPortForward) {
	f = normalizeForward(f)
	for i, x := range state.WanPortForwards {
		if x.Proto == f.Proto && x.WanPort == f.WanPort {
			state.WanPortForwards[i] = f
			return
		}
	}
	state.WanPortForwards = append(state.WanPortForwards, f)
}

func normalizeForward(f WanPortForward) WanPortForward {
	f.Proto = strings.ToLower(strings.TrimSpace(f.Proto))
	if f.Proto != "udp" {
		f.Proto = "tcp"
	}
	if f.HostIP == "" {
		f.HostIP = interlinkHostIP()
	}
	if f.HostPort == 0 {
		f.HostPort = f.WanPort
	}
	if f.WanPort == 0 {
		f.WanPort = f.HostPort
	}
	return f
}

func getWanForwards() []WanPortForward {
	mu.RLock()
	out := append([]WanPortForward(nil), state.WanPortForwards...)
	mu.RUnlock()
	return out
}

func enableNatnsForward() {
	_, _ = nsExec("sysctl", "-w", "net.ipv4.ip_forward=1")
}

func canonicalPolicyRoutes(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, c := range in {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		canon, err := canonicalCIDR(c)
		if err != nil {
			continue
		}
		if seen[canon] {
			continue
		}
		seen[canon] = true
		out = append(out, canon)
	}
	sort.Strings(out)
	return out
}

func getPolicyRoutes() []string {
	mu.RLock()
	routes := append([]string(nil), state.PolicyRoutes...)
	mu.RUnlock()
	if len(routes) == 0 {
		return []string{vpnNet}
	}
	return routes
}

func saveState() error {
	mu.RLock()
	defer mu.RUnlock()
	if err := os.MkdirAll(filepath.Dir(stateFile), 0755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(stateFile, b, 0600)
}

// --- 网络操作 ---
func nsExec(args ...string) (string, error) {
	cmd := exec.Command("ip", append([]string{"netns", "exec", nsName}, args...)...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func nft(args ...string) (string, error) {
	return nsExec(append([]string{"nft"}, args...)...)
}

func reloadNftFromState() error {
	mu.RLock()
	ips := append([]string(nil), state.SharedIPs...)
	static := copyMap(state.StaticMappings)
	prefix := copyMap(state.PrefixMappings)
	routes := append([]string(nil), state.PolicyRoutes...)
	mu.RUnlock()
	if len(routes) == 0 {
		routes = []string{vpnNet}
	}
	if len(ips) == 0 {
		return fmt.Errorf("共享 IP 池为空")
	}
	nftPath := "/etc/nat-qos/nftables-natns.nft"
	var b strings.Builder
	b.WriteString("# NAT-QoS natns — 由 nat-admin 根据 state.json 生成\nflush ruleset\n\n")
	mu.RLock()
	forwards := append([]WanPortForward(nil), state.WanPortForwards...)
	mu.RUnlock()

	b.WriteString("table ip natqos {\n")
	b.WriteString(fmt.Sprintf("    flowtable fwd_fastpath { hook ingress priority filter; devices = { %s }; }\n\n", devWAN))
	b.WriteString("    chain prerouting {\n        type nat hook prerouting priority dstnat; policy accept;\n")
	for _, f := range forwards {
		b.WriteString(fmt.Sprintf("        iifname \"%s\" %s dport %d dnat to %s:%d\n",
			devWAN, f.Proto, f.WanPort, f.HostIP, f.HostPort))
		// 宿主机经 veth 访问公网 IP（hairpin）
		b.WriteString(fmt.Sprintf("        iifname \"%s\" %s dport %d dnat to %s:%d\n",
			vethNS, f.Proto, f.WanPort, f.HostIP, f.HostPort))
	}
	b.WriteString("    }\n\n")
	b.WriteString("    chain postrouting {\n        type nat hook postrouting priority srcnat; policy accept;\n")

	// 1) 单 IP 出口（最精确）
	staticKeys := sortedKeys(static)
	for _, inner := range staticKeys {
		b.WriteString(fmt.Sprintf("        ip saddr %s snat to %s\n", inner, static[inner]))
	}

	// 2) 网段 prefix → prefix 映射
	if len(prefix) > 0 {
		b.WriteString(fmt.Sprintf("        oifname \"%s\" snat ip prefix to ip saddr map {\n", devWAN))
		for _, inner := range sortedKeys(prefix) {
			b.WriteString(fmt.Sprintf("            %s : %s,\n", inner, prefix[inner]))
		}
		b.WriteString("        }\n")
	}

	// 3) 策略路由网段走共享池（未命中独立 IP / 网段映射的流量）
	var inline []string
	for i, ip := range ips {
		inline = append(inline, fmt.Sprintf("%d : %s", i, ip))
	}
	for _, cidr := range routes {
		b.WriteString(fmt.Sprintf("        ip saddr %s snat to numgen inc mod %d map { %s }\n",
			cidr, len(ips), strings.Join(inline, ", ")))
	}
	b.WriteString(fmt.Sprintf("        iifname \"%s\" oifname \"%s\" masquerade\n", devWAN, vethNS))
	b.WriteString(fmt.Sprintf("        oifname \"%s\" masquerade\n", vethNS))

	b.WriteString("    }\n\n    chain forward {\n        type filter hook forward priority filter; policy accept;\n")
	b.WriteString("        ct state established,related counter flow add @fwd_fastpath\n")
	b.WriteString("        ct state established,related accept\n")
	b.WriteString(fmt.Sprintf("        iifname \"%s\" oifname \"%s\" accept\n", devWAN, vethNS))
	b.WriteString(fmt.Sprintf("        iifname \"%s\" oifname \"%s\" accept\n", vethNS, devWAN))
	b.WriteString(fmt.Sprintf("        iifname \"%s\" accept\n        oifname \"%s\" accept\n        iifname \"%s\" accept\n", vethNS, devWAN, devWAN))
	b.WriteString("    }\n}\n")

	if err := os.WriteFile(nftPath, []byte(b.String()), 0644); err != nil {
		return err
	}
	if out, err := nsExec("nft", "-c", "-f", nftPath); err != nil {
		return fmt.Errorf("nft 语法检查: %s %w", strings.TrimSpace(out), err)
	}
	_, err := nsExec("nft", "-f", nftPath)
	if err != nil {
		return err
	}
	enableNatnsForward()
	return reloadHostNft()
}

func reloadHostNft() error {
	forwards := getWanForwards()
	routes := getPolicyRoutes()
	var tcpPorts, udpPorts []string
	seenTCP, seenUDP := map[int]bool{}, map[int]bool{}
	for _, f := range forwards {
		if f.Proto == "udp" {
			if !seenUDP[f.HostPort] {
				seenUDP[f.HostPort] = true
				udpPorts = append(udpPorts, strconv.Itoa(f.HostPort))
			}
		} else if !seenTCP[f.HostPort] {
			seenTCP[f.HostPort] = true
			tcpPorts = append(tcpPorts, strconv.Itoa(f.HostPort))
		}
	}
	ap, _ := strconv.Atoi(adminPort)
	if ap > 0 && !seenTCP[ap] {
		tcpPorts = append(tcpPorts, strconv.Itoa(ap))
	}
	if !seenTCP[22] {
		tcpPorts = append(tcpPorts, "22")
	}

	var b strings.Builder
	b.WriteString("# 宿主机防火墙 — 由 nat-admin 生成\nflush ruleset\n\n")
	b.WriteString("table inet hostfilter {\n    chain input {\n")
	b.WriteString("        type filter hook input priority filter; policy drop;\n")
	b.WriteString("        iifname \"lo\" accept\n        ct state established,related accept\n")
	b.WriteString(fmt.Sprintf("        iifname \"%s\" accept\n", devLAN))
	b.WriteString(fmt.Sprintf("        iifname \"%s\" tcp dport { 22, %s } accept\n", devLAN, adminPort))
	if len(tcpPorts) > 0 {
		b.WriteString(fmt.Sprintf("        ip daddr %s tcp dport { %s } accept\n", interlinkHostIP(), strings.Join(tcpPorts, ", ")))
	}
	if len(udpPorts) > 0 {
		b.WriteString(fmt.Sprintf("        ip daddr %s udp dport { %s } accept\n", interlinkHostIP(), strings.Join(udpPorts, ", ")))
	}
	b.WriteString(fmt.Sprintf("        iifname \"lo\" tcp dport %s accept\n        tcp dport %s accept\n", adminPort, adminPort))
	b.WriteString("        icmp type echo-request limit rate 10/second burst 20 packets accept\n    }\n")
	b.WriteString("    chain forward {\n        type filter hook forward priority filter; policy accept;\n")
	b.WriteString(fmt.Sprintf("        iifname \"%s\" accept\n        oifname \"%s\" accept\n", vethHost, vethHost))
	b.WriteString(fmt.Sprintf("        iifname \"%s\" oifname \"%s\" accept\n", devLAN, devLAN))
	// 须在宽泛 accept 之前：公网源直达 VPN 网段会绕过 natns（非对称回程）
	for _, cidr := range routes {
		b.WriteString(fmt.Sprintf(
			"        iifname \"%s\" ip daddr %s ip saddr != %s ip saddr != 172.16.0.0/24 drop\n",
			devLAN, cidr, cidr))
	}
	b.WriteString(fmt.Sprintf("        iifname \"%s\" accept\n    }\n    chain output {\n", devLAN))
	b.WriteString("        type filter hook output priority filter; policy accept;\n    }\n}\n")

	path := "/etc/nat-qos/hostfilter.nft"
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(b.String()), 0644); err != nil {
		return err
	}
	out, err := exec.Command("nft", "-f", path).CombinedOutput()
	if err != nil {
		return fmt.Errorf("host nft: %s %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func applyWanForwards() error {
	enableNatnsForward()
	if err := reloadNftFromState(); err != nil {
		return err
	}
	ensureWanReplyRoutes()
	return reloadHostNft()
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func applySharedPool() error      { return reloadNftFromState() }
func applyStaticMappings() error { return reloadNftFromState() }
func applyPrefixMappings() error { return reloadNftFromState() }

func linkExists(name string) bool {
	_, err := exec.Command("ip", "link", "show", name).Output()
	return err == nil
}

// ensureVethInterlink 创建 host↔natns veth（ipvlan l3s 在本环境无法互通，改用 veth）
func ensureVethInterlink() error {
	if !linkExists(vethHost) {
		peerTmp := vethNS + "-tmp"
		if out, err := exec.Command("ip", "link", "add", vethHost, "type", "veth", "peer", "name", peerTmp).CombinedOutput(); err != nil {
			return fmt.Errorf("创建 veth: %s %w", strings.TrimSpace(string(out)), err)
		}
		if out, err := exec.Command("ip", "link", "set", peerTmp, "netns", nsName).CombinedOutput(); err != nil {
			return fmt.Errorf("veth 移入 netns: %s %w", strings.TrimSpace(string(out)), err)
		}
		if out, err := nsExec("ip", "link", "set", peerTmp, "name", vethNS); err != nil {
			return fmt.Errorf("veth 重命名: %s %w", strings.TrimSpace(out), err)
		}
		addr := fmt.Sprintf("%s/%s", vethHostIP, vethPrefix)
		if out, err := exec.Command("ip", "addr", "add", addr, "dev", vethHost).CombinedOutput(); err != nil && !strings.Contains(string(out), "File exists") {
			return fmt.Errorf("veth 宿主机地址: %s %w", strings.TrimSpace(string(out)), err)
		}
		exec.Command("ip", "link", "set", vethHost, "up").Run()
		nsAddr := fmt.Sprintf("%s/%s", vethNSIP, vethPrefix)
		nsExec("ip", "addr", "add", nsAddr, "dev", vethNS)
		nsExec("ip", "link", "set", vethNS, "up")
		log.Printf("已创建 veth 互联 %s (%s) ↔ %s (%s)", vethHost, vethHostIP, vethNS, vethNSIP)
	}
	exec.Command("ip", "link", "set", vethHost, "up").Run()
	nsExec("ip", "link", "set", vethNS, "up")
	if out, err := exec.Command("ip", "route", "replace", "table", "100", "default", "via", vethNSIP, "dev", vethHost).CombinedOutput(); err != nil {
		return fmt.Errorf("策略路由表 100: %s %w", strings.TrimSpace(string(out)), err)
	}
	ensureWanReplyRoutes()
	return nil
}

// ensureWanReplyRoutes 公网网段经 veth 回程（配合 WAN DNAT）
func ensureWanReplyRoutes() {
	mu.RLock()
	ips := append([]string(nil), state.SharedIPs...)
	mu.RUnlock()
	seen := map[string]bool{}
	for _, ip := range ips {
		p := net.ParseIP(ip).To4()
		if p == nil {
			continue
		}
		subnet := fmt.Sprintf("%d.%d.%d.0/24", p[0], p[1], p[2])
		if seen[subnet] {
			continue
		}
		seen[subnet] = true
		exec.Command("ip", "route", "replace", subnet, "via", vethNSIP, "dev", vethHost).Run()
	}
}

func delPolicyRoutes(cidr string) {
	if cidr == "" {
		return
	}
	exec.Command("ip", "rule", "del", "from", "all", "to", cidr, "lookup", "100").Run()
	exec.Command("ip", "rule", "del", "from", cidr, "lookup", "100").Run()
	nsExec("ip", "route", "del", cidr, "via", vethHostIP, "dev", vethNS)
	nsExec("ip", "route", "del", cidr, "via", hostGWIP, "dev", ipvlanIF)
}

func applyPolicyRoutes(cidr string) error {
	if err := ensureVethInterlink(); err != nil {
		return err
	}
	canon, err := canonicalCIDR(cidr)
	if err != nil {
		return err
	}
	exec.Command("ip", "rule", "del", "from", "all", "to", canon, "lookup", "100").Run()
	exec.Command("ip", "rule", "del", "to", canon, "lookup", "main", "priority", "50").Run()
	if out, err := exec.Command("ip", "rule", "add", "to", canon, "lookup", "main", "priority", "50").CombinedOutput(); err != nil {
		return fmt.Errorf("ip rule to: %s %w", strings.TrimSpace(string(out)), err)
	}
	exec.Command("ip", "rule", "del", "from", canon, "lookup", "100").Run()
	if out, err := exec.Command("ip", "rule", "add", "from", canon, "lookup", "100", "priority", "100").CombinedOutput(); err != nil {
		return fmt.Errorf("ip rule from: %s %w", strings.TrimSpace(string(out)), err)
	}
	if out, err := nsExec("ip", "route", "replace", canon, "via", vethHostIP, "dev", vethNS); err != nil {
		return fmt.Errorf("natns 路由: %s %w", strings.TrimSpace(string(out)), err)
	}
	exec.Command("ip", "route", "flush", "cache").Run()
	return nil
}

func syncAllPolicyRoutes() error {
	if err := ensureVethInterlink(); err != nil {
		return err
	}
	routes := getPolicyRoutes()
	if len(routes) == 0 {
		return fmt.Errorf("策略路由网段为空")
	}
	for _, cidr := range routes {
		if err := applyPolicyRoutes(cidr); err != nil {
			return err
		}
	}
	exec.Command("ip", "route", "flush", "cache").Run()
	return nil
}

func addPolicyRoute(cidr string) error {
	canon, err := canonicalCIDR(cidr)
	if err != nil {
		return err
	}
	mu.RLock()
	for _, r := range state.PolicyRoutes {
		if r == canon {
			mu.RUnlock()
			return nil
		}
	}
	mu.RUnlock()
	if err := applyPolicyRoutes(canon); err != nil {
		return err
	}
	if err := reloadNftFromState(); err != nil {
		delPolicyRoutes(canon)
		return err
	}
	mu.Lock()
	state.PolicyRoutes = canonicalPolicyRoutes(append(state.PolicyRoutes, canon))
	mu.Unlock()
	return nil
}

func removePolicyRoute(cidr string) error {
	canon, err := canonicalCIDR(cidr)
	if err != nil {
		return err
	}
	mu.RLock()
	var kept []string
	for _, r := range state.PolicyRoutes {
		if r != canon {
			kept = append(kept, r)
		}
	}
	mu.RUnlock()
	if len(kept) == 0 {
		return fmt.Errorf("至少保留一个策略路由网段")
	}
	delPolicyRoutes(canon)
	if err := reloadNftFromState(); err != nil {
		_ = applyPolicyRoutes(canon)
		return err
	}
	mu.Lock()
	state.PolicyRoutes = kept
	mu.Unlock()
	return nil
}

const bpfCtl = "/usr/local/bin/nat-qos-bpf"

func bpfRun(args ...string) error {
	out, err := exec.Command(bpfCtl, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func applyDefaultRate(rl RateLimit) error {
	down, up := rl.Down, rl.Up
	if down == "" {
		down = "10mbit"
	}
	if up == "" {
		up = "4mbit"
	}
	return bpfRun("set-default", down, up)
}

func canonicalCIDR(cidr string) (string, error) {
	_, n, err := net.ParseCIDR(strings.TrimSpace(cidr))
	if err != nil {
		return "", fmt.Errorf("无效 CIDR: %s", cidr)
	}
	ones, bits := n.Mask.Size()
	if bits != 32 {
		return "", fmt.Errorf("仅支持 IPv4 CIDR")
	}
	ip4 := n.IP.To4()
	if ip4 == nil {
		return "", fmt.Errorf("仅支持 IPv4 CIDR")
	}
	masked := ip4.Mask(n.Mask)
	return fmt.Sprintf("%s/%d", masked.String(), ones), nil
}

func applyCidrLimit(cidr string, rl RateLimit) error {
	down, up := rl.Down, rl.Up
	if down == "" || up == "" {
		return fmt.Errorf("上下行速率均不能为空")
	}
	return bpfRun("set-cidr", cidr, down, up)
}

func removeCidrLimit(cidr string) error {
	return bpfRun("del-cidr", cidr)
}

func applyRateLimit(ip string, rl RateLimit) error {
	down := rl.Down
	if down == "" {
		down = "10mbit"
	}
	up := rl.Up
	if up == "" {
		up = "4mbit"
	}
	return bpfRun("set-ip", ip, down, up)
}

func removeRateLimit(ip string) error {
	return bpfRun("del-ip", ip)
}

func restoreAll() {
	log.Println("从持久化状态恢复配置...")
	if err := ensureVethInterlink(); err != nil {
		log.Printf("veth 互联: %v", err)
	}
	if err := syncAllPolicyRoutes(); err != nil {
		log.Printf("恢复策略路由: %v", err)
	}
	if err := reloadNftFromState(); err != nil {
		log.Printf("恢复 Nftables: %v", err)
	}
	mu.RLock()
	def := state.DefaultRate
	cidrs := copyRateLimits(state.CidrLimits)
	limits := copyRateLimits(state.RateLimits)
	mu.RUnlock()
	if def.Down != "" || def.Up != "" {
		if err := applyDefaultRate(def); err != nil {
			log.Printf("恢复默认限速: %v", err)
		}
	}
	for cidr, rl := range cidrs {
		if err := applyCidrLimit(cidr, rl); err != nil {
			log.Printf("恢复网段限速 %s: %v", cidr, err)
		}
	}
	for ip, rl := range limits {
		if err := applyRateLimit(ip, rl); err != nil {
			log.Printf("恢复单 IP 限速 %s: %v", ip, err)
		}
	}
}

func copyMap(m map[string]string) map[string]string {
	o := make(map[string]string, len(m))
	for k, v := range m {
		o[k] = v
	}
	return o
}

func copyRateLimits(m map[string]RateLimit) map[string]RateLimit {
	o := make(map[string]RateLimit, len(m))
	for k, v := range m {
		o[k] = v
	}
	return o
}

// --- HTTP 鉴权 ---
func loadSessions() error {
	b, err := os.ReadFile(sessionFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var loaded map[string]time.Time
	if err := json.Unmarshal(b, &loaded); err != nil {
		return err
	}
	now := time.Now()
	mu.Lock()
	defer mu.Unlock()
	for sid, exp := range loaded {
		if now.Before(exp) {
			sessions[sid] = exp
		}
	}
	return nil
}

func saveSessions() error {
	mu.Lock()
	defer mu.Unlock()
	now := time.Now()
	active := make(map[string]time.Time, len(sessions))
	for sid, exp := range sessions {
		if now.Before(exp) {
			active[sid] = exp
		}
	}
	if err := os.MkdirAll(filepath.Dir(sessionFile), 0755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(active, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(sessionFile, b, 0600)
}

func newSession() string {
	b := make([]byte, 32)
	rand.Read(b)
	sid := hex.EncodeToString(b)
	mu.Lock()
	sessions[sid] = time.Now().Add(sessionTTL)
	mu.Unlock()
	if err := saveSessions(); err != nil {
		log.Printf("保存会话: %v", err)
	}
	return sid
}

func validSession(r *http.Request) bool {
	c, err := r.Cookie(sessionKey)
	if err != nil {
		return false
	}
	mu.Lock()
	defer mu.Unlock()
	exp, ok := sessions[c.Value]
	return ok && time.Now().Before(exp)
}

func validAPIKey(r *http.Request) bool {
	key := r.Header.Get("X-API-Key")
	if key == "" {
		return false
	}
	mu.RLock()
	defer mu.RUnlock()
	for _, k := range state.APIKeys {
		if subtle.ConstantTimeCompare([]byte(k.Key), []byte(key)) == 1 {
			return true
		}
	}
	return false
}

func requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if validSession(r) || validAPIKey(r) {
			next(w, r)
			return
		}
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// --- 路由处理器 ---
func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var req struct{ User, Pass string }
	json.NewDecoder(r.Body).Decode(&req)
	if req.User != adminUser || req.Pass != adminPass {
		http.Error(w, `{"error":"invalid credentials"}`, 401)
		return
	}
	sid := newSession()
	maxAge := int(sessionTTL.Seconds())
	http.SetCookie(w, &http.Cookie{Name: sessionKey, Value: sid, Path: "/", HttpOnly: true, MaxAge: maxAge, SameSite: http.SameSiteLaxMode})
	writeJSON(w, map[string]bool{"ok": true})
}

func handleSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}
	writeJSON(w, map[string]bool{"ok": validSession(r)})
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionKey); err == nil {
		mu.Lock()
		delete(sessions, c.Value)
		mu.Unlock()
		_ = saveSessions()
	}
	http.SetCookie(w, &http.Cookie{Name: sessionKey, Value: "", Path: "/", MaxAge: -1})
	writeJSON(w, map[string]bool{"ok": true})
}

func handleStats(w http.ResponseWriter, r *http.Request) {
	cpu := readCPU()
	mem := readMem()
	ct := readConntrack()
	lanRx, lanTx := readIfaceMbps(devLAN)
	wanRx, wanTx := readIfaceMbpsNS(devWAN)
	mu.RLock()
	defer mu.RUnlock()
	writeJSON(w, map[string]any{
		"cpu_percent": cpu, "mem_percent": mem, "conntrack": ct,
		"lan": map[string]float64{"rx_mbps": lanRx, "tx_mbps": lanTx},
		"wan": map[string]float64{"rx_mbps": wanRx, "tx_mbps": wanTx},
		"shared_ips": len(state.SharedIPs), "static_mappings": len(state.StaticMappings),
	})
}

func handleWanForwards(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, getWanForwards())
	case http.MethodPost:
		var f WanPortForward
		json.NewDecoder(r.Body).Decode(&f)
		if f.WanPort < 1 || f.WanPort > 65535 || f.HostPort < 1 || f.HostPort > 65535 {
			http.Error(w, "端口须在 1-65535", 400)
			return
		}
		f = normalizeForward(f)
		mu.Lock()
		upsertWanForwardLocked(f)
		mu.Unlock()
		if err := applyWanForwards(); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		saveState()
		writeJSON(w, getWanForwards())
	case http.MethodDelete:
		proto := strings.ToLower(r.URL.Query().Get("proto"))
		if proto == "" {
			proto = "tcp"
		}
		wp, _ := strconv.Atoi(r.URL.Query().Get("wan_port"))
		if wp < 1 {
			http.Error(w, "wan_port required", 400)
			return
		}
		mu.Lock()
		var kept []WanPortForward
		for _, x := range state.WanPortForwards {
			if !(x.Proto == proto && x.WanPort == wp) {
				kept = append(kept, x)
			}
		}
		state.WanPortForwards = kept
		mu.Unlock()
		if err := applyWanForwards(); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		saveState()
		writeJSON(w, getWanForwards())
	default:
		http.Error(w, "method not allowed", 405)
	}
}

func handlePolicyRoutes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, getPolicyRoutes())
	case http.MethodPost:
		var body struct {
			CIDR string `json:"cidr"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		if strings.TrimSpace(body.CIDR) == "" {
			http.Error(w, "cidr required", 400)
			return
		}
		if err := addPolicyRoute(body.CIDR); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		saveState()
		writeJSON(w, getPolicyRoutes())
	case http.MethodDelete:
		cidr := r.URL.Query().Get("cidr")
		if cidr == "" {
			http.Error(w, "cidr required", 400)
			return
		}
		if err := removePolicyRoute(cidr); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		saveState()
		writeJSON(w, getPolicyRoutes())
	default:
		http.Error(w, "method not allowed", 405)
	}
}

func handleSharedIPs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		mu.RLock()
		ips := append([]string(nil), state.SharedIPs...)
		mu.RUnlock()
		writeJSON(w, ips)
	case http.MethodPost:
		var body struct{ IP string `json:"ip"` }
		json.NewDecoder(r.Body).Decode(&body)
		if body.IP == "" {
			http.Error(w, "ip required", 400)
			return
		}
		mu.Lock()
		state.SharedIPs = append(state.SharedIPs, body.IP)
		mu.Unlock()
		if err := applySharedPool(); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		saveState()
		writeJSON(w, map[string]bool{"ok": true})
	case http.MethodDelete:
		ip := r.URL.Query().Get("ip")
		mu.Lock()
		var n []string
		for _, x := range state.SharedIPs {
			if x != ip {
				n = append(n, x)
			}
		}
		state.SharedIPs = n
		mu.Unlock()
		if err := applySharedPool(); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		saveState()
		writeJSON(w, map[string]bool{"ok": true})
	}
}

func handlePrefixMappings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		mu.RLock()
		m := copyMap(state.PrefixMappings)
		mu.RUnlock()
		writeJSON(w, m)
	case http.MethodPost, http.MethodPut:
		var body struct {
			InnerCIDR string `json:"inner_cidr"`
			OuterCIDR string `json:"outer_cidr"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		inner, err := canonicalCIDR(body.InnerCIDR)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		outer, err := canonicalCIDR(body.OuterCIDR)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		mu.Lock()
		if state.PrefixMappings == nil {
			state.PrefixMappings = map[string]string{}
		}
		state.PrefixMappings[inner] = outer
		mu.Unlock()
		if err := applyPrefixMappings(); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		saveState()
		writeJSON(w, map[string]bool{"ok": true})
	case http.MethodDelete:
		inner := r.URL.Query().Get("inner_cidr")
		if canon, err := canonicalCIDR(inner); err == nil {
			inner = canon
		}
		mu.Lock()
		delete(state.PrefixMappings, inner)
		mu.Unlock()
		if err := applyPrefixMappings(); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		saveState()
		writeJSON(w, map[string]bool{"ok": true})
	}
}

func handleStatic(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		mu.RLock()
		m := copyMap(state.StaticMappings)
		mu.RUnlock()
		writeJSON(w, m)
	case http.MethodPost:
		var body struct{ Inner, Outer string }
		json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		if state.StaticMappings == nil {
			state.StaticMappings = map[string]string{}
		}
		state.StaticMappings[body.Inner] = body.Outer
		mu.Unlock()
		if err := applyStaticMappings(); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		saveState()
		writeJSON(w, map[string]bool{"ok": true})
	case http.MethodDelete:
		inner := r.URL.Query().Get("inner")
		mu.Lock()
		delete(state.StaticMappings, inner)
		mu.Unlock()
		if err := applyStaticMappings(); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		saveState()
		writeJSON(w, map[string]bool{"ok": true})
	}
}

func handleRateDefault(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		mu.RLock()
		rl := state.DefaultRate
		mu.RUnlock()
		writeJSON(w, rl)
	case http.MethodPost, http.MethodPut:
		var rl RateLimit
		json.NewDecoder(r.Body).Decode(&rl)
		if err := applyDefaultRate(rl); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		mu.Lock()
		state.DefaultRate = rl
		mu.Unlock()
		saveState()
		writeJSON(w, map[string]bool{"ok": true})
	}
}

func handleRateCidrs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		mu.RLock()
		m := copyRateLimits(state.CidrLimits)
		mu.RUnlock()
		writeJSON(w, m)
	case http.MethodPost, http.MethodPut:
		var body struct {
			CIDR string `json:"cidr"`
			Down string `json:"down"`
			Up   string `json:"up"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		canon, err := canonicalCIDR(body.CIDR)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		rl := RateLimit{Down: body.Down, Up: body.Up}
		if err := applyCidrLimit(canon, rl); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		mu.Lock()
		if state.CidrLimits == nil {
			state.CidrLimits = map[string]RateLimit{}
		}
		state.CidrLimits[canon] = rl
		mu.Unlock()
		saveState()
		writeJSON(w, map[string]bool{"ok": true})
	case http.MethodDelete:
		cidr := r.URL.Query().Get("cidr")
		if canon, err := canonicalCIDR(cidr); err == nil {
			cidr = canon
		}
		if err := removeCidrLimit(cidr); err != nil {
			log.Printf("删除网段限速 %s: %v", cidr, err)
		}
		mu.Lock()
		delete(state.CidrLimits, cidr)
		mu.Unlock()
		saveState()
		writeJSON(w, map[string]bool{"ok": true})
	}
}

func handleRate(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		mu.RLock()
		m := copyRateLimits(state.RateLimits)
		mu.RUnlock()
		writeJSON(w, m)
	case http.MethodPost, http.MethodPut:
		var body struct {
			IP   string `json:"ip"`
			Down string `json:"down"`
			Up   string `json:"up"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		rl := RateLimit{Down: body.Down, Up: body.Up}
		if err := applyRateLimit(body.IP, rl); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		mu.Lock()
		if state.RateLimits == nil {
			state.RateLimits = map[string]RateLimit{}
		}
		state.RateLimits[body.IP] = rl
		mu.Unlock()
		saveState()
		writeJSON(w, map[string]bool{"ok": true})
	case http.MethodDelete:
		ip := r.URL.Query().Get("ip")
		if err := removeRateLimit(ip); err != nil {
			log.Printf("删除限速 %s: %v", ip, err)
		}
		mu.Lock()
		delete(state.RateLimits, ip)
		mu.Unlock()
		saveState()
		writeJSON(w, map[string]bool{"ok": true})
	}
}

func handleAPIKeys(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		mu.RLock()
		keys := make([]APIKey, len(state.APIKeys))
		copy(keys, state.APIKeys)
		mu.RUnlock()
		writeJSON(w, keys)
	case http.MethodPost:
		var body struct{ Name string `json:"name"` }
		json.NewDecoder(r.Body).Decode(&body)
		b := make([]byte, 24)
		rand.Read(b)
		key := hex.EncodeToString(b)
		ak := APIKey{ID: hex.EncodeToString(b[:8]), Name: body.Name, Key: key, CreatedAt: time.Now()}
		mu.Lock()
		state.APIKeys = append(state.APIKeys, ak)
		mu.Unlock()
		saveState()
		writeJSON(w, ak)
	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		mu.Lock()
		var n []APIKey
		for _, k := range state.APIKeys {
			if k.ID != id {
				n = append(n, k)
			}
		}
		state.APIKeys = n
		mu.Unlock()
		saveState()
		writeJSON(w, map[string]bool{"ok": true})
	}
}

// --- 监控指标 ---
var lastLAN, lastWAN [2]uint64
var lastTS time.Time

func readCPU() float64 {
	b, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0
	}
	lines := strings.Split(string(b), "\n")
	if len(lines) < 1 {
		return 0
	}
	f := strings.Fields(lines[0])
	if len(f) < 8 {
		return 0
	}
	var total, idle uint64
	for i := 1; i < len(f); i++ {
		v, _ := strconv.ParseUint(f[i], 10, 64)
		total += v
		if i == 4 {
			idle = v
		}
	}
	time.Sleep(200 * time.Millisecond)
	b2, _ := os.ReadFile("/proc/stat")
	lines2 := strings.Split(string(b2), "\n")
	f2 := strings.Fields(lines2[0])
	var total2, idle2 uint64
	for i := 1; i < len(f2); i++ {
		v, _ := strconv.ParseUint(f2[i], 10, 64)
		total2 += v
		if i == 4 {
			idle2 = v
		}
	}
	dt := float64(total2 - total)
	if dt == 0 {
		return 0
	}
	return (1 - float64(idle2-idle)/dt) * 100
}

func readMem() float64 {
	b, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	var total, avail float64
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "MemTotal:") {
			fmt.Sscanf(line, "MemTotal: %f kB", &total)
		}
		if strings.HasPrefix(line, "MemAvailable:") {
			fmt.Sscanf(line, "MemAvailable: %f kB", &avail)
		}
	}
	if total == 0 {
		return 0
	}
	return (1 - avail/total) * 100
}

func readConntrack() int {
	out, err := nsExec("sysctl", "-n", "net.netfilter.nf_conntrack_count")
	if err != nil {
		b, e2 := os.ReadFile("/proc/sys/net/netfilter/nf_conntrack_count")
		if e2 != nil {
			return 0
		}
		out = string(b)
	}
	n, _ := strconv.Atoi(strings.TrimSpace(out))
	return n
}

func readIfaceBytes(dev string) (rx, tx uint64) {
	readCounters := func(path string) uint64 {
		b, err := os.ReadFile(path)
		if err != nil {
			return 0
		}
		f := strings.Fields(string(b))
		if len(f) < 2 {
			return 0
		}
		v, _ := strconv.ParseUint(f[1], 10, 64)
		return v
	}
	rx = readCounters(fmt.Sprintf("/sys/class/net/%s/statistics/rx_bytes", dev))
	tx = readCounters(fmt.Sprintf("/sys/class/net/%s/statistics/tx_bytes", dev))
	return
}

func readIfaceMbps(dev string) (rx, tx float64) {
	rx1, tx1 := readIfaceBytes(dev)
	now := time.Now()
	dt := now.Sub(lastTS).Seconds()
	if dt < 0.1 {
		dt = 1
	}
	if lastTS.IsZero() {
		lastTS = now
		return 0, 0
	}
	rx = float64(rx1-lastLAN[0]) * 8 / dt / 1e6
	tx = float64(tx1-lastLAN[1]) * 8 / dt / 1e6
	lastLAN = [2]uint64{rx1, tx1}
	lastTS = now
	return
}

func readIfaceMbpsNS(dev string) (rx, tx float64) {
	out, _ := nsExec("cat", fmt.Sprintf("/sys/class/net/%s/statistics/rx_bytes", dev))
	rx1, _ := strconv.ParseUint(strings.TrimSpace(out), 10, 64)
	out, _ = nsExec("cat", fmt.Sprintf("/sys/class/net/%s/statistics/tx_bytes", dev))
	tx1, _ := strconv.ParseUint(strings.TrimSpace(out), 10, 64)
	dt := 1.0
	rx = float64(rx1) * 8 / dt / 1e6
	tx = float64(tx1) * 8 / dt / 1e6
	return
}

func runApplyState() {
	if err := loadState(); err != nil {
		log.Fatalf("加载状态: %v", err)
	}
	restoreAll()
	if err := saveState(); err != nil {
		log.Printf("保存状态: %v", err)
	}
	log.Println("配置已从 state.json 应用")
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	initConfig()
	if len(os.Args) > 1 && os.Args[1] == "apply-state" {
		runApplyState()
		return
	}
	if err := loadState(); err != nil {
		log.Fatalf("加载状态: %v", err)
	}
	if err := loadSessions(); err != nil {
		log.Fatalf("加载会话: %v", err)
	}
	// 等待 nat-qos 就绪后恢复
	go func() {
		for i := 0; i < 30; i++ {
			if _, err := exec.Command("ip", "netns", "list").Output(); err == nil {
				if out, _ := exec.Command("ip", "netns", "list").Output(); strings.Contains(string(out), nsName) {
					break
				}
			}
			time.Sleep(2 * time.Second)
		}
		time.Sleep(3 * time.Second)
		restoreAll()
	}()

	mux := http.NewServeMux()
	// API
	mux.HandleFunc("/api/login", handleLogin)
	mux.HandleFunc("/api/session", handleSession)
	mux.HandleFunc("/api/logout", requireAuth(handleLogout))
	mux.HandleFunc("/api/stats", requireAuth(handleStats))
	mux.HandleFunc("/api/policy-routes", requireAuth(handlePolicyRoutes))
	mux.HandleFunc("/api/wan-forwards", requireAuth(handleWanForwards))
	mux.HandleFunc("/api/shared-ips", requireAuth(handleSharedIPs))
	mux.HandleFunc("/api/static-mappings", requireAuth(handleStatic))
	mux.HandleFunc("/api/prefix-mappings", requireAuth(handlePrefixMappings))
	mux.HandleFunc("/api/rate-default", requireAuth(handleRateDefault))
	mux.HandleFunc("/api/rate-cidrs", requireAuth(handleRateCidrs))
	mux.HandleFunc("/api/rate-limits", requireAuth(handleRate))
	mux.HandleFunc("/api/keys", requireAuth(handleAPIKeys))

	// 静态 UI
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/" {
			path = "/index.html"
		}
		f, err := staticFS.Open("static" + path)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		switch {
		case strings.HasSuffix(path, ".html"):
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
		case strings.HasSuffix(path, ".yaml"), strings.HasSuffix(path, ".yml"):
			w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		}
		io.Copy(w, f)
	})

	addr := ":" + adminPort
	log.Printf("nat-admin 监听 %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
