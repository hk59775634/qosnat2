package store

import "strings"

// VhostFromGlobal 新建 vhost 时从全局 OCServ 配置复制默认值，便于在高级页逐项覆盖。
// RADIUS 仍为 nil（继承全局 radcli）；plain_passwd_path 为空（继承全局 ocpasswd）。
func VhostFromGlobal(o OCServState, domain, comment, authMethod string) OCServVhost {
	adv := o.Advanced
	am := strings.TrimSpace(authMethod)
	if am == "" {
		am = strings.TrimSpace(o.AuthMethod)
	}
	if am == "" {
		am = OCServAuthPlain
	}
	cfgPerGroup := strings.TrimSpace(o.ConfigPerGroup)
	if cfgPerGroup == "" {
		cfgPerGroup = strings.TrimSpace(adv.ConfigPerGroup)
	}
	v := OCServVhost{
		Enabled:            true,
		Domain:             strings.TrimSpace(domain),
		Comment:            strings.TrimSpace(comment),
		AuthMethod:         am,
		Users:              []OCServUser{},
		ServerCertPath:     o.ServerCertPath,
		ServerKeyPath:      o.ServerKeyPath,
		CaCertPath:         o.CaCertPath,
		IPv4Network:        o.IPv4Network,
		IPv4Netmask:        o.IPv4Netmask,
		DNS:                append([]string(nil), o.DNS...),
		Routes:             append([]string(nil), o.Routes...),
		NoRoutes:           append([]string(nil), o.NoRoutes...),
		ConfigPerGroup:     cfgPerGroup,
		ConfigPerUser:      o.ConfigPerUser,
		DefaultGroupConfig: o.DefaultGroupConfig,
		DefaultUserConfig:  o.DefaultUserConfig,
		AutoSelectGroup:    o.AutoSelectGroup,
		DefaultSelectGroup: o.DefaultSelectGroup,
		Compression:        adv.Compression,
		DtlsLegacy:         adv.DtlsLegacy,
		CiscoClientCompat:  adv.CiscoClientCompat,
		CiscoSvcCompat:     adv.CiscoSvcCompat,
		DenyRoaming:        adv.DenyRoaming,
		PredictableIPs:     adv.PredictableIPs,
		Camouflage:         adv.Camouflage,
		CamouflageSecret:   adv.CamouflageSecret,
		CamouflageRealm:    adv.CamouflageRealm,
		MaxSameClients:     adv.MaxSameClients,
		RekeyTime:          adv.RekeyTime,
		RekeyMethod:        adv.RekeyMethod,
		RxDataPerSec:       adv.RxDataPerSec,
		TxDataPerSec:       adv.TxDataPerSec,
		DefaultDomain:      adv.DefaultDomain,
		CertUserOID:        adv.CertUserOID,
		TLSPriorities:      adv.TLSPriorities,
		AcctEnabled:        o.Radius.AcctEnabled,
		StatsReportTime:    o.Radius.StatsReportTime,
	}
	if adv.Keepalive && adv.KeepaliveSec > 0 {
		v.Keepalive = adv.KeepaliveSec
	}
	if adv.DPD && adv.DPDSec > 0 {
		v.DPD = adv.DPDSec
	}
	if adv.MobileDPD && adv.MobileDPDSec > 0 {
		v.MobileDPD = adv.MobileDPDSec
	}
	if adv.CookieTimeout > 0 {
		v.CookieTimeout = adv.CookieTimeout
	}
	return v
}
