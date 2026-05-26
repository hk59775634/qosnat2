package certs

import (
	"crypto/x509"
	"net"
	"os"
	"strings"

	"encoding/pem"
)

// HostnamesFromCertFile 从 PEM 证书文件读取 CN 与 DNS SAN。
func HostnamesFromCertFile(certPath string) ([]string, error) {
	b, err := os.ReadFile(certPath)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(b)
	if block == nil {
		return nil, os.ErrInvalid
	}
	c, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}
	return HostnamesFromCert(c), nil
}

// HostnamesFromCert 汇总证书中的可用主机名。
func HostnamesFromCert(c *x509.Certificate) []string {
	if c == nil {
		return nil
	}
	seen := make(map[string]struct{})
	var out []string
	add := func(s string) {
		s = strings.TrimSpace(strings.ToLower(s))
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	if cn := strings.TrimSpace(c.Subject.CommonName); cn != "" {
		add(cn)
	}
	for _, d := range c.DNSNames {
		add(d)
	}
	return out
}

// PrimaryConnectHostname 优先选择适合客户端填写的 DNS 名（非 IP、非通配符优先）。
func PrimaryConnectHostname(candidates []string) string {
	var ipFallback, wildcardFallback string
	for _, raw := range candidates {
		h := strings.TrimSpace(strings.ToLower(raw))
		if h == "" {
			continue
		}
		if strings.HasPrefix(h, "*.") {
			if wildcardFallback == "" {
				wildcardFallback = strings.TrimPrefix(h, "*.")
			}
			continue
		}
		if net.ParseIP(h) != nil {
			if ipFallback == "" {
				ipFallback = h
			}
			continue
		}
		return h
	}
	if wildcardFallback != "" {
		return wildcardFallback
	}
	return ipFallback
}
