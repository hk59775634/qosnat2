package ocserv

// 与 ocserv(8) 手册一致：配置文件中「### The following directives do not change with server reload」
// 之上的全局项，以及标记为 (non-reloadable) 的项在 SIGHUP/reload 后不会生效，需重启主进程。
// 参考: https://ocserv.openconnect-vpn.net/ocserv.8.html

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/hk59775634/qosnat2/internal/store"
)

// NonReloadableChangeReasons 比较 prev 与 next 在「非 reload 生效」维度上的差异，返回稳定 reason code（供 UI i18n）。
func NonReloadableChangeReasons(prev, next store.OCServState, managed []store.ManagedCertificate) []string {
	p := normalizeOCServForNRCompare(prev)
	n := normalizeOCServForNRCompare(next)
	var out []string
	if globalAuthFingerprint(p) != globalAuthFingerprint(n) {
		out = append(out, "auth_global")
	}
	if globalAcctFingerprint(p) != globalAcctFingerprint(n) {
		out = append(out, "radius_acct")
	}
	if socketFileFingerprint(p) != socketFileFingerprint(n) {
		out = append(out, "socket_file")
	}
	if tcpUdpFingerprint(p) != tcpUdpFingerprint(n) {
		out = append(out, "listen_ports")
	}
	if globalTLSFingerprint(p, managed) != globalTLSFingerprint(n, managed) {
		out = append(out, "tls_global")
	}
	if p.Advanced.LogLevel != n.Advanced.LogLevel {
		out = append(out, "log_level")
	}
	if p.Advanced.ServerStatsResetTime != n.Advanced.ServerStatsResetTime {
		out = append(out, "server_stats_reset")
	}
	vp := vhostsNRFingerprints(p, managed)
	vn := vhostsNRFingerprints(n, managed)
	if !mapsEqualStringString(vp, vn) {
		// 细分：便于用户理解
		for d := range unionDomainKeys(vp, vn) {
			if vp[d] != vn[d] {
				out = append(out, fmt.Sprintf("vhost:%s", d))
			}
		}
	}
	return dedupeReasons(out)
}

func normalizeOCServForNRCompare(o store.OCServState) store.OCServState {
	o2 := o
	if store.OCServUsesRadius(o2) {
		_ = NormalizeRadiusConfig(&o2)
	}
	store.MergeOCServAdvanced(&o2.Advanced, nil)
	return o2
}

func globalAuthFingerprint(o store.OCServState) string {
	if store.OCServUsesRadius(o) {
		o2 := o
		_ = NormalizeRadiusConfig(&o2)
		r := o2.Radius
		sec := strings.TrimSpace(r.Secret)
		sk := "sec:empty"
		if sec != "" {
			sum := sha256.Sum256([]byte(sec))
			sk = "sec:" + hex.EncodeToString(sum[:8])
		}
		authPort := r.AuthPort
		if authPort <= 0 {
			authPort = 1812
		}
		acctPort := r.AcctPort
		if acctPort <= 0 {
			acctPort = 1813
		}
		host := strings.TrimSpace(r.Server)
		return radiusAuthDirective(o2) + "|rad:" + host + "|authp:" + strconv.Itoa(authPort) + "|acctp:" + strconv.Itoa(acctPort) + "|" + sk
	}
	return fmt.Sprintf("plain:%s", PasswdPath)
}

func globalAcctFingerprint(o store.OCServState) string {
	if !store.OCServUsesRadius(o) {
		return ""
	}
	if !o.Radius.AcctEnabled {
		return "off"
	}
	return "on:" + radiusConfigPath(o)
}

func socketFileFingerprint(o store.OCServState) string {
	s := strings.TrimSpace(o.SocketFile)
	if s == "" {
		s = "/var/run/ocserv-socket"
	}
	return s
}

func tcpUdpFingerprint(o store.OCServState) string {
	a := o.Advanced
	var tcp, udp string
	if a.Tcp {
		tcp = fmt.Sprintf("%d", o.TCPPort)
	} else {
		tcp = "off"
	}
	if a.Udp {
		udp = fmt.Sprintf("%d", o.UDPPort)
	} else {
		udp = "off"
	}
	return tcp + "/" + udp
}

func globalTLSFingerprint(o store.OCServState, managed []store.ManagedCertificate) string {
	cert, key, ca := store.ResolveOCServGlobalCerts(o, managed)
	return cert + "\n" + key + "\n" + ca
}

func vhostsNRFingerprints(o store.OCServState, managed []store.ManagedCertificate) map[string]string {
	m := make(map[string]string)
	for _, v := range o.Vhosts {
		d := strings.TrimSpace(v.Domain)
		if d == "" || !v.Enabled {
			continue
		}
		m[strings.ToLower(d)] = vhostNRFingerprint(v, o, managed)
	}
	return m
}

func vhostNRFingerprint(v store.OCServVhost, global store.OCServState, managed []store.ManagedCertificate) string {
	var b strings.Builder
	b.WriteString(vhostAuthFingerprint(v, global))
	b.WriteByte('\n')
	cert, key, ca := store.ResolveOCServVhostCerts(v, global, managed)
	fmt.Fprintf(&b, "%s|%s|%s\n", cert, key, ca)
	fmt.Fprintf(&b, "dh:%s\ncrl:%s\n", strings.TrimSpace(v.DHParamsPath), strings.TrimSpace(v.CRLPath))
	b.WriteString(vhostAcctFingerprint(v, global))
	return b.String()
}

func vhostAuthFingerprint(v store.OCServVhost, global store.OCServState) string {
	auth := strings.TrimSpace(v.AuthMethod)
	if auth == "" {
		auth = strings.TrimSpace(global.AuthMethod)
	}
	switch auth {
	case store.OCServAuthPlain:
		passwd := PasswdPath
		if p := strings.TrimSpace(v.PlainPasswdPath); p != "" {
			passwd = p
		}
		return fmt.Sprintf("plain:%s", passwd)
	case store.OCServAuthRadius:
		return vhostRadiusAuthFingerprint(v, global)
	case "certificate":
		return "certificate"
	default:
		return auth
	}
}

func vhostRadiusAuthFingerprint(v store.OCServVhost, global store.OCServState) string {
	if store.VhostUsesOwnRadius(v) {
		cfg := vhostRadcliConfPath(v.Domain)
		if p := strings.TrimSpace(v.Radius.ConfigPath); p != "" {
			cfg = p
		}
		opts := []string{"config=" + cfg}
		if v.Radius.GroupConfig {
			opts = append(opts, "groupconfig=true")
		}
		if nas := strings.TrimSpace(v.Radius.NASIdentifier); nas != "" {
			opts = append(opts, "nas-identifier="+nas)
		}
		return fmt.Sprintf(`radius[%s]`, strings.Join(opts, ","))
	}
	return "inherit:" + radiusAuthDirective(global)
}

func vhostAcctFingerprint(v store.OCServVhost, global store.OCServState) string {
	acct := global.Radius.AcctEnabled
	if store.VhostUsesOwnRadius(v) {
		acct = v.Radius.AcctEnabled
	} else if v.AcctEnabled {
		acct = true
	}
	if !acct {
		return "acct:off"
	}
	cfg := radiusConfigPath(global)
	if store.VhostUsesOwnRadius(v) {
		cfg = vhostRadcliConfPath(v.Domain)
		if p := strings.TrimSpace(v.Radius.ConfigPath); p != "" {
			cfg = p
		}
	}
	return "acct:on:" + cfg
}

func unionDomainKeys(a, b map[string]string) map[string]struct{} {
	u := make(map[string]struct{})
	for k := range a {
		u[k] = struct{}{}
	}
	for k := range b {
		u[k] = struct{}{}
	}
	return u
}

func mapsEqualStringString(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func dedupeReasons(in []string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, s := range in {
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
