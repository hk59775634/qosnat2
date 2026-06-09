package ocserv

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/hk59775634/qosnat2/internal/store"
)

func appendBoolLine(b *bytes.Buffer, key string, val bool) {
	fmt.Fprintf(b, "%s = %t\n", key, val)
}

func appendQuotedLine(b *bytes.Buffer, key, val string) {
	val = strings.ReplaceAll(val, `"`, `\"`)
	fmt.Fprintf(b, "%s = \"%s\"\n", key, val)
}

func renderAdvanced(b *bytes.Buffer, o store.OCServState) {
	a := o.Advanced
	if a.Tcp {
		fmt.Fprintf(b, "tcp-port = %d\n", o.TCPPort)
	}
	if a.Udp {
		fmt.Fprintf(b, "udp-port = %d\n", o.UDPPort)
	}
	appendBoolLine(b, "try-mtu-discovery", a.TryMTUDiscovery)
	appendBoolLine(b, "isolate-workers", a.IsolateWorkers)
	appendBoolLine(b, "dtls-legacy", a.DtlsLegacy)
	appendBoolLine(b, "deny-roaming", a.DenyRoaming)
	appendBoolLine(b, "cisco-client-compat", a.CiscoClientCompat)
	appendBoolLine(b, "cisco-svc-client-compat", a.CiscoSvcCompat)
	appendBoolLine(b, "select-group-by-url", a.SelectGroupByURL)
	appendBoolLine(b, "client-bypass-protocol", a.ClientBypassProto)
	appendBoolLine(b, "compression", a.Compression)
	appendBoolLine(b, "predictable-ips", a.PredictableIPs)
	appendBoolLine(b, "ping-leases", a.PingLeases)
	appendBoolLine(b, "use-occtl", a.UseOcctl)

	if a.RateLimitMs > 0 {
		fmt.Fprintf(b, "rate-limit-ms = %d\n", a.RateLimitMs)
	}
	if a.ServerStatsResetTime > 0 {
		fmt.Fprintf(b, "server-stats-reset-time = %d\n", a.ServerStatsResetTime)
	}
	if a.LogLevel >= 0 {
		fmt.Fprintf(b, "log-level = %d\n", a.LogLevel)
	}
	if a.MaxBanScore > 0 {
		fmt.Fprintf(b, "max-ban-score = %d\n", a.MaxBanScore)
	}
	if a.BanTime > 0 {
		fmt.Fprintf(b, "ban-time = %d\n", a.BanTime)
	}
	if a.BanResetTime > 0 {
		fmt.Fprintf(b, "ban-reset-time = %d\n", a.BanResetTime)
	}

	if a.Keepalive {
		fmt.Fprintf(b, "keepalive = %d\n", a.KeepaliveSec)
	}
	if a.DPD {
		fmt.Fprintf(b, "dpd = %d\n", a.DPDSec)
	}
	if a.MobileDPD {
		fmt.Fprintf(b, "mobile-dpd = %d\n", a.MobileDPDSec)
	}
	if a.SwitchToTcp {
		fmt.Fprintf(b, "switch-to-tcp-timeout = %d\n", a.SwitchToTcpTimeout)
	}
	fmt.Fprintf(b, "cookie-timeout = %d\n", a.CookieTimeout)
	fmt.Fprintf(b, "auth-timeout = %d\n", a.AuthTimeout)
	if a.Rekey {
		fmt.Fprintf(b, "rekey-time = %d\n", a.RekeyTime)
		method := strings.TrimSpace(a.RekeyMethod)
		if method == "" {
			method = "ssl"
		}
		fmt.Fprintf(b, "rekey-method = %s\n", method)
	}
	fmt.Fprintf(b, "max-same-clients = %d\n", a.MaxSameClients)

	if strings.TrimSpace(a.CertUserOID) != "" {
		fmt.Fprintf(b, "cert-user-oid = %s\n", strings.TrimSpace(a.CertUserOID))
	}
	if strings.TrimSpace(a.TLSPriorities) != "" {
		appendQuotedLine(b, "tls-priorities", strings.TrimSpace(a.TLSPriorities))
	}
	if strings.TrimSpace(a.DefaultDomain) != "" {
		fmt.Fprintf(b, "default-domain = %s\n", strings.TrimSpace(a.DefaultDomain))
	}
	if a.RxDataPerSec > 0 {
		fmt.Fprintf(b, "rx-data-per-sec = %d\n", a.RxDataPerSec)
	}
	if a.TxDataPerSec > 0 {
		fmt.Fprintf(b, "tx-data-per-sec = %d\n", a.TxDataPerSec)
	}
	if a.Camouflage {
		appendBoolLine(b, "camouflage", true)
		if s := strings.TrimSpace(a.CamouflageSecret); s != "" {
			appendQuotedLine(b, "camouflage_secret", s)
		}
		if r := strings.TrimSpace(a.CamouflageRealm); r != "" {
			appendQuotedLine(b, "camouflage_realm", r)
		}
	} else {
		appendBoolLine(b, "camouflage", false)
	}
}
