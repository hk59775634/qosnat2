package store

import (
	"fmt"
	"strings"
)

// SNMPState net-snmp snmpd 配置（持久化于 state.json）。
type SNMPState struct {
	Enabled             bool     `json:"enabled"`
	Port                int      `json:"port,omitempty"`
	ListenLocalhostOnly bool     `json:"listen_localhost_only,omitempty"`
	SysLocation         string   `json:"sys_location,omitempty"`
	SysContact          string   `json:"sys_contact,omitempty"`
	SysName             string   `json:"sys_name,omitempty"`
	ROCommunity         string   `json:"ro_community,omitempty"`
	AllowedNetworks     []string `json:"allowed_networks,omitempty"`
}

func DefaultSNMP() SNMPState {
	return SNMPState{
		Port:            161,
		AllowedNetworks: []string{},
	}
}

// SNMPFirewallChanged 判断 SNMP 变更是否影响 WAN 防火墙自动规则。
func SNMPFirewallChanged(a, b SNMPState) bool {
	if a.Enabled != b.Enabled || a.ListenLocalhostOnly != b.ListenLocalhostOnly || a.Port != b.Port {
		return true
	}
	if len(a.AllowedNetworks) != len(b.AllowedNetworks) {
		return true
	}
	for i := range a.AllowedNetworks {
		if a.AllowedNetworks[i] != b.AllowedNetworks[i] {
			return true
		}
	}
	return false
}

func NormalizeSNMP(s *SNMPState) error {
	if s == nil {
		return fmt.Errorf("snmp config nil")
	}
	s.SysLocation = strings.TrimSpace(s.SysLocation)
	s.SysContact = strings.TrimSpace(s.SysContact)
	s.SysName = strings.TrimSpace(s.SysName)
	s.ROCommunity = strings.TrimSpace(s.ROCommunity)
	if s.Port <= 0 {
		s.Port = 161
	}
	if s.Port > 65535 {
		return fmt.Errorf("port out of range")
	}
	s.ListenLocalhostOnly = false
	if !s.Enabled {
		return nil
	}
	if s.ROCommunity == "" {
		return fmt.Errorf("ro_community required when snmp enabled")
	}
	if strings.ContainsAny(s.ROCommunity, " \t\n\r\"'") {
		return fmt.Errorf("ro_community contains invalid characters")
	}
	if len(s.AllowedNetworks) == 0 {
		return fmt.Errorf("allowed_networks required when snmp enabled")
	}
	var nets []string
	for _, cidr := range s.AllowedNetworks {
		cidr = strings.TrimSpace(cidr)
		if cidr == "" {
			continue
		}
		if err := ValidateCIDR(cidr); err != nil {
			return fmt.Errorf("allowed_networks: %w", err)
		}
		nets = append(nets, cidr)
	}
	if len(nets) == 0 {
		return fmt.Errorf("allowed_networks required when snmp enabled")
	}
	s.AllowedNetworks = nets
	return nil
}
