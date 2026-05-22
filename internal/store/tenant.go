package store

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
)

// TenantEntry 租户 QoS：一组 CIDR 共享上下行模板（展开为多条 profile_lpm + mirred）
type TenantEntry struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	CIDRs  []string `json:"cidrs"`
	Down   string   `json:"down"`
	Up     string   `json:"up"`
	Device string   `json:"device,omitempty"`
}

func NewTenantID() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return "tenant-" + hex.EncodeToString(b[:])
}

// NormalizeTenant 校验租户并规范化 CIDR 列表
func NormalizeTenant(t *TenantEntry) error {
	if t == nil {
		return fmt.Errorf("tenant nil")
	}
	if strings.TrimSpace(t.ID) == "" {
		t.ID = NewTenantID()
	}
	t.Name = strings.TrimSpace(t.Name)
	if t.Name == "" {
		return fmt.Errorf("name required")
	}
	if strings.TrimSpace(t.Down) == "" {
		t.Down = "8mbit"
	}
	if strings.TrimSpace(t.Up) == "" {
		t.Up = "8mbit"
	}
	var cidrs []string
	for _, c := range t.CIDRs {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		if _, _, err := net.ParseCIDR(c); err != nil {
			return fmt.Errorf("invalid cidr %q", c)
		}
		cidrs = append(cidrs, c)
	}
	if len(cidrs) == 0 {
		return fmt.Errorf("at least one cidr required")
	}
	t.CIDRs = cidrs
	return nil
}

// TenantProfileTag 写入 profile 的 tenant 归属标记（存于 Device 字段后缀不可用，用 CIDR 映射）
func TenantProfileKey(tenantID, cidr string) string {
	return tenantID + "|" + cidr
}
