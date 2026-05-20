package store

import (
	"fmt"
	"net"
	"sort"
)

// ProfileHostIP 若 CIDR 为 /32 返回主机 IP
func ProfileHostIP(cidr string) (string, bool) {
	ip, n, err := net.ParseCIDR(cidr)
	if err != nil || n == nil {
		return "", false
	}
	ones, bits := n.Mask.Size()
	if ones != bits || bits != 32 {
		return "", false
	}
	return ip.String(), true
}

// Host32ProfileCIDR 由主机 IP 构造 /32 模板键
func Host32ProfileCIDR(ip string) string {
	return ip + "/32"
}

// MigrateHostsToProfiles 将旧版 VIP host_exact 配置迁入 profiles
func MigrateHostsToProfiles(profiles *[]ProfileEntry, hosts map[string]HostRate) {
	if len(hosts) == 0 {
		return
	}
	seen := make(map[string]struct{}, len(*profiles))
	for _, p := range *profiles {
		seen[p.CIDR] = struct{}{}
	}
	for ip, h := range hosts {
		cidr := Host32ProfileCIDR(ip)
		if _, ok := seen[cidr]; ok {
			continue
		}
		*profiles = append(*profiles, ProfileEntry{
			CIDR: cidr, Down: h.Down, Up: h.Up, Mask: 32,
			ID: NextProfileID(*profiles),
		})
		seen[cidr] = struct{}{}
	}
}

// ProfileHost32IPs 所有 /32 模板对应的主机 IP（供 GC 保留）
func ProfileHost32IPs(profiles []ProfileEntry) map[string]bool {
	m := make(map[string]bool)
	for _, p := range profiles {
		if ip, ok := ProfileHostIP(p.CIDR); ok {
			m[ip] = true
		}
	}
	return m
}

// MigrateProfilePriorityToID 将旧版 priority 字段迁入 id
func MigrateProfilePriorityToID(profiles *[]ProfileEntry) {
	for i := range *profiles {
		p := &(*profiles)[i]
		if p.ID <= 0 && p.Priority > 0 {
			p.ID = p.Priority
		}
		p.Priority = 0
	}
}

// SortProfilesByID id 越小优先级越高（排在前）
func SortProfilesByID(profiles []ProfileEntry) []ProfileEntry {
	out := make([]ProfileEntry, len(profiles))
	copy(out, profiles)
	sort.Slice(out, func(i, j int) bool {
		if out[i].ID != out[j].ID {
			return out[i].ID < out[j].ID
		}
		return out[i].CIDR < out[j].CIDR
	})
	return out
}

// NormalizeProfileIDs 为无 id 的条目按当前切片顺序赋 1,2,…
func NormalizeProfileIDs(profiles *[]ProfileEntry) {
	MigrateProfilePriorityToID(profiles)
	if len(*profiles) == 0 {
		return
	}
	need := false
	for _, p := range *profiles {
		if p.ID <= 0 {
			need = true
			break
		}
	}
	if !need {
		return
	}
	for i := range *profiles {
		if (*profiles)[i].ID <= 0 {
			(*profiles)[i].ID = i + 1
		}
	}
}

// NextProfileID 新规则默认 id = max(id)+1
func NextProfileID(profiles []ProfileEntry) int {
	max := 0
	for _, p := range profiles {
		if p.ID > max {
			max = p.ID
		}
	}
	return max + 1
}

// ReorderProfiles 按 cidr 顺序重写 id（1 最高优先级）
func ReorderProfiles(profiles []ProfileEntry, order []string) ([]ProfileEntry, error) {
	byCIDR := make(map[string]ProfileEntry, len(profiles))
	for _, p := range profiles {
		byCIDR[p.CIDR] = p
	}
	seen := make(map[string]struct{}, len(order))
	var out []ProfileEntry
	for i, cidr := range order {
		p, ok := byCIDR[cidr]
		if !ok {
			return nil, fmt.Errorf("unknown profile cidr: %s", cidr)
		}
		if _, dup := seen[cidr]; dup {
			return nil, fmt.Errorf("duplicate cidr in order: %s", cidr)
		}
		seen[cidr] = struct{}{}
		p.ID = i + 1
		out = append(out, p)
	}
	for _, p := range profiles {
		if _, ok := seen[p.CIDR]; ok {
			continue
		}
		p.ID = len(out) + 1
		out = append(out, p)
	}
	return out, nil
}

// SortProfilesByPriority 兼容旧调用
func SortProfilesByPriority(profiles []ProfileEntry) []ProfileEntry {
	return SortProfilesByID(profiles)
}

// NormalizeProfilePriorities 兼容旧调用
func NormalizeProfilePriorities(profiles *[]ProfileEntry) {
	NormalizeProfileIDs(profiles)
}

// NextProfilePriority 兼容旧调用
func NextProfilePriority(profiles []ProfileEntry) int {
	return NextProfileID(profiles)
}
