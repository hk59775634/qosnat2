package warpnetns

import (
	"encoding/json"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const exitInfoCacheTTL = 60 * time.Second

// ExitInfo is the WARP tunnel egress as seen from the isolated netns (via ipinfo.io).
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

type ipinfoResponse struct {
	IP      string `json:"ip"`
	City    string `json:"city"`
	Region  string `json:"region"`
	Country string `json:"country"`
	Org     string `json:"org"`
	Error   string `json:"error"`
}

func parseIPInfoJSON(raw []byte) ExitInfo {
	raw = []byte(strings.TrimSpace(string(raw)))
	if len(raw) == 0 {
		return ExitInfo{Error: "empty response from ipinfo.io"}
	}
	var resp ipinfoResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return ExitInfo{Error: "parse ipinfo: " + err.Error()}
	}
	if msg := strings.TrimSpace(resp.Error); msg != "" {
		return ExitInfo{Error: msg}
	}
	ip := strings.TrimSpace(resp.IP)
	if ip == "" {
		return ExitInfo{Error: "ipinfo response missing ip"}
	}
	return ExitInfo{
		IP:        ip,
		Country:   strings.TrimSpace(resp.Country),
		City:      strings.TrimSpace(resp.City),
		Region:    strings.TrimSpace(resp.Region),
		Org:       strings.TrimSpace(resp.Org),
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
	out, err := netnsExec("curl", "-fsS", "--max-time", "12", "https://ipinfo.io/json")
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return ExitInfo{Error: msg}
	}
	return parseIPInfoJSON(out)
}
