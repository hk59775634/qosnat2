package api

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/hk59775634/qosnat2/internal/route"
	"github.com/hk59775634/qosnat2/internal/store"
	"github.com/hk59775634/qosnat2/internal/warpnetns"
)

const warpConsumerOverridesPath = "/var/lib/cloudflare-warp/consumer_overrides.json"

// warpConsumerOverrides 将 WARP 设为 Include 分流，仅 Cloudflare CDN 网段走隧道（不劫持默认出口）。
type warpConsumerOverrides struct {
	OperationMode       string           `json:"operation_mode,omitempty"`
	DisableAutoFallback *bool            `json:"disable_auto_fallback,omitempty"`
	SplitConfig         *warpSplitConfig `json:"split_config,omitempty"`
}

type warpSplitConfig struct {
	Mode string        `json:"mode"`
	IPs  []warpSplitIP `json:"ips"`
}

type warpSplitIP struct {
	IP          string `json:"ip"`
	Description string `json:"description,omitempty"`
}

// prepareWarpPolicyOnly 已弃用：WARP 改在 warpnetns 中运行。保留空实现供兼容。
func prepareWarpPolicyOnly() error {
	return nil
}

func writeWarpIncludeSplitTunnel() error {
	ips := make([]warpSplitIP, 0, len(store.CloudflareCDNCIDRsV4())+8)
	for _, cidr := range store.CloudflareCDNCIDRsV4() {
		ips = append(ips, warpSplitIP{IP: cidr, Description: "qosnat2 cloudflare cdn"})
	}
	// WARP 隧道端点，避免 Include 模式下无法建连
	for _, cidr := range []string{
		"162.159.192.0/24",
		"162.159.198.0/24",
		"2606:4700::/32",
	} {
		ips = append(ips, warpSplitIP{IP: cidr, Description: "qosnat2 warp endpoint"})
	}
	fallback := true
	ov := warpConsumerOverrides{
		OperationMode:       "Warp",
		DisableAutoFallback: &fallback,
		SplitConfig: &warpSplitConfig{
			Mode: "Include",
			IPs:  ips,
		},
	}
	b, err := json.MarshalIndent(ov, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(warpConsumerOverridesPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(warpConsumerOverridesPath, b, 0644)
}

// excludeServerPublicIPsFromWarp 在仍为 Exclude 模式时，将本机公网地址加入排除列表，避免 SSH/管理流量进隧道。
func excludeServerPublicIPsFromWarp() {
	for _, cidr := range collectServerPublicIPCIDRs() {
		_, _ = exec.Command("warp-cli", "--accept-tos", "tunnel", "ip", "add-range", cidr).CombinedOutput()
	}
}

func collectServerPublicIPCIDRs() []string {
	out, err := exec.Command("bash", "-lc", `ip -4 -o addr show scope global | awk '{print $4}'`).Output()
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var cidrs []string
	for _, line := range strings.Split(string(out), "\n") {
		cidr := strings.TrimSpace(line)
		if cidr == "" || seen[cidr] {
			continue
		}
		seen[cidr] = true
		cidrs = append(cidrs, cidr)
	}
	return cidrs
}

// restoreRoutesAfterWarpConnect 连接后移除 WARP 写入的 default 路由，并回放 qosnat2 托管的主表默认路由。
func restoreRoutesAfterWarpConnect(srv *Server) error {
	iface := warpnetns.HostInterface()
	if iface == "" {
		iface = detectWarpInterface()
	}
	removeDefaultRoutesViaDevice(iface)
	_, _ = exec.Command("ip", "netns", "exec", warpnetns.NetnsName, "warp-cli", "--accept-tos", "override", "local-network", "allow").CombinedOutput()
	st := srv.store.Get()
	sync := st
	store.SyncWanRoutes(&sync)
	var applyErr error
	for _, r := range sync.Routes {
		if !r.Enabled {
			continue
		}
		dest := strings.TrimSpace(r.Dest)
		if dest != "default" && dest != "0.0.0.0/0" {
			continue
		}
		if r.Table != 0 && r.Table != 254 {
			continue
		}
		dev := strings.TrimSpace(r.Device)
		if dev != "" && strings.EqualFold(dev, iface) {
			continue
		}
		if err := route.Apply(r); err != nil {
			applyErr = err
		}
	}
	return applyErr
}

func removeDefaultRoutesViaDevice(dev string) {
	if dev == "" {
		return
	}
	for i := 0; i < 16; i++ {
		out, err := exec.Command("ip", "route", "del", "default", "dev", dev).CombinedOutput()
		if err != nil {
			msg := strings.ToLower(string(out))
			if strings.Contains(msg, "not found") || strings.Contains(msg, "no such process") {
				break
			}
		}
	}
	// 部分版本经 nexthop 写入 default
	for i := 0; i < 8; i++ {
		out, _ := exec.Command("bash", "-lc",
			fmt.Sprintf(`ip -4 route show default | while read -r line; do
  case "$line" in *"dev %s"*) ip route del $line 2>/dev/null ;; esac
done`, dev)).Output()
		if strings.TrimSpace(string(out)) == "" {
			break
		}
	}
}
