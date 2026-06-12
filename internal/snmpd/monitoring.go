package snmpd

import (
	"github.com/hk59775634/qosnat2/internal/netif"
)

// MonitoringHintsFor 返回 LAN/WAN ifIndex 与 IF-MIB HC 计数 OID 模板。
func MonitoringHintsFor(devLAN, devWAN string) MonitoringHints {
	h := MonitoringHints{
		DevLAN: devLAN,
		DevWAN: devWAN,
		OIDTemplates: map[string]string{
			"if_hc_in_octets":  "1.3.6.1.2.1.31.1.1.1.6.{ifIndex}",
			"if_hc_out_octets": "1.3.6.1.2.1.31.1.1.1.10.{ifIndex}",
			"if_name":          "1.3.6.1.2.1.31.1.1.1.1.{ifIndex}",
		},
	}
	if devLAN != "" {
		if idx, err := netif.IfIndex(devLAN); err == nil {
			h.LANIfIndex = idx
		}
	}
	if devWAN != "" {
		if idx, err := netif.IfIndex(devWAN); err == nil {
			h.WANIfIndex = idx
		}
	}
	return h
}
