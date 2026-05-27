package certs

import (
	"strings"
)

// ACMEErrorInfo 续期失败分类（用于 UI 与是否暂停自动续期）
type ACMEErrorInfo struct {
	PauseAutoRenew bool
	IsDNS          bool
	Summary        string
}

// ClassifyACMEError 判断 ACME 失败原因；域名解析类错误将暂停自动续期
func ClassifyACMEError(err error) ACMEErrorInfo {
	if err == nil {
		return ACMEErrorInfo{}
	}
	msg := strings.ToLower(err.Error())
	info := ACMEErrorInfo{Summary: err.Error()}

	if strings.Contains(msg, "invalid account url") || strings.Contains(msg, "keyid header") {
		info.Summary = "ACME 账户环境与当前「测试(staging)」开关不一致。请取消勾选测试后重试，或删除 /var/lib/qosnat2/acme/account*.json 后重新申请。"
		return info
	}
	if strings.Contains(msg, "not a public ipv4") || strings.Contains(msg, "cgnat") {
		info.Summary = "Let's Encrypt IP 证书仅支持公网 IPv4（不可为 10.x、192.168.x、100.64.x 等内网/CGNAT 地址）。请填写从公网访问本机所用的 IP。"
		return info
	}

	dnsPatterns := []string{
		"dns problem",
		"dnsproble",
		"nxdomain",
		"no valid a records",
		"no valid aaaa records",
		"incorrect txt record",
		"could not resolve",
		"failed to resolve",
		"lookup ",
		" name not known",
		"cname loop",
		"caa ",
		"域名",
		"解析",
		"dns record",
		"propagation",
		"authoritative nameserver",
	}
	for _, p := range dnsPatterns {
		if strings.Contains(msg, p) {
			info.IsDNS = true
			info.PauseAutoRenew = true
			info.Summary = "域名解析或 DNS 记录不符合 ACME 校验要求（HTTP-01 需域名正确解析到本机且 80 端口可达）。请修正 DNS 后，在证书管理中手动开启自动续期。"
			return info
		}
	}
	return info
}
