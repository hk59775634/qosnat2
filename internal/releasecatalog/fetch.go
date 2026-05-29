package releasecatalog

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	manifestDirectTimeout = 8 * time.Second
	manifestProxyTimeout  = 15 * time.Second
	// release 包约 6MB，国内链路常需更长时间。
	releaseDirectTimeout = 45 * time.Second
	releaseProxyTimeout  = 3 * time.Minute
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

// FetchBytes 依次尝试 urls 拉取小文件（如 manifest）；首个成功响应体返回。
func FetchBytes(urls []string) (body []byte, usedURL string, err error) {
	return fetchBytes(urls, manifestDirectTimeout, manifestProxyTimeout)
}

// FetchBytesRelease 拉取 release 压缩包等大文件，超时更长。
func FetchBytesRelease(urls []string) (body []byte, usedURL string, err error) {
	return fetchBytes(urls, releaseDirectTimeout, releaseProxyTimeout)
}

func fetchBytes(urls []string, directTimeout, proxyTimeout time.Duration) (body []byte, usedURL string, err error) {
	if len(urls) == 0 {
		return nil, "", fmt.Errorf("no urls to fetch")
	}
	var lastErr error
	for i, u := range urls {
		timeout := proxyTimeout
		if i == 0 {
			timeout = directTimeout
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
