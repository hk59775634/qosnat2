package dnsmasq

import (
	"strconv"
	"strings"
	"time"
)

// LeaseEntry 解析 dnsmasq.leases 一行
type LeaseEntry struct {
	ExpiresUnix int64  `json:"expires_unix"`
	Expires     string `json:"expires,omitempty"`
	MAC         string `json:"mac"`
	IP          string `json:"ip"`
	Hostname    string `json:"hostname,omitempty"`
	ClientID    string `json:"client_id,omitempty"`
	Family      string `json:"family"` // ipv4 | ipv6
}

// ParseLeases 解析租约文件（跳过空行与 #）
func ParseLeases(raw string) []LeaseEntry {
	var out []LeaseEntry
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		exp, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			continue
		}
		mac := fields[1]
		ip := fields[2]
		host := ""
		if len(fields) > 3 {
			host = fields[3]
		}
		cid := ""
		if len(fields) > 4 {
			cid = fields[4]
		}
		fam := "ipv4"
		if strings.Contains(ip, ":") {
			fam = "ipv6"
		}
		ent := LeaseEntry{
			ExpiresUnix: exp,
			MAC:         mac,
			IP:          ip,
			Hostname:    host,
			ClientID:    cid,
			Family:      fam,
		}
		if exp > 0 {
			ent.Expires = time.Unix(exp, 0).UTC().Format(time.RFC3339)
		}
		out = append(out, ent)
	}
	return out
}
