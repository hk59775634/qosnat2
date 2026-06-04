package dnsmasq

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	DefaultChnroutesPath = "/etc/qosnat2/chnroutes.txt"
	// 默认从 hk59775634/chnroutes 拉取（APNIC CN 聚合，每日更新）
	// https://github.com/hk59775634/chnroutes
	DefaultChnroutesURL = "https://cdn.jsdelivr.net/gh/hk59775634/chnroutes@main/chnroutes.txt"
)

// defaultChnroutesURLs 按优先级尝试（jsDelivr 国内较稳，其次 GitHub Raw）
var defaultChnroutesURLs = []string{
	DefaultChnroutesURL,
	"https://raw.githubusercontent.com/hk59775634/chnroutes/main/chnroutes.txt",
}

// ChnroutesInfo 路由表文件摘要
type ChnroutesInfo struct {
	Path    string `json:"path"`
	Exists  bool   `json:"exists"`
	Entries int    `json:"entries"`
}

// SupportsChnroutes 检测当前 dnsmasq 是否含 chnroutes 补丁
func SupportsChnroutes() bool {
	if !installed() {
		return false
	}
	out, err := execHelp()
	if err != nil {
		return false
	}
	return strings.Contains(out, "chnroutes-file")
}

func execHelp() (string, error) {
	out, err := exec.Command("dnsmasq", "--help").CombinedOutput()
	return string(out), err
}

// ChnroutesFileInfo 统计有效 CIDR 行数
func ChnroutesFileInfo(path string) ChnroutesInfo {
	path = strings.TrimSpace(path)
	if path == "" {
		path = DefaultChnroutesPath
	}
	info := ChnroutesInfo{Path: path}
	b, err := os.ReadFile(path)
	if err != nil {
		return info
	}
	info.Exists = true
	info.Entries = countChnrouteLines(string(b))
	return info
}

func countChnrouteLines(body string) int {
	n := 0
	sc := bufio.NewScanner(strings.NewReader(body))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		n++
	}
	return n
}

// DownloadChnroutes 下载并写入 chnroutes 文件（跳过 # 注释行）
func DownloadChnroutes(dest, url string) (entries int, err error) {
	dest = strings.TrimSpace(dest)
	if dest == "" {
		dest = DefaultChnroutesPath
	}
	urls := chnroutesFetchURLs(url)
	var lastErr error
	for _, u := range urls {
		entries, err = downloadChnroutesFrom(dest, u)
		if err == nil {
			return entries, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return 0, lastErr
	}
	return 0, fmt.Errorf("download chnroutes: no URL")
}

func chnroutesFetchURLs(url string) []string {
	url = strings.TrimSpace(url)
	if url == "" {
		return append([]string(nil), defaultChnroutesURLs...)
	}
	for _, u := range defaultChnroutesURLs {
		if url == u {
			return append([]string(nil), defaultChnroutesURLs...)
		}
	}
	return []string{url}
}

// ValidateChnroutesPath 限制写入路径在 /etc/qosnat2 下。
func ValidateChnroutesPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return DefaultChnroutesPath, nil
	}
	if strings.Contains(path, "..") {
		return "", fmt.Errorf("invalid chnroutes path")
	}
	clean := filepath.Clean(path)
	if clean != path {
		return "", fmt.Errorf("invalid chnroutes path")
	}
	if !strings.HasPrefix(clean, "/etc/qosnat2/") {
		return "", fmt.Errorf("chnroutes path must be under /etc/qosnat2")
	}
	return clean, nil
}

func downloadChnroutesFrom(dest, url string) (entries int, err error) {
	dest, err = ValidateChnroutesPath(dest)
	if err != nil {
		return 0, err
	}
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return 0, fmt.Errorf("download chnroutes from %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("download chnroutes from %s: HTTP %d", url, resp.StatusCode)
	}
	body, err := filterChnroutesBody(resp.Body)
	if err != nil {
		return 0, err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return 0, err
	}
	if err := os.WriteFile(dest, body, 0644); err != nil {
		return 0, err
	}
	return countChnrouteLines(string(body)), nil
}

func filterChnroutesBody(r io.Reader) ([]byte, error) {
	var b strings.Builder
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if b.Len() == 0 {
		return nil, fmt.Errorf("chnroutes download empty after filtering")
	}
	return []byte(b.String()), nil
}
