package store

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"time"
)

const (
	CertTypeManual = "manual"
	CertTypeACME   = "acme"
	// DefaultAcmeAutoRenewDays 证书管理库内独立 ACME 自动续期（天）
	DefaultAcmeAutoRenewDays = 30
	// DefaultIPAcmeAutoRenewDays 公网 IP 短期证书（Let's Encrypt 全周期约 7 天）建议提前续期天数
	DefaultIPAcmeAutoRenewDays = 3
	// maxIPAcmeRenewBeforeDays IP 证书周期短，续期阈值（天）若过大会在「剩余天数」整数化后几乎始终触发续期；用户配置超过此值时按 DefaultIPAcmeAutoRenewDays 处理
	maxIPAcmeRenewBeforeDays = 5
	// ServiceAutoRenewDays Web UI / ocserv 绑定证书（域名证书）自动续期：到期前若干天
	ServiceAutoRenewDays = 7
)

// ManagedCertificate 证书库条目（PEM 存于 CertPath/KeyPath）
type ManagedCertificate struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Type          string   `json:"type"` // manual | acme
	Domains       []string `json:"domains,omitempty"`
	CertPath      string   `json:"cert_path"`
	KeyPath       string   `json:"key_path"`
	CaPath        string   `json:"ca_path,omitempty"`
	Subject       string   `json:"subject,omitempty"`
	NotAfter      string   `json:"not_after,omitempty"`
	CreatedAt     string   `json:"created_at"`
	AcmeEmail     string   `json:"acme_email,omitempty"`
	AcmeStaging   bool     `json:"acme_staging,omitempty"`
	AcmeRenewDays          int    `json:"acme_renew_days,omitempty"`
	AcmeLastOK             string `json:"acme_last_ok,omitempty"`
	AcmeLastError          string `json:"acme_last_error,omitempty"`
	AutoRenewEnabled       bool   `json:"auto_renew_enabled,omitempty"`
	AutoRenewPaused        bool   `json:"auto_renew_paused,omitempty"`
	AutoRenewPauseReason   string `json:"auto_renew_pause_reason,omitempty"`
}

// FindManagedCert 按 id 查找证书
func FindManagedCert(certs []ManagedCertificate, id string) (ManagedCertificate, bool) {
	id = strings.TrimSpace(id)
	for _, c := range certs {
		if c.ID == id {
			return c, true
		}
	}
	return ManagedCertificate{}, false
}

// NewManagedCertID 生成 cert-xxxxxxxx
func NewManagedCertID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "cert-" + hex.EncodeToString(b)
}

// NormalizeManagedCertificate 校验并填充默认值
func NormalizeManagedCertificate(c *ManagedCertificate) error {
	if c == nil {
		return fmt.Errorf("certificate nil")
	}
	c.Name = strings.TrimSpace(c.Name)
	if c.Name == "" {
		return fmt.Errorf("name required")
	}
	typ := strings.ToLower(strings.TrimSpace(c.Type))
	if typ != CertTypeManual && typ != CertTypeACME {
		return fmt.Errorf("type must be manual or acme")
	}
	c.Type = typ
	if c.ID == "" {
		c.ID = NewManagedCertID()
	}
	if c.CreatedAt == "" {
		c.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if len(c.Domains) > 0 && net.ParseIP(strings.TrimSpace(c.Domains[0])) != nil {
		if c.AcmeRenewDays <= 0 || c.AcmeRenewDays > maxIPAcmeRenewBeforeDays {
			c.AcmeRenewDays = DefaultIPAcmeAutoRenewDays
		}
	} else if c.AcmeRenewDays <= 0 {
		c.AcmeRenewDays = DefaultAcmeAutoRenewDays
	}
	if c.Type == CertTypeACME && !c.AutoRenewPaused {
		c.AutoRenewEnabled = true
	}
	var domains []string
	for _, d := range c.Domains {
		d = strings.TrimSpace(d)
		if d != "" {
			domains = append(domains, d)
		}
	}
	c.Domains = domains
	if c.Type == CertTypeACME && len(c.Domains) == 0 {
		return fmt.Errorf("acme certificate requires domain")
	}
	return nil
}

// ResolveOCServGlobalCerts 解析 ocserv 全局 TLS 路径
func ResolveOCServGlobalCerts(o OCServState, certs []ManagedCertificate) (cert, key, ca string) {
	if id := strings.TrimSpace(o.ManagedCertID); id != "" {
		if c, ok := FindManagedCert(certs, id); ok {
			return c.CertPath, c.KeyPath, c.CaPath
		}
	}
	if o.UseQoSnatTLS {
		return "/etc/qosnat2/tls.crt", "/etc/qosnat2/tls.key", ""
	}
	cert = strings.TrimSpace(o.ServerCertPath)
	key = strings.TrimSpace(o.ServerKeyPath)
	ca = strings.TrimSpace(o.CaCertPath)
	if cert == "" {
		cert = "/etc/ocserv/certs/server-cert.pem"
	}
	if key == "" {
		key = "/etc/ocserv/certs/server-key.pem"
	}
	return cert, key, ca
}

// ResolveOCServVhostCerts 解析 vhost TLS 路径（空 managed_cert_id 时继承全局）
func ResolveOCServVhostCerts(v OCServVhost, global OCServState, certs []ManagedCertificate) (cert, key, ca string) {
	if id := strings.TrimSpace(v.ManagedCertID); id != "" {
		if c, ok := FindManagedCert(certs, id); ok {
			return c.CertPath, c.KeyPath, c.CaPath
		}
	}
	if p := strings.TrimSpace(v.ServerCertPath); p != "" {
		cert = p
		key = strings.TrimSpace(v.ServerKeyPath)
		ca = strings.TrimSpace(v.CaCertPath)
		return cert, key, ca
	}
	return ResolveOCServGlobalCerts(global, certs)
}

// CertUsage 证书被引用位置
type CertUsage struct {
	Place string `json:"place"`
	Label string `json:"label,omitempty"`
}

// ManagedCertUsages 列出证书引用（删除前检查）
func ManagedCertUsages(id string, st State) []CertUsage {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	var out []CertUsage
	o := st.VPN.OCServ
	if strings.TrimSpace(o.ManagedCertID) == id {
		out = append(out, CertUsage{Place: "ocserv-global", Label: "ocserv 全局"})
	}
	for _, v := range o.Vhosts {
		if strings.TrimSpace(v.ManagedCertID) == id {
			out = append(out, CertUsage{Place: "ocserv-vhost:" + v.Domain, Label: "vhost " + v.Domain})
		}
	}
	if strings.TrimSpace(st.System.TLSManagedCertID) == id {
		out = append(out, CertUsage{Place: "qosnat-https", Label: "qosnat2 HTTPS"})
	}
	return out
}

// ManagedCertIsIP 证书是否以公网 IP 为标识（短期 IP 证书）
func ManagedCertIsIP(c ManagedCertificate) bool {
	if len(c.Domains) == 0 {
		return false
	}
	return net.ParseIP(strings.TrimSpace(c.Domains[0])) != nil
}

// CertAcmeRenewBeforeDays 自动续期触发阈值（剩余有效期 ≤ 该天数时尝试续期）
func CertAcmeRenewBeforeDays(c ManagedCertificate) int {
	if ManagedCertIsIP(c) {
		if c.AcmeRenewDays > 0 && c.AcmeRenewDays <= maxIPAcmeRenewBeforeDays {
			return c.AcmeRenewDays
		}
		return DefaultIPAcmeAutoRenewDays
	}
	if c.AcmeRenewDays > 0 {
		return c.AcmeRenewDays
	}
	return DefaultAcmeAutoRenewDays
}

// ServiceBoundCertRenewBeforeDays HTTPS/ocserv 引用的 ACME 证书续期阈值（天）
func ServiceBoundCertRenewBeforeDays(c ManagedCertificate) int {
	if ManagedCertIsIP(c) {
		return CertAcmeRenewBeforeDays(c)
	}
	return ServiceAutoRenewDays
}

// CertShouldAutoRenew 是否参与后台自动续期
func CertShouldAutoRenew(c ManagedCertificate) bool {
	if c.Type != CertTypeACME || c.CertPath == "" {
		return false
	}
	if c.AutoRenewPaused {
		return false
	}
	return c.AutoRenewEnabled
}

// CertIsServiceBound 证书是否被 HTTPS 或 ocserv 引用
func CertIsServiceBound(id string, st State) bool {
	id = strings.TrimSpace(id)
	if id == "" {
		return false
	}
	if strings.TrimSpace(st.System.TLSManagedCertID) == id {
		return true
	}
	o := st.VPN.OCServ
	if strings.TrimSpace(o.ManagedCertID) == id {
		return true
	}
	for _, v := range o.Vhosts {
		if strings.TrimSpace(v.ManagedCertID) == id {
			return true
		}
	}
	return false
}

// ServiceBoundCertIDs 收集所有服务绑定的证书 ID（去重）
func ServiceBoundCertIDs(st State) []string {
	seen := map[string]struct{}{}
	var ids []string
	add := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	add(st.System.TLSManagedCertID)
	add(st.VPN.OCServ.ManagedCertID)
	for _, v := range st.VPN.OCServ.Vhosts {
		add(v.ManagedCertID)
	}
	return ids
}

