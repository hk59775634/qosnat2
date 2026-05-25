package ocserv

import (
	"bytes"
	"fmt"

	"github.com/hk59775634/qosnat2/internal/store"
)

func appendBoolLine(b *bytes.Buffer, key string, val bool) {
	fmt.Fprintf(b, "%s = %t\n", key, val)
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
	appendBoolLine(b, "compression", a.Compression)
	appendBoolLine(b, "predictable-ips", a.PredictableIPs)
	appendBoolLine(b, "ping-leases", a.PingLeases)
	appendBoolLine(b, "use-occtl", a.UseOcctl)
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
		fmt.Fprintf(b, "rekey-method = ssl\n")
	}
	fmt.Fprintf(b, "max-same-clients = %d\n", a.MaxSameClients)
}
