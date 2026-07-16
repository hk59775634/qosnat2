package store

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const aliasFetchTimeout = 60 * time.Second
const aliasFetchMaxBytes = 4 << 20 // 4 MiB

// FetchCIDRListFromURL 从 URL 拉取 CIDR 列表（每行一条，忽略空行与 # 注释）。
func FetchCIDRListFromURL(rawURL string) ([]string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, fmt.Errorf("url required")
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("url scheme must be http or https")
	}
	if u.Host == "" {
		return nil, fmt.Errorf("url host required")
	}

	client := &http.Client{Timeout: aliasFetchTimeout}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "qosnat2/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("fetch: HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, aliasFetchMaxBytes))
	if err != nil {
		return nil, err
	}
	return ParseCIDRListText(string(body))
}

// ParseCIDRListText 解析文本 CIDR 列表。
func ParseCIDRListText(text string) ([]string, error) {
	seen := map[string]struct{}{}
	var out []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		if line == "" {
			continue
		}
		if err := ValidateIPv4OrCIDR(line); err != nil {
			return nil, fmt.Errorf("line %q: %w", line, err)
		}
		if _, ok := seen[line]; ok {
			continue
		}
		seen[line] = struct{}{}
		out = append(out, line)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no valid CIDR in response")
	}
	return out, nil
}

// RefreshAliasFromURL 拉取 URL 并更新 alias members。
func RefreshAliasFromURL(a *AliasSet) error {
	if a == nil {
		return fmt.Errorf("alias nil")
	}
	u := strings.TrimSpace(a.URL)
	if u == "" {
		return fmt.Errorf("alias %q has no url", a.Name)
	}
	members, err := FetchCIDRListFromURL(u)
	if err != nil {
		return err
	}
	a.Members = members
	a.URLFetchedAt = time.Now().UTC().Format(time.RFC3339)
	return nil
}

// RefreshURLAliases 刷新所有带 URL 的别名（不含 FQDN）；兼容旧调用。
func RefreshURLAliases(aliases []AliasSet) ([]AliasSet, []string) {
	out := make([]AliasSet, len(aliases))
	copy(out, aliases)
	var warns []string
	for i := range out {
		if strings.ToLower(strings.TrimSpace(out[i].Type)) == "fqdn" {
			continue
		}
		if strings.TrimSpace(out[i].URL) == "" {
			continue
		}
		if err := RefreshAliasFromURL(&out[i]); err != nil {
			warns = append(warns, fmt.Sprintf("%s: %v", out[i].Name, err))
		}
	}
	return out, warns
}

// AliasByName 构建别名索引。
func AliasByName(aliases []AliasSet) map[string]AliasSet {
	m := make(map[string]AliasSet, len(aliases))
	for _, a := range aliases {
		m[a.Name] = a
	}
	return m
}

// AliasMembers 解析别名或 CIDR 为成员列表；cidr 非空时返回单元素。
func AliasMembers(cidr, aliasName string, byName map[string]AliasSet) ([]string, error) {
	cidr = strings.TrimSpace(cidr)
	aliasName = strings.TrimSpace(aliasName)
	if cidr != "" && aliasName != "" {
		return nil, fmt.Errorf("cidr and alias are mutually exclusive")
	}
	if aliasName != "" {
		a, ok := byName[aliasName]
		if !ok {
			return nil, fmt.Errorf("alias %q not found", aliasName)
		}
		if len(a.Members) == 0 {
			return nil, fmt.Errorf("alias %q has no members (refresh url/fqdn first)", aliasName)
		}
		return append([]string(nil), a.Members...), nil
	}
	if cidr != "" {
		if err := ValidateCIDR(cidr); err != nil {
			return nil, err
		}
		return []string{cidr}, nil
	}
	return nil, nil // any
}
