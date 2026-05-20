// nat-qos-bpf — 加载/卸载 eBPF 每 IP 限速，管理 config_map
package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/cilium/ebpf"
)

const (
	pinDir    = "/sys/fs/bpf/nat_qos"
	configPin = pinDir + "/config_map"
	cidrPin   = pinDir + "/cidr_map"
	statePin  = pinDir + "/state_map"
	bpfObject = "/usr/lib/nat-qos/ratelimit.bpf.o"
)

// LpmKey 与 BPF struct lpm_v4_key 对齐
type LpmKey struct {
	PrefixLen uint32
	Addr      [4]byte
}

type ipRates struct {
	UpBps     uint64
	UpBurst   uint64
	DownBps   uint64
	DownBurst uint64
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	var err error
	switch os.Args[1] {
	case "attach":
		iface := argOr(2, env("DEV_LAN", "ens19"))
		down := argOr(3, env("DEFAULT_DOWN_RATE", "10mbit"))
		up := argOr(4, env("DEFAULT_UP_RATE", "4mbit"))
		err = doAttach(iface, down, up)
	case "detach":
		iface := argOr(2, env("DEV_LAN", "ens19"))
		err = doDetach(iface)
	case "set-default":
		err = setDefault(argOr(2, "10mbit"), argOr(3, "4mbit"))
	case "set-ip":
		if len(os.Args) < 5 {
			die("用法: set-ip <ip> <down> <up>")
		}
		err = setIP(os.Args[2], os.Args[3], os.Args[4])
	case "del-ip":
		if len(os.Args) < 3 {
			die("用法: del-ip <ip>")
		}
		err = delIP(os.Args[2])
	case "set-cidr":
		if len(os.Args) < 5 {
			die("用法: set-cidr <cidr> <down> <up>")
		}
		err = setCIDR(os.Args[2], os.Args[3], os.Args[4])
	case "del-cidr":
		if len(os.Args) < 3 {
			die("用法: del-cidr <cidr>")
		}
		err = delCIDR(os.Args[2])
	case "list":
		err = listLimits()
	case "status":
		err = showStatus()
	default:
		usage()
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `用法: nat-qos-bpf <命令> [参数]
  attach [iface] [down] [up]  加载 eBPF 并挂接 TC clsact
  detach [iface]              卸载
  set-default <down> <up>       设置全局默认限速（未单独配置的每 IP）
  set-cidr <cidr> <down> <up> 设置网段限速（如 10.100.0.0/16）
  del-cidr <cidr>             删除网段规则
  set-ip <ip> <down> <up>     设置单 IP 覆盖（最高优先级）
  del-ip <ip>                 删除单 IP 覆盖
  list                        列出当前限速规则
  status                      查看状态
`)
}

func argOr(i int, def string) string {
	if len(os.Args) > i && os.Args[i] != "" {
		return os.Args[i]
	}
	return def
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func die(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}

func parseRate(s string) (bps uint64, burst uint64, err error) {
	s = strings.TrimSpace(strings.ToLower(s))
	var mult uint64 = 1
	switch {
	case strings.HasSuffix(s, "gbit"):
		mult = 125000000
		s = strings.TrimSuffix(s, "gbit")
	case strings.HasSuffix(s, "mbit"):
		mult = 125000
		s = strings.TrimSuffix(s, "mbit")
	case strings.HasSuffix(s, "kbit"):
		mult = 125
		s = strings.TrimSuffix(s, "kbit")
	case strings.HasSuffix(s, "bps"):
		mult = 1
		s = strings.TrimSuffix(s, "bps")
	default:
		return 0, 0, fmt.Errorf("未知速率单位: %s (支持 mbit/kbit/gbit)", s)
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, 0, err
	}
	bps = uint64(v * float64(mult))
	if bps == 0 {
		return 0, 0, errors.New("速率必须大于 0")
	}
	// 突发 = 5 秒满速率（字节/秒），利于 TCP 窗口与测速
	burst = bps * 5
	if burst < 131072 {
		burst = 131072
	}
	if burst > 32*1024*1024 {
		burst = 32 * 1024 * 1024
	}
	return bps, burst, nil
}

func ratesFromStrings(down, up string) (ipRates, error) {
	dBps, dBurst, err := parseRate(down)
	if err != nil {
		return ipRates{}, err
	}
	uBps, uBurst, err := parseRate(up)
	if err != nil {
		return ipRates{}, err
	}
	return ipRates{UpBps: uBps, UpBurst: uBurst, DownBps: dBps, DownBurst: dBurst}, nil
}

func ipToKey(ip string) (uint32, error) {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return 0, fmt.Errorf("无效 IP: %s", ip)
	}
	v4 := parsed.To4()
	if v4 == nil {
		return 0, fmt.Errorf("需要 IPv4: %s", ip)
	}
	return binary.BigEndian.Uint32(v4), nil
}

func openConfigMap() (*ebpf.Map, error) {
	if err := os.MkdirAll(pinDir, 0755); err != nil {
		return nil, err
	}
	m, err := ebpf.LoadPinnedMap(configPin, nil)
	if err == nil {
		return m, nil
	}
	return nil, fmt.Errorf("config_map 未加载，请先 attach: %w", err)
}

func openCidrMap() (*ebpf.Map, error) {
	m, err := ebpf.LoadPinnedMap(cidrPin, nil)
	if err != nil {
		return nil, fmt.Errorf("cidr_map 未加载，请先 attach 并升级 BPF: %w", err)
	}
	return m, nil
}

func normalizeCIDR(cidr string) (string, LpmKey, error) {
	_, n, err := net.ParseCIDR(strings.TrimSpace(cidr))
	if err != nil {
		return "", LpmKey{}, fmt.Errorf("无效 CIDR: %s", cidr)
	}
	ones, bits := n.Mask.Size()
	if bits != 32 {
		return "", LpmKey{}, errors.New("仅支持 IPv4 CIDR")
	}
	ip4 := n.IP.To4()
	if ip4 == nil {
		return "", LpmKey{}, errors.New("仅支持 IPv4 CIDR")
	}
	masked := ip4.Mask(n.Mask)
	var k LpmKey
	k.PrefixLen = uint32(ones)
	copy(k.Addr[:], masked)
	return fmt.Sprintf("%s/%d", masked.String(), ones), k, nil
}

func decodeRates(b []byte) ipRates {
	if len(b) < 32 {
		return ipRates{}
	}
	return ipRates{
		UpBps:     binary.LittleEndian.Uint64(b[0:8]),
		UpBurst:   binary.LittleEndian.Uint64(b[8:16]),
		DownBps:   binary.LittleEndian.Uint64(b[16:24]),
		DownBurst: binary.LittleEndian.Uint64(b[24:32]),
	}
}

func formatRate(bps uint64) string {
	mbit := float64(bps) / 125000.0
	if mbit >= 1 {
		if mbit == float64(int(mbit)) {
			return fmt.Sprintf("%.0fmbit", mbit)
		}
		return fmt.Sprintf("%.1fmbit", mbit)
	}
	return fmt.Sprintf("%dbps", bps)
}

func encodeRates(r ipRates) []byte {
	b := make([]byte, 32)
	binary.LittleEndian.PutUint64(b[0:8], r.UpBps)
	binary.LittleEndian.PutUint64(b[8:16], r.UpBurst)
	binary.LittleEndian.PutUint64(b[16:24], r.DownBps)
	binary.LittleEndian.PutUint64(b[24:32], r.DownBurst)
	return b
}

func doAttach(iface, down, up string) error {
	obj := bpfObject
	if _, err := os.Stat(obj); err != nil {
		obj = "/opt/nat-qos-bpf/build/ratelimit.bpf.o"
	}
	if _, err := os.Stat(obj); err != nil {
		return fmt.Errorf("找不到 %s", bpfObject)
	}

	_ = doDetach(iface)

	objs, err := loadBPFObjects(obj)
	if err != nil {
		return fmt.Errorf("加载 BPF: %w", err)
	}
	defer closeBPFObjects(objs)

	if err := pinBPFObjects(objs); err != nil {
		return err
	}

	run("tc", "qdisc", "add", "dev", iface, "clsact")
	ingressPin := pinDir + "/tc_ingress"
	egressPin := pinDir + "/tc_egress"
	if msg, err := run("tc", "filter", "add", "dev", iface, "ingress", "bpf",
		"direct-action", "object-pinned", ingressPin); err != nil {
		return fmt.Errorf("ingress bpf: %s %w", msg, err)
	}
	if msg, err := run("tc", "filter", "add", "dev", iface, "egress", "bpf",
		"direct-action", "object-pinned", egressPin); err != nil {
		return fmt.Errorf("egress bpf: %s %w", msg, err)
	}

	return setDefault(down, up)
}

func doDetach(iface string) error {
	run("tc", "filter", "del", "dev", iface, "ingress")
	run("tc", "filter", "del", "dev", iface, "egress")
	run("tc", "qdisc", "del", "dev", iface, "clsact")
	for _, p := range []string{configPin, cidrPin, statePin, pinDir + "/tc_ingress", pinDir + "/tc_egress"} {
		os.Remove(p)
	}
	return nil
}

func setCIDR(cidr, down, up string) error {
	canonical, key, err := normalizeCIDR(cidr)
	if err != nil {
		return err
	}
	r, err := ratesFromStrings(down, up)
	if err != nil {
		return err
	}
	m, err := openCidrMap()
	if err != nil {
		return err
	}
	defer m.Close()
	if err := m.Update(&key, encodeRates(r), ebpf.UpdateAny); err != nil {
		return err
	}
	flushAllState()
	fmt.Printf("已设置网段 %s 限速: 下行 %s 上行 %s\n", canonical, down, up)
	return nil
}

func delCIDR(cidr string) error {
	_, key, err := normalizeCIDR(cidr)
	if err != nil {
		return err
	}
	m, err := openCidrMap()
	if err != nil {
		return err
	}
	defer m.Close()
	return m.Delete(&key)
}

func listLimits() error {
	if _, err := os.Stat(configPin); err != nil {
		return fmt.Errorf("eBPF 未加载")
	}
	cm, err := openConfigMap()
	if err != nil {
		return err
	}
	defer cm.Close()

	fmt.Println("=== 全局默认限速 ===")
	var defKey uint32
	var defVal []byte
	if err := cm.Lookup(&defKey, &defVal); err == nil {
		r := decodeRates(defVal)
		fmt.Printf("  下行 %s  上行 %s\n", formatRate(r.DownBps), formatRate(r.UpBps))
	} else {
		fmt.Println("  (未设置，使用程序内置默认)")
	}

	if _, err := os.Stat(cidrPin); err == nil {
		m, err := openCidrMap()
		if err == nil {
			fmt.Println("=== 网段规则 ===")
			var k LpmKey
			var v []byte
			iter := m.Iterate()
			n := 0
			for iter.Next(&k, &v) {
				ip := net.IP(k.Addr[:])
				r := decodeRates(v)
				fmt.Printf("  %s/%d  下行 %s  上行 %s\n", ip, k.PrefixLen, formatRate(r.DownBps), formatRate(r.UpBps))
				n++
			}
			if n == 0 {
				fmt.Println("  (无)")
			}
			m.Close()
		}
	}

	fmt.Println("=== 单 IP 覆盖 ===")
	var k uint32
	var v []byte
	iter := cm.Iterate()
	n := 0
	for iter.Next(&k, &v) {
		if k == 0 {
			continue
		}
		ip := make(net.IP, 4)
		binary.BigEndian.PutUint32(ip, k)
		r := decodeRates(v)
		fmt.Printf("  %s  下行 %s  上行 %s\n", ip, formatRate(r.DownBps), formatRate(r.UpBps))
		n++
	}
	if n == 0 {
		fmt.Println("  (无)")
	}
	return nil
}

func openStateMap() (*ebpf.Map, error) {
	m, err := ebpf.LoadPinnedMap(statePin, nil)
	if err != nil {
		return nil, fmt.Errorf("state_map 未加载: %w", err)
	}
	return m, nil
}

// 修改限速后清除该 IP 的令牌桶，避免仍按旧 burst/速率计数
func flushStateForIP(ipKey uint32) {
	sm, err := openStateMap()
	if err != nil {
		return
	}
	defer sm.Close()
	for _, dir := range []uint64{0, 1} {
		k := (uint64(ipKey) << 1) | dir
		_ = sm.Delete(&k)
	}
}

func flushAllState() {
	sm, err := openStateMap()
	if err != nil {
		return
	}
	defer sm.Close()
	var k uint64
	var v []byte
	iter := sm.Iterate()
	for iter.Next(&k, &v) {
		_ = sm.Delete(&k)
	}
}

func setDefault(down, up string) error {
	r, err := ratesFromStrings(down, up)
	if err != nil {
		return err
	}
	m, err := openConfigMap()
	if err != nil {
		return err
	}
	defer m.Close()
	var key uint32
	if err := m.Update(&key, encodeRates(r), ebpf.UpdateAny); err != nil {
		return err
	}
	flushAllState()
	return nil
}

func setIP(ip, down, up string) error {
	r, err := ratesFromStrings(down, up)
	if err != nil {
		return err
	}
	key, err := ipToKey(ip)
	if err != nil {
		return err
	}
	m, err := openConfigMap()
	if err != nil {
		return err
	}
	defer m.Close()
	if err := m.Update(&key, encodeRates(r), ebpf.UpdateAny); err != nil {
		return err
	}
	flushStateForIP(key)
	return nil
}

func delIP(ip string) error {
	key, err := ipToKey(ip)
	if err != nil {
		return err
	}
	m, err := openConfigMap()
	if err != nil {
		return err
	}
	defer m.Close()
	_ = m.Delete(&key)
	flushStateForIP(key)
	return nil
}

func showStatus() error {
	if _, err := os.Stat(configPin); err != nil {
		fmt.Println("eBPF 限速: 未加载")
		return nil
	}
	fmt.Println("eBPF 限速: 已加载")
	fmt.Printf("  config_map: %s\n", configPin)
	if _, err := os.Stat(cidrPin); err == nil {
		fmt.Printf("  cidr_map:   %s\n", cidrPin)
	}
	fmt.Printf("  state_map:  %s\n", statePin)
	return nil
}

func run(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
