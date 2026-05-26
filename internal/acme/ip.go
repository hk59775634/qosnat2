package acme

import (
	"fmt"
	"net"
	"strings"

	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge/http01"
)

const (
	// ShortLivedProfile Let's Encrypt 短期/IP 证书 profile（约 6–7 天有效）
	ShortLivedProfile = "shortlived"
)

// NormalizeIP 校验公网 IPv4（用于 ACME IP 证书标识）
func NormalizeIP(ip string) (string, error) {
	ip = strings.TrimSpace(ip)
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return "", fmt.Errorf("invalid IP address")
	}
	v4 := parsed.To4()
	if v4 == nil {
		return "", fmt.Errorf("only IPv4 is supported for ACME IP certificates")
	}
	return v4.String(), nil
}

// ObtainIP 通过 HTTP-01 为公网 IPv4 申请 Let's Encrypt 短期证书（profile=shortlived）。
// 需本机 80 端口可从公网访问；证书有效期约 6 天，请启用自动续期。
func ObtainIP(cfg Config, ip string) (*Result, error) {
	ip, err := NormalizeIP(ip)
	if err != nil {
		return nil, err
	}
	email := strings.TrimSpace(cfg.Email)
	if email == "" {
		return nil, fmt.Errorf("ACME 邮箱必填（Let's Encrypt 账户）")
	}
	// LE IP 证书（profile shortlived）要求 IP 仅出现在 SAN，不能写入 CSR Common Name。
	client, err := newClient(email, cfg.Staging, true)
	if err != nil {
		return nil, err
	}
	if err := client.Challenge.SetHTTP01Provider(http01.NewProviderServer("", "80")); err != nil {
		return nil, fmt.Errorf("http-01 provider: %w", err)
	}
	res, err := client.Certificate.Obtain(certificate.ObtainRequest{
		Domains: []string{ip},
		Bundle:  true,
		Profile: ShortLivedProfile,
	})
	if err != nil {
		return nil, err
	}
	return resultFromResource(res)
}

// RenewIP 为 IP 短期证书重新签发
func RenewIP(cfg Config, ip string) (*Result, error) {
	return ObtainIP(cfg, ip)
}
