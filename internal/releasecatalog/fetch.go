package releasecatalog

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	directFetchTimeout = 8 * time.Second
	proxyFetchTimeout  = 15 * time.Second
)

// gh-proxy 镜像前缀（中国等地区 GitHub 直连超时时备选）。
var ghProxyPrefixes = []string{
	"https://v4.gh-proxy.org/",
	"https://cdn.gh-proxy.org/",
}

// MirrorURLs 返回直连 URL 及 gh-proxy 加速镜像 URL（顺序：直连 → v4 → cdn）。
func MirrorURLs(directURL string) []string {
	directURL = strings.TrimSpace(directURL)
	if directURL == "" {
		return nil
	}
	out := make([]string, 0, 1+len(ghProxyPrefixes))
	out = append(out, directURL)
	for _, prefix := range ghProxyPrefixes {
		out = append(out, prefix+directURL)
	}
	return out
}

// FetchBytes 依次尝试 urls，首个成功响应体返回；全部失败则返回最后一次错误。
func FetchBytes(urls []string) (body []byte, usedURL string, err error) {
	if len(urls) == 0 {
		return nil, "", fmt.Errorf("no urls to fetch")
	}
	var lastErr error
	for i, u := range urls {
		timeout := proxyFetchTimeout
		if i == 0 {
			timeout = directFetchTimeout
		}
		b, err := fetchOne(u, timeout)
		if err == nil {
			return b, u, nil
		}
		lastErr = err
	}
	return nil, "", fmt.Errorf("fetch failed (%d urls): %w", len(urls), lastErr)
}

func fetchOne(url string, timeout time.Duration) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("User-Agent", "qosnat2-releasecatalog/1")
	cli := &http.Client{Timeout: timeout}
	resp, err := cli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	const maxBody = 64 << 20
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, err
	}
	return b, nil
}
