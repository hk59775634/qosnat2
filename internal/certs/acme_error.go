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

// extractACMERetryAfter 从 Let's Encrypt 等返回的长错误串中截取「retry after …」时间戳（原文片段）
func extractACMERetryAfter(raw string) string {
	lo := strings.ToLower(raw)
	const needle = "retry after "
	idx := strings.Index(lo, needle)
	if idx < 0 {
		return ""
	}
	s := strings.TrimSpace(raw[idx+len(needle):])
	if cut := strings.Index(strings.ToLower(s), ": see "); cut >= 0 {
		s = strings.TrimSpace(s[:cut])
	}
	if cut := strings.IndexByte(s, '\n'); cut >= 0 {
		s = strings.TrimSpace(s[:cut])
	}
	return s
}

func isACMECertRateLimited(msg string) bool {
	return strings.Contains(msg, "urn:ietf:params:acme:error:ratelimited") ||
		strings.Contains(msg, "too many certificates") ||
		strings.Contains(msg, "rate-limits") ||
		(strings.Contains(msg, "error: 429") && strings.Contains(msg, "acme"))
}

// acmeBriefSummary 压缩 lego 返回的冗长 ACME 串：保留错误类型 + 核心一句，去掉 URL 与文档链接尾
func acmeBriefSummary(raw string) string {
	const maxTail = 140
	const maxTotal = 220
	if len(raw) <= maxTotal {
		return raw
	}
	parts := strings.Split(raw, " :: ")
	urnIdx := -1
	for i, p := range parts {
		if strings.Contains(p, "urn:ietf:params:acme:error:") {
			urnIdx = i
			break
		}
	}
	if urnIdx < 0 {
		if len(raw) > maxTotal {
			return raw[:maxTotal] + "…"
		}
		return raw
	}
	urn := strings.TrimSpace(parts[urnIdx])
	code := urn
	if i := strings.LastIndex(urn, ":"); i >= 0 {
		code = urn[i+1:]
	}
	var b strings.Builder
	for i := urnIdx + 1; i < len(parts); i++ {
		p := strings.TrimSpace(parts[i])
		pl := strings.ToLower(p)
		if strings.HasPrefix(pl, "http://") || strings.HasPrefix(pl, "https://") {
			break
		}
		if b.Len() > 0 {
			b.WriteString(" ")
		}
		b.WriteString(p)
	}
	detail := strings.TrimSpace(b.String())
	if cut := strings.Index(strings.ToLower(detail), " see http"); cut >= 0 {
		detail = strings.TrimSpace(detail[:cut])
	}
	if len(detail) > maxTail {
		detail = detail[:maxTail] + "…"
	}
	head := acmeErrorCodeZH(code)
	if detail != "" {
		return head + "：" + detail
	}
	return head
}

func acmeErrorCodeZH(code string) string {
	switch strings.ToLower(strings.TrimSpace(code)) {
	case "ratelimited":
		return "签发次数限速"
	case "dns":
		return "DNS 未通过"
	case "tls":
		return "TLS 相关错误"
	case "unauthorized":
		return "授权校验失败"
	case "badcsr":
		return "CSR 无效"
	case "malformed":
		return "请求格式错误"
	case "serverinternal":
		return "CA 内部错误"
	case "badnonce":
		return "nonce 错误"
	case "invalidcontact":
		return "联系邮箱无效"
	case "rejectedidentifier":
		return "标识被拒绝"
	case "caa":
		return "CAA 策略拒绝"
	default:
		if code == "" {
			return "ACME 错误"
		}
		return "ACME（" + code + "）"
	}
}

// ClassifyACMEError 判断 ACME 失败原因；域名解析类错误将暂停自动续期
func ClassifyACMEError(err error) ACMEErrorInfo {
	if err == nil {
		return ACMEErrorInfo{}
	}
	raw := err.Error()
	msg := strings.ToLower(raw)
	info := ACMEErrorInfo{Summary: raw}

	if strings.Contains(msg, "invalid account url") || strings.Contains(msg, "keyid header") {
		info.Summary = "ACME 账户环境与当前「测试(staging)」开关不一致。请取消勾选测试后重试，或删除 /var/lib/qosnat2/acme/account*.json 后重新申请。"
		return info
	}
	if strings.Contains(msg, "not a public ipv4") || strings.Contains(msg, "cgnat") {
		info.Summary = "Let's Encrypt IP 证书仅支持公网 IPv4（不可为 10.x、192.168.x、100.64.x 等内网/CGNAT 地址）。请填写从公网访问本机所用的 IP。"
		return info
	}

	if isACMECertRateLimited(msg) {
		info.Summary = "Let's Encrypt 限速：同一域名或 IP 在约 168 小时内最多新签 5 次，请减少重试频率。"
		if t := extractACMERetryAfter(raw); t != "" {
			info.Summary += " 建议 " + t + " 后再试。"
		}
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
	info.Summary = acmeBriefSummary(raw)
	return info
}
