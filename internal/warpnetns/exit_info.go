package warpnetns

import (
	"os/exec"
	"strings"
	"sync"
	"time"
)

const (
	exitInfoCacheTTL    = 60 * time.Second
	cloudflareTraceURL  = "https://1.1.1.1/cdn-cgi/trace"
)

// ExitInfo is the WARP tunnel egress as seen from the isolated netns (Cloudflare trace).
type ExitInfo struct {
	IP        string `json:"ip,omitempty"`
	Country   string `json:"country,omitempty"`
	City      string `json:"city,omitempty"`
	Region    string `json:"region,omitempty"`
	Org       string `json:"org,omitempty"`
	FetchedAt string `json:"fetched_at,omitempty"`
	Error     string `json:"error,omitempty"`
}

var (
	exitInfoMu       sync.Mutex
	exitInfoCache    ExitInfo
	exitInfoCachedAt time.Time
)

// ClearExitInfoCache drops cached egress lookup (e.g. after disconnect).
func ClearExitInfoCache() {
	exitInfoMu.Lock()
	defer exitInfoMu.Unlock()
	exitInfoCache = ExitInfo{}
	exitInfoCachedAt = time.Time{}
}

// GetExitInfo returns cached egress info when connected; refreshes periodically.
func GetExitInfo(connected bool) ExitInfo {
	if !connected {
		ClearExitInfoCache()
		return ExitInfo{}
	}
	exitInfoMu.Lock()
	if !exitInfoCachedAt.IsZero() && time.Since(exitInfoCachedAt) < exitInfoCacheTTL {
		out := exitInfoCache
		exitInfoMu.Unlock()
		return out
	}
	exitInfoMu.Unlock()

	st := fetchExitInfoFromNetns()
	exitInfoMu.Lock()
	exitInfoCache = st
	exitInfoCachedAt = time.Now()
	exitInfoMu.Unlock()
	return st
}

func parseCloudflareTrace(raw []byte) ExitInfo {
	vars := map[string]string{}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		i := strings.IndexByte(line, '=')
		if i <= 0 {
			continue
		}
		vars[line[:i]] = strings.TrimSpace(line[i+1:])
	}
	ip := strings.TrimSpace(vars["ip"])
	if ip == "" {
		return ExitInfo{Error: "cloudflare trace missing ip"}
	}
	country := strings.TrimSpace(vars["loc"])
	region := strings.TrimSpace(vars["colo"])
	org := ""
	if w := strings.TrimSpace(vars["warp"]); w != "" {
		org = "warp=" + w
		if g := strings.TrimSpace(vars["gateway"]); g != "" {
			org += ", gateway=" + g
		}
	}
	return ExitInfo{
		IP:        ip,
		Country:   country,
		Region:    region,
		Org:       org,
		FetchedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

func fetchExitInfoFromNetns() ExitInfo {
	if _, err := exec.LookPath("curl"); err != nil {
		return ExitInfo{Error: "curl not installed"}
	}
	if !NetnsHealthy() {
		return ExitInfo{Error: "warp netns not ready"}
	}
	out, err := netnsExec("curl", "-fsS", "--max-time", "12", cloudflareTraceURL)
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return ExitInfo{Error: msg}
	}
	if len(strings.TrimSpace(string(out))) == 0 {
		return ExitInfo{Error: "empty response from cloudflare trace"}
	}
	return parseCloudflareTrace(out)
}
