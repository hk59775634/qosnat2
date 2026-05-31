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
	IP          string `json:"ip,omitempty"`
	Country     string `json:"country,omitempty"`
	City        string `json:"city,omitempty"`
	Region      string `json:"region,omitempty"`
	Warp        string `json:"warp,omitempty"`         // trace warp= (off, on, plus, …)
	Gateway     string `json:"gateway,omitempty"`      // trace gateway= (off, on)
	WarpTier    string `json:"warp_tier,omitempty"`    // normalized: off, standard, plus, 2xc
	AccountType string `json:"account_type,omitempty"` // warp-cli registration show
	Org         string `json:"org,omitempty"`          // legacy summary for tooltips
	FetchedAt   string `json:"fetched_at,omitempty"`
	Error       string `json:"error,omitempty"`
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
		out := withFetchedAt(exitInfoCache, exitInfoCachedAt)
		exitInfoMu.Unlock()
		return out
	}
	exitInfoMu.Unlock()

	st := fetchExitInfoFromNetns()
	now := time.Now()
	st = withFetchedAt(st, now)
	exitInfoMu.Lock()
	exitInfoCache = st
	exitInfoCachedAt = now
	exitInfoMu.Unlock()
	return st
}

func withFetchedAt(st ExitInfo, at time.Time) ExitInfo {
	if st.FetchedAt == "" && !at.IsZero() {
		st.FetchedAt = at.UTC().Format(time.RFC3339)
	}
	return st
}

// NormalizeWarpTier maps cdn-cgi/trace warp= to a stable tier id for UI/API.
func NormalizeWarpTier(warp string) string {
	switch strings.ToLower(strings.TrimSpace(warp)) {
	case "", "off":
		return "off"
	case "on":
		return "standard"
	case "plus":
		return "plus"
	case "2xc", "2x":
		return "2xc"
	default:
		return strings.ToLower(strings.TrimSpace(warp))
	}
}

func warpTraceSummary(warp, gateway string) string {
	w := strings.TrimSpace(warp)
	g := strings.TrimSpace(gateway)
	if w == "" && g == "" {
		return ""
	}
	parts := make([]string, 0, 2)
	if w != "" {
		parts = append(parts, "warp="+w)
	}
	if g != "" {
		parts = append(parts, "gateway="+g)
	}
	return strings.Join(parts, ", ")
}

func parseRegistrationAccountType(raw []byte) string {
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		lower := strings.ToLower(line)
		if !strings.HasPrefix(lower, "account type:") && !strings.HasPrefix(lower, "account:") {
			continue
		}
		if i := strings.IndexByte(line, ':'); i >= 0 {
			return strings.TrimSpace(line[i+1:])
		}
	}
	return ""
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
	warp := strings.TrimSpace(vars["warp"])
	gateway := strings.TrimSpace(vars["gateway"])
	return ExitInfo{
		IP:        ip,
		Country:   strings.TrimSpace(vars["loc"]),
		Region:    strings.TrimSpace(vars["colo"]),
		Warp:      warp,
		Gateway:   gateway,
		WarpTier:  NormalizeWarpTier(warp),
		Org:       warpTraceSummary(warp, gateway),
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
	info := parseCloudflareTrace(out)
	if regOut, err := netnsExec(warpCLI, "--accept-tos", "registration", "show"); err == nil {
		if acct := parseRegistrationAccountType(regOut); acct != "" {
			info.AccountType = acct
		}
	}
	return info
}
