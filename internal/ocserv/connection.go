package ocserv

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/hk59775634/qosnat2/internal/certs"
	"github.com/hk59775634/qosnat2/internal/store"
)

// ConnectionInfo 客户端连接地址（仅管理 API 返回）。
type ConnectionInfo struct {
	URL              string   `json:"url,omitempty"`
	Host             string   `json:"host,omitempty"`
	Port             int      `json:"port,omitempty"`
	PortInURL        bool     `json:"port_in_url"`
	Camouflage       bool     `json:"camouflage_enabled"`
	CamouflageSecret string   `json:"camouflage_secret,omitempty"`
	CertHostnames    []string `json:"cert_hostnames,omitempty"`
	Issue            string   `json:"issue,omitempty"` // no_cert | no_hostname | camouflage_secret_missing
}

// BuildConnectionInfo 根据 TLS 证书与 ocserv 配置拼装 OpenConnect 连接 URL。
// 伪装开启时格式：https://host[:port]/?camouflage_secret
func BuildConnectionInfo(o store.OCServState, managed []store.ManagedCertificate) ConnectionInfo {
	port := o.TCPPort
	if port <= 0 {
		port = 443
	}
	camouflage := o.Advanced.Camouflage
	secret := strings.TrimSpace(o.Advanced.CamouflageSecret)

	host, certHosts, certIssue := resolveConnectHost(o, managed)
	out := ConnectionInfo{
		Port:             port,
		PortInURL:        port != 443,
		Camouflage:       camouflage,
		CamouflageSecret: secret,
		CertHostnames:    certHosts,
		Host:             host,
	}
	if certIssue != "" {
		out.Issue = certIssue
		return out
	}
	if host == "" {
		out.Issue = "no_hostname"
		return out
	}
	if camouflage && secret == "" {
		out.Issue = "camouflage_secret_missing"
		out.URL = buildConnectURL(host, port, false, "")
		return out
	}
	out.URL = buildConnectURL(host, port, camouflage, secret)
	return out
}

// BuildVhostConnectionInfo 按 vhost 域名与证书生成客户端连接 URL（端口继承全局 tcp-port）。
func BuildVhostConnectionInfo(v store.OCServVhost, global store.OCServState, managed []store.ManagedCertificate) ConnectionInfo {
	port := global.TCPPort
	if port <= 0 {
		port = 443
	}
	domain := strings.TrimSpace(v.Domain)
	camouflage := v.Camouflage || global.Advanced.Camouflage
	secret := strings.TrimSpace(v.CamouflageSecret)
	if secret == "" && camouflage {
		secret = strings.TrimSpace(global.Advanced.CamouflageSecret)
	}

	var candidates []string
	if domain != "" {
		candidates = append(candidates, domain)
	}
	if id := strings.TrimSpace(v.ManagedCertID); id != "" {
		if c, ok := store.FindManagedCert(managed, id); ok {
			candidates = append(candidates, c.Domains...)
		}
	}
	certPath, _, _ := store.ResolveOCServVhostCerts(v, global, managed)
	if certPath != "" {
		if fromFile, err := certs.HostnamesFromCertFile(certPath); err == nil {
			candidates = append(candidates, fromFile...)
		}
	}
	all := uniqueStrings(candidates)
	host := certs.PrimaryConnectHostname(all)
	if host == "" {
		host = domain
	}
	out := ConnectionInfo{
		Port:             port,
		PortInURL:        port != 443,
		Camouflage:       camouflage,
		CamouflageSecret: secret,
		CertHostnames:    all,
		Host:             host,
	}
	if host == "" {
		out.Issue = "no_hostname"
		return out
	}
	if camouflage && secret == "" {
		out.Issue = "camouflage_secret_missing"
		out.URL = buildConnectURL(host, port, false, "")
		return out
	}
	out.URL = buildConnectURL(host, port, camouflage, secret)
	return out
}

func resolveConnectHost(o store.OCServState, managed []store.ManagedCertificate) (host string, all []string, issue string) {
	var candidates []string
	if id := strings.TrimSpace(o.ManagedCertID); id != "" {
		if c, ok := store.FindManagedCert(managed, id); ok {
			for _, d := range c.Domains {
				candidates = append(candidates, d)
			}
		}
	}
	certPath, _, _ := store.ResolveOCServGlobalCerts(o, managed)
	if certPath == "" {
		if len(candidates) == 0 {
			return "", nil, "no_cert"
		}
		host = certs.PrimaryConnectHostname(candidates)
		return host, uniqueStrings(candidates), ""
	}
	fromFile, err := certs.HostnamesFromCertFile(certPath)
	if err != nil {
		if len(candidates) == 0 {
			return "", nil, "no_cert"
		}
	} else {
		candidates = append(candidates, fromFile...)
	}
	all = uniqueStrings(candidates)
	host = certs.PrimaryConnectHostname(all)
	return host, all, ""
}

func buildConnectURL(host string, port int, camouflage bool, secret string) string {
	hostport := host
	if port != 443 {
		hostport = net.JoinHostPort(host, strconv.Itoa(port))
	}
	base := "https://" + hostport
	if !camouflage || secret == "" {
		return base
	}
	return base + "/?" + secret
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, s := range in {
		s = strings.TrimSpace(strings.ToLower(s))
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

// FormatConnectionInfo 调试用。
func FormatConnectionInfo(ci ConnectionInfo) string {
	return fmt.Sprintf("url=%q host=%q issue=%q", ci.URL, ci.Host, ci.Issue)
}
