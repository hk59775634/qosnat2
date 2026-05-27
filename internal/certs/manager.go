package certs

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hk59775634/qosnat2/internal/acme"
	"github.com/hk59775634/qosnat2/internal/store"
)

const BaseDir = "/var/lib/qosnat2/certs"

// SavePEM 将证书写入证书库目录并返回路径
func SavePEM(id, certPEM, keyPEM, caPEM string) (certPath, keyPath, caPath string, err error) {
	certPEM = strings.TrimSpace(certPEM)
	keyPEM = strings.TrimSpace(keyPEM)
	if certPEM == "" || keyPEM == "" {
		return "", "", "", fmt.Errorf("certificate and private key PEM required")
	}
	if _, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM)); err != nil {
		return "", "", "", fmt.Errorf("invalid certificate or key: %w", err)
	}
	dir := filepath.Join(BaseDir, id)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return "", "", "", err
	}
	certPath = filepath.Join(dir, "fullchain.pem")
	keyPath = filepath.Join(dir, "privkey.pem")
	if err := os.WriteFile(certPath, []byte(normalizePEM(certPEM)), 0644); err != nil {
		return "", "", "", err
	}
	if err := os.WriteFile(keyPath, []byte(normalizePEM(keyPEM)), 0600); err != nil {
		return "", "", "", err
	}
	caPath = ""
	if caPEM = strings.TrimSpace(caPEM); caPEM != "" {
		caPath = filepath.Join(dir, "ca.pem")
		if err := os.WriteFile(caPath, []byte(normalizePEM(caPEM)), 0644); err != nil {
			return "", "", "", err
		}
	}
	return certPath, keyPath, caPath, nil
}

func normalizePEM(s string) string {
	return strings.TrimSpace(s) + "\n"
}

// RemoveDir 删除证书文件目录
func RemoveDir(id string) error {
	return os.RemoveAll(filepath.Join(BaseDir, id))
}

// ParseMeta 从 cert 文件解析 subject 与 not_after
func ParseMeta(certPath string) (subject, notAfter string) {
	b, err := os.ReadFile(certPath)
	if err != nil {
		return "", ""
	}
	block, _ := pem.Decode(b)
	if block == nil {
		return "", ""
	}
	c, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", ""
	}
	return c.Subject.String(), c.NotAfter.UTC().Format(time.RFC3339)
}

// ObtainACME 申请 ACME 证书
func ObtainACME(domain, email string, staging bool) (*store.ManagedCertificate, error) {
	domain, err := acme.NormalizeDomain(domain)
	if err != nil {
		return nil, err
	}
	email = strings.TrimSpace(email)
	if email == "" {
		return nil, fmt.Errorf("ACME email required")
	}
	res, err := acme.Obtain(acme.Config{Domain: domain, Email: email, Staging: staging})
	if err != nil {
		return nil, err
	}
	mc := &store.ManagedCertificate{
		ID:            store.NewManagedCertID(),
		Name:          domain,
		Type:          store.CertTypeACME,
		Domains:       []string{domain},
		AcmeEmail:     email,
		AcmeStaging:   staging,
		AcmeRenewDays:    store.DefaultAcmeAutoRenewDays,
		AutoRenewEnabled: true,
		AcmeLastOK:       time.Now().UTC().Format(time.RFC3339),
		AcmeLastError:    "",
	}
	certPath, keyPath, _, err := SavePEM(mc.ID, res.CertPEM, res.KeyPEM, "")
	if err != nil {
		_ = RemoveDir(mc.ID)
		return nil, err
	}
	mc.CertPath = certPath
	mc.KeyPath = keyPath
	if !res.NotAfter.IsZero() {
		mc.NotAfter = res.NotAfter.UTC().Format(time.RFC3339)
	}
	subject, na := ParseMeta(certPath)
	mc.Subject = subject
	if mc.NotAfter == "" {
		mc.NotAfter = na
	}
	return mc, nil
}

// ObtainACMEIP 为公网 IPv4 申请 Let's Encrypt 短期 IP 证书（profile shortlived）
func ObtainACMEIP(ip, email string, staging bool) (*store.ManagedCertificate, error) {
	ip, err := acme.NormalizeIP(ip)
	if err != nil {
		return nil, err
	}
	email = strings.TrimSpace(email)
	if email == "" {
		return nil, fmt.Errorf("ACME email required")
	}
	res, err := acme.ObtainIP(acme.Config{Email: email, Staging: staging}, ip)
	if err != nil {
		return nil, err
	}
	mc := &store.ManagedCertificate{
		ID:               store.NewManagedCertID(),
		Name:             ip,
		Type:             store.CertTypeACME,
		Domains:          []string{ip},
		AcmeEmail:        email,
		AcmeStaging:      staging,
		AcmeRenewDays:    store.DefaultIPAcmeAutoRenewDays,
		AutoRenewEnabled: true,
		AcmeLastOK:       time.Now().UTC().Format(time.RFC3339),
		AcmeLastError:    "",
	}
	certPath, keyPath, _, err := SavePEM(mc.ID, res.CertPEM, res.KeyPEM, "")
	if err != nil {
		_ = RemoveDir(mc.ID)
		return nil, err
	}
	mc.CertPath = certPath
	mc.KeyPath = keyPath
	if !res.NotAfter.IsZero() {
		mc.NotAfter = res.NotAfter.UTC().Format(time.RFC3339)
	}
	mc.Subject, mc.NotAfter = ParseMeta(certPath)
	return mc, nil
}

// RenewACME 续签 ACME 证书
func RenewACME(mc store.ManagedCertificate) (*store.ManagedCertificate, error) {
	if mc.Type != store.CertTypeACME || len(mc.Domains) == 0 {
		return nil, fmt.Errorf("not an ACME certificate")
	}
	domain := mc.Domains[0]
	cfg := acme.Config{
		Domain:  domain,
		Email:   mc.AcmeEmail,
		Staging: mc.AcmeStaging,
	}
	var res *acme.Result
	var err error
	if store.ManagedCertIsIP(mc) {
		res, err = acme.RenewIP(cfg, domain)
	} else {
		certPEM, rerr := os.ReadFile(mc.CertPath)
		if rerr != nil {
			return nil, rerr
		}
		keyPEM, rerr := os.ReadFile(mc.KeyPath)
		if rerr != nil {
			return nil, rerr
		}
		res, err = acme.Renew(cfg, string(certPEM), string(keyPEM))
	}
	if err != nil {
		mc.AcmeLastError = ClassifyACMEError(err).Summary
		return &mc, err
	}
	certPath, keyPath, caPath, err := SavePEM(mc.ID, res.CertPEM, res.KeyPEM, "")
	if err != nil {
		return nil, err
	}
	out := mc
	out.CertPath = certPath
	out.KeyPath = keyPath
	out.CaPath = caPath
	out.AcmeLastOK = time.Now().UTC().Format(time.RFC3339)
	out.AcmeLastError = ""
	out.Subject, out.NotAfter = ParseMeta(certPath)
	if !res.NotAfter.IsZero() {
		out.NotAfter = res.NotAfter.UTC().Format(time.RFC3339)
	}
	return &out, nil
}

// DaysUntilExpiry 距到期天数
func DaysUntilExpiry(certPath string) int {
	d, err := acme.DaysUntilExpiry(certPath)
	if err != nil {
		return -1
	}
	return d
}

// SyncToOCServPaths 复制到 ocserv 目标路径（供 nobody 读取）
func SyncToOCServPaths(srcCert, srcKey, destCert, destKey string) error {
	if err := os.MkdirAll(filepath.Dir(destCert), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destKey), 0755); err != nil {
		return err
	}
	if err := copyFile(srcCert, destCert, 0644); err != nil {
		return err
	}
	return copyFile(srcKey, destKey, 0600)
}

func copyFile(src, dst string, mode os.FileMode) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, b, mode)
}
