package ocserv

import (
	"fmt"
	"strings"

	"github.com/hk59775634/qosnat2/internal/store"
)

// SessionVhostUnknown 非 [vhost:domain] 会话（occtl 常为 default 或空）
const SessionVhostUnknown = "unknown"

// SessionRawVhost 从 occtl show users JSON 提取 vhost 原始值
func SessionRawVhost(s map[string]any) string {
	for _, k := range []string{"vhost", "Vhost", "Virtual host", "virtual host"} {
		if v, ok := s[k]; ok {
			return strings.TrimSpace(fmt.Sprint(v))
		}
	}
	return ""
}

// ResolveSessionVhostInfo 将 occtl vhost 映射为展示名与域名；有备注时展示备注，否则展示域名
func ResolveSessionVhostInfo(raw string, vhosts []store.OCServVhost) (label, hostname string) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.EqualFold(raw, "default") {
		return SessionVhostUnknown, ""
	}
	for _, v := range vhosts {
		if !v.Enabled {
			continue
		}
		d := strings.TrimSpace(v.Domain)
		if d == "" || !strings.EqualFold(d, raw) {
			continue
		}
		if c := strings.TrimSpace(v.Comment); c != "" {
			return c, d
		}
		return d, d
	}
	return SessionVhostUnknown, ""
}

// EnrichSessionsVhost 为每条会话附加 vhost 展示字段
func EnrichSessionsVhost(sessions []map[string]any, o store.OCServState) []map[string]any {
	out := make([]map[string]any, len(sessions))
	for i, s := range sessions {
		m := make(map[string]any, len(s)+3)
		for k, v := range s {
			m[k] = v
		}
		raw := SessionRawVhost(s)
		label, hostname := ResolveSessionVhostInfo(raw, o.Vhosts)
		m["vhost_raw"] = raw
		m["vhost_domain"] = label
		if hostname != "" {
			m["vhost_hostname"] = hostname
		}
		out[i] = m
	}
	return out
}
