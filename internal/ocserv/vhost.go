package ocserv

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/hk59775634/qosnat2/internal/store"
)

func vhostRadcliConfPath(domain string) string {
	return filepath.Join(RadcliDir, "vhosts", safeVhostFileName(domain)+".conf")
}

func safeVhostFileName(domain string) string {
	d := strings.TrimSpace(strings.ToLower(domain))
	d = strings.ReplaceAll(d, "..", "")
	return strings.ReplaceAll(d, ".", "_")
}

// WriteVhostRadcliConfig writes per-vhost radcli files when vhost has its own RADIUS server.
func WriteVhostRadcliConfig(v store.OCServVhost) error {
	if !store.VhostUsesOwnRadius(v) {
		return nil
	}
	r := *v.Radius
	if strings.TrimSpace(r.ConfigPath) == "" {
		r.ConfigPath = vhostRadcliConfPath(v.Domain)
	}
	o := store.OCServState{AuthMethod: store.OCServAuthRadius, Radius: r}
	return WriteRadcliConfig(o)
}

// SyncVhostRadcliConfigs writes radcli for all vhosts with dedicated RADIUS.
func SyncVhostRadcliConfigs(vhosts []store.OCServVhost) error {
	for _, v := range vhosts {
		if !v.Enabled || !store.VhostUsesOwnRadius(v) {
			continue
		}
		if err := WriteVhostRadcliConfig(v); err != nil {
			return err
		}
	}
	return nil
}

func vhostRadiusAuthDirective(v store.OCServVhost, global store.OCServState) string {
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
		return fmt.Sprintf(`auth = "radius[%s]"`, strings.Join(opts, ","))
	}
	return radiusAuthDirective(global)
}

func renderVhostBlock(b *bytes.Buffer, v store.OCServVhost, global store.OCServState, managed []store.ManagedCertificate) {
	domain := strings.TrimSpace(v.Domain)
	if domain == "" {
		return
	}
	fmt.Fprintf(b, "\n[vhost:%s]\n", domain)

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
		fmt.Fprintf(b, "auth = \"plain[passwd=%s]\"\n", passwd)
	case store.OCServAuthRadius:
		b.WriteString(vhostRadiusAuthDirective(v, global))
		b.WriteByte('\n')
	case "certificate":
		b.WriteString("auth = \"certificate\"\n")
	}

	cert, key, ca := store.ResolveOCServVhostCerts(v, global, managed)
	if cert != "" {
		fmt.Fprintf(b, "server-cert = %s\n", cert)
	}
	if key != "" {
		fmt.Fprintf(b, "server-key = %s\n", key)
	}
	if ca != "" {
		fmt.Fprintf(b, "ca-cert = %s\n", ca)
	}
	if p := strings.TrimSpace(v.CRLPath); p != "" {
		fmt.Fprintf(b, "crl = %s\n", p)
	}
	if p := strings.TrimSpace(v.DHParamsPath); p != "" {
		fmt.Fprintf(b, "dh-params = %s\n", p)
	}
	if p := strings.TrimSpace(v.TLSPriorities); p != "" {
		appendQuotedLine(b, "tls-priorities", p)
	}
	if oid := strings.TrimSpace(v.CertUserOID); oid != "" {
		fmt.Fprintf(b, "cert-user-oid = %s\n", oid)
	}
	if oid := strings.TrimSpace(v.CertGroupOID); oid != "" {
		fmt.Fprintf(b, "cert-group-oid = %s\n", oid)
	}

	if n := strings.TrimSpace(v.IPv4Network); n != "" {
		fmt.Fprintf(b, "ipv4-network = %s\n", n)
	}
	if m := strings.TrimSpace(v.IPv4Netmask); m != "" {
		fmt.Fprintf(b, "ipv4-netmask = %s\n", m)
	}
	if n := strings.TrimSpace(v.IPv6Network); n != "" {
		if v.IPv6Prefix > 0 {
			fmt.Fprintf(b, "ipv6-network = %s/%d\n", n, v.IPv6Prefix)
		} else {
			fmt.Fprintf(b, "ipv6-network = %s\n", n)
		}
	}
	for _, d := range v.DNS {
		fmt.Fprintf(b, "dns = %s\n", d)
	}
	for _, n := range v.NBNS {
		fmt.Fprintf(b, "nbns = %s\n", n)
	}
	if dom := strings.TrimSpace(v.DefaultDomain); dom != "" {
		fmt.Fprintf(b, "default-domain = %s\n", dom)
	}
	if v.TunnelAllDNS {
		appendBoolLine(b, "tunnel-all-dns", true)
	}
	if v.MTU > 0 {
		fmt.Fprintf(b, "mtu = %d\n", v.MTU)
	}
	for _, r := range v.Routes {
		fmt.Fprintf(b, "route = %s\n", r)
	}
	for _, r := range v.NoRoutes {
		fmt.Fprintf(b, "no-route = %s\n", r)
	}
	for _, r := range v.IRoutes {
		fmt.Fprintf(b, "iroute = %s\n", r)
	}
	if v.ExposeIRoutes {
		appendBoolLine(b, "expose-iroutes", true)
	}
	if v.RxDataPerSec > 0 {
		fmt.Fprintf(b, "rx-data-per-sec = %d\n", v.RxDataPerSec)
	}
	if v.TxDataPerSec > 0 {
		fmt.Fprintf(b, "tx-data-per-sec = %d\n", v.TxDataPerSec)
	}
	if v.PktMTUSize > 0 {
		fmt.Fprintf(b, "pkt-mtu-size = %d\n", v.PktMTUSize)
	}
	if v.IdleTimeout > 0 {
		fmt.Fprintf(b, "idle-timeout = %d\n", v.IdleTimeout)
	}
	if v.SessionTimeout > 0 {
		fmt.Fprintf(b, "session-timeout = %d\n", v.SessionTimeout)
	}
	if v.MobileIdleTimeout > 0 {
		fmt.Fprintf(b, "mobile-idle-timeout = %d\n", v.MobileIdleTimeout)
	}
	if v.MaxSameClients > 0 {
		fmt.Fprintf(b, "max-same-clients = %d\n", v.MaxSameClients)
	}
	if v.Keepalive > 0 {
		fmt.Fprintf(b, "keepalive = %d\n", v.Keepalive)
	}
	if v.DPD > 0 {
		fmt.Fprintf(b, "dpd = %d\n", v.DPD)
	}
	if v.MobileDPD > 0 {
		fmt.Fprintf(b, "mobile-dpd = %d\n", v.MobileDPD)
	}
	if v.CookieTimeout > 0 {
		fmt.Fprintf(b, "cookie-timeout = %d\n", v.CookieTimeout)
	}
	if v.DenyRoaming {
		appendBoolLine(b, "deny-roaming", true)
	}
	if v.PersistentCookies {
		appendBoolLine(b, "persistent-cookies", true)
	}
	if v.RekeyTime > 0 {
		fmt.Fprintf(b, "rekey-time = %d\n", v.RekeyTime)
	}
	if rm := strings.TrimSpace(v.RekeyMethod); rm != "" {
		fmt.Fprintf(b, "rekey-method = %s\n", rm)
	}
	if v.Compression {
		appendBoolLine(b, "compression", true)
	}
	if v.PredictableIPs {
		appendBoolLine(b, "predictable-ips", true)
	}
	if v.DtlsLegacy {
		appendBoolLine(b, "dtls-legacy", true)
	}
	if v.CiscoClientCompat {
		appendBoolLine(b, "cisco-client-compat", true)
	}
	if v.CiscoSvcCompat {
		appendBoolLine(b, "cisco-svc-client-compat", true)
	}
	if v.NoUDP {
		appendBoolLine(b, "no-udp", true)
	}
	if s := strings.TrimSpace(v.Banner); s != "" {
		appendQuotedLine(b, "banner", s)
	}
	if s := strings.TrimSpace(v.PreLoginBanner); s != "" {
		appendQuotedLine(b, "pre-login-banner", s)
	}
	if v.Camouflage {
		appendBoolLine(b, "camouflage", true)
		if s := strings.TrimSpace(v.CamouflageSecret); s != "" {
			appendQuotedLine(b, "camouflage_secret", s)
		}
		if r := strings.TrimSpace(v.CamouflageRealm); r != "" {
			appendQuotedLine(b, "camouflage_realm", r)
		}
	}
	if p := strings.TrimSpace(v.ConfigPerUser); p != "" {
		fmt.Fprintf(b, "config-per-user = %s\n", p)
	}
	if p := strings.TrimSpace(v.ConfigPerGroup); p != "" {
		fmt.Fprintf(b, "config-per-group = %s\n", p)
	}
	if p := strings.TrimSpace(v.DefaultUserConfig); p != "" {
		fmt.Fprintf(b, "default-user-config = %s\n", p)
	}
	if p := strings.TrimSpace(v.DefaultGroupConfig); p != "" {
		fmt.Fprintf(b, "default-group-config = %s\n", p)
	}
	for _, g := range v.SelectGroups {
		g = strings.TrimSpace(g)
		if g == "" {
			continue
		}
		fmt.Fprintf(b, "select-group = %s\n", g)
	}
	if v.AutoSelectGroup {
		appendBoolLine(b, "auto-select-group", true)
	}
	if g := strings.TrimSpace(v.DefaultSelectGroup); g != "" {
		fmt.Fprintf(b, "default-select-group = %s\n", g)
	}
	if s := strings.TrimSpace(v.ConnectScript); s != "" {
		appendQuotedLine(b, "connect-script", s)
	}
	if s := strings.TrimSpace(v.DisconnectScript); s != "" {
		appendQuotedLine(b, "disconnect-script", s)
	}

	acct := global.Radius.AcctEnabled
	stats := global.Radius.StatsReportTime
	if store.VhostUsesOwnRadius(v) {
		acct = v.Radius.AcctEnabled
		stats = v.Radius.StatsReportTime
	} else if v.AcctEnabled {
		acct = true
	}
	if v.StatsReportTime > 0 {
		stats = v.StatsReportTime
	}
	if acct {
		cfg := radiusConfigPath(global)
		if store.VhostUsesOwnRadius(v) {
			cfg = vhostRadcliConfPath(v.Domain)
			if p := strings.TrimSpace(v.Radius.ConfigPath); p != "" {
				cfg = p
			}
		}
		fmt.Fprintf(b, "acct = \"radius[config=%s]\"\n", cfg)
		if stats > 0 {
			fmt.Fprintf(b, "stats-report-time = %d\n", stats)
		}
	}
}
