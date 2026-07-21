package netif

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/hk59775634/qosnat2/internal/store"
)

// EnsureAddrOnDev 确保地址以 CIDR 形式存在于网卡上（已存在则跳过）。
func EnsureAddrOnDev(dev, cidr string) error {
	dev = strings.TrimSpace(dev)
	cidr = strings.TrimSpace(cidr)
	if dev == "" || cidr == "" {
		return fmt.Errorf("device and cidr required")
	}
	if !LinkExists(dev) {
		return fmt.Errorf("interface %s not found", dev)
	}
	host := strings.SplitN(cidr, "/", 2)[0]
	out, _ := exec.Command("ip", "-4", "addr", "show", "dev", dev).CombinedOutput()
	s := string(out)
	if strings.Contains(s, cidr) || strings.Contains(s, host+"/") || strings.Contains(s, " "+host+" ") {
		return nil
	}
	if out, err := exec.Command("ip", "addr", "add", cidr, "dev", dev).CombinedOutput(); err != nil {
		msg := strings.TrimSpace(string(out))
		if strings.Contains(msg, "File exists") {
			return nil
		}
		return fmt.Errorf("ip addr add %s dev %s: %s %w", cidr, dev, msg, err)
	}
	return nil
}

// RemoveAddrFromDev 从网卡移除地址（不存在则忽略）。
func RemoveAddrFromDev(dev, cidr string) error {
	dev = strings.TrimSpace(dev)
	cidr = strings.TrimSpace(cidr)
	if dev == "" || cidr == "" {
		return nil
	}
	if !LinkExists(dev) {
		return nil
	}
	if out, err := exec.Command("ip", "addr", "del", cidr, "dev", dev).CombinedOutput(); err != nil {
		msg := strings.TrimSpace(string(out))
		if strings.Contains(msg, "Cannot assign") || strings.Contains(msg, "Address not found") ||
			strings.Contains(msg, "No such") || strings.Contains(msg, "not found") {
			return nil
		}
		// 尝试仅主机 /32
		host := strings.SplitN(cidr, "/", 2)[0]
		if host != cidr {
			if out2, err2 := exec.Command("ip", "addr", "del", host+"/32", "dev", dev).CombinedOutput(); err2 == nil {
				return nil
			} else {
				msg2 := strings.TrimSpace(string(out2))
				if strings.Contains(msg2, "Cannot assign") || strings.Contains(msg2, "not found") {
					return nil
				}
			}
		}
		return fmt.Errorf("ip addr del %s dev %s: %s %w", cidr, dev, msg, err)
	}
	return nil
}

// ApplyVirtualIPs 将启用的虚拟 IP（IP Alias）绑定到接口；禁用项尝试移除。
// 与 netplan 托管地址并存：已由 IfaceConfig 声明的地址不会被禁用项删除。
func ApplyVirtualIPs(net store.NetworkState) error {
	managed := ifaceManagedHosts(net.Ifaces)
	var errs []string
	for _, v := range net.VirtualIPs {
		dev := strings.TrimSpace(v.Interface)
		cidr, host, err := store.NormalizeVirtualIPAddress(v.Address)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", v.ID, err))
			continue
		}
		if !v.Enabled {
			if managed[dev+"|"+host] {
				continue
			}
			if err := RemoveAddrFromDev(dev, cidr); err != nil {
				errs = append(errs, err.Error())
			}
			continue
		}
		if err := EnsureAddrOnDev(dev, cidr); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("virtual ips: %s", strings.Join(errs, "; "))
	}
	return nil
}

func ifaceManagedHosts(ifaces []store.IfaceConfig) map[string]bool {
	out := map[string]bool{}
	for _, ic := range ifaces {
		dev := strings.TrimSpace(ic.Device)
		for _, a := range ic.IPv4 {
			_, host, err := store.NormalizeVirtualIPAddress(a)
			if err != nil {
				host = strings.SplitN(strings.TrimSpace(a), "/", 2)[0]
			}
			if dev != "" && host != "" {
				out[dev+"|"+host] = true
			}
		}
	}
	return out
}

// MergeVirtualIPsIntoIfaces 将启用 VIP 合并进 netplan 用的 IfaceConfig 副本（不改 store）。
// 仅合并到已存在的托管物理口，避免为 DHCP WAN 新建 ethernets 段。
func MergeVirtualIPsIntoIfaces(ifaces []store.IfaceConfig, vips []store.VirtualIP) []store.IfaceConfig {
	if len(vips) == 0 {
		return ifaces
	}
	out := make([]store.IfaceConfig, len(ifaces))
	copy(out, ifaces)
	idx := map[string]int{}
	for i, ic := range out {
		idx[ic.Device] = i
	}
	for _, v := range vips {
		if !v.Enabled {
			continue
		}
		typ := strings.ToLower(strings.TrimSpace(v.Type))
		if typ == "" {
			typ = store.VirtualIPTypeIPAlias
		}
		if typ != store.VirtualIPTypeIPAlias {
			continue
		}
		dev := strings.TrimSpace(v.Interface)
		i, ok := idx[dev]
		if !ok {
			continue
		}
		cidr, _, err := store.NormalizeVirtualIPAddress(v.Address)
		if err != nil {
			continue
		}
		if containsCIDR(out[i].IPv4, cidr) {
			continue
		}
		out[i].IPv4 = append(append([]string(nil), out[i].IPv4...), cidr)
	}
	return out
}

func containsCIDR(list []string, cidr string) bool {
	_, host, _ := store.NormalizeVirtualIPAddress(cidr)
	for _, a := range list {
		if strings.TrimSpace(a) == cidr {
			return true
		}
		_, h, err := store.NormalizeVirtualIPAddress(a)
		if err == nil && h == host {
			return true
		}
	}
	return false
}
