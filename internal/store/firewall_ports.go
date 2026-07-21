package store

import (
	"fmt"
	"strconv"
	"strings"
)

// ParsePortSpec 解析端口规格：单端口、范围或逗号列表，如 "80"、"8000-8100"、"80,443,8000-8010"。
// 返回可用于 nft `{ ... }` 的规范化片段（已排序去重）。
func ParsePortSpec(spec string) ([]string, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil, nil
	}
	seen := map[string]struct{}{}
	var out []string
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "-") {
			a, b, ok := strings.Cut(part, "-")
			if !ok {
				return nil, fmt.Errorf("invalid port range %q", part)
			}
			lo, err := strconv.Atoi(strings.TrimSpace(a))
			if err != nil {
				return nil, fmt.Errorf("invalid port range %q", part)
			}
			hi, err := strconv.Atoi(strings.TrimSpace(b))
			if err != nil {
				return nil, fmt.Errorf("invalid port range %q", part)
			}
			if lo < 1 || hi > 65535 || lo > hi {
				return nil, fmt.Errorf("invalid port range %q", part)
			}
			tok := fmt.Sprintf("%d-%d", lo, hi)
			if _, ok := seen[tok]; ok {
				continue
			}
			seen[tok] = struct{}{}
			out = append(out, tok)
			continue
		}
		n, err := strconv.Atoi(part)
		if err != nil || n < 1 || n > 65535 {
			return nil, fmt.Errorf("invalid port %q", part)
		}
		tok := strconv.Itoa(n)
		if _, ok := seen[tok]; ok {
			continue
		}
		seen[tok] = struct{}{}
		out = append(out, tok)
	}
	return out, nil
}

// NormalizePortSpec 校验并规范化端口规格字符串。
func NormalizePortSpec(spec string) (string, error) {
	parts, err := ParsePortSpec(spec)
	if err != nil {
		return "", err
	}
	return strings.Join(parts, ","), nil
}

// NftPortMatch 生成 sport/dport 匹配子句（含单端口、集合、别名 set）。
func NftPortMatch(kind string, single int, multi string, alias string) string {
	kind = strings.TrimSpace(kind)
	if alias != "" {
		return kind + " @alias_" + alias
	}
	multi = strings.TrimSpace(multi)
	if multi != "" {
		parts, err := ParsePortSpec(multi)
		if err != nil || len(parts) == 0 {
			return ""
		}
		if len(parts) == 1 && !strings.Contains(parts[0], "-") {
			return fmt.Sprintf("%s %s", kind, parts[0])
		}
		return fmt.Sprintf("%s { %s }", kind, strings.Join(parts, ", "))
	}
	if single > 0 {
		return fmt.Sprintf("%s %d", kind, single)
	}
	return ""
}
