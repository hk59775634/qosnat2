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
