package singbox

import (
	"encoding/json"
	"os/exec"
	"strings"
	"time"

	"github.com/hk59775634/qosnat2/internal/store"
)

const cloudflareTraceURL = "https://1.1.1.1/cdn-cgi/trace"

// ExitInfo 经 TUN 探测到的代理出口信息（与 WARP exit_info 字段对齐）。
type ExitInfo struct {
	IP        string `json:"ip,omitempty"`
	Country   string `json:"country,omitempty"`
	City      string `json:"city,omitempty"`
	Region    string `json:"region,omitempty"`
	Org       string `json:"org,omitempty"`
	FetchedAt string `json:"fetched_at,omitempty"`
	Error     string `json:"error,omitempty"`
}

// ProbeExitInfo 经代理 TUN 探测出口 IP 与地理位置。
func ProbeExitInfo(p store.ProxyEgress) ExitInfo {
	dev := store.ProxyTunDevice(p.TunIndex)
	if dev == "" || !linkExists(dev) {
		return ExitInfo{Error: "proxy TUN not ready"}
	}
	if _, err := exec.LookPath("curl"); err != nil {
		return ExitInfo{Error: "curl not installed"}
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if info := probeExitViaIPWhois(dev); info.IP != "" {
		info.FetchedAt = now
		return info
	}
	if info := probeExitViaIPAPI(dev); info.IP != "" {
		info.FetchedAt = now
		return info
	}
	if info := probeExitViaCloudflareTrace(dev); info.IP != "" {
		info.FetchedAt = now
		return info
	}
	if ip := ProbeEgressIP(p); ip != "" {
		return ExitInfo{IP: ip, FetchedAt: now}
	}
	return ExitInfo{Error: "proxy test failed: unable to resolve egress IP"}
}

func curlViaIface(dev, url string) ([]byte, error) {
	return exec.Command("curl", "-fsS", "--max-time", "12", "--interface", dev, url).CombinedOutput()
}

func probeExitViaIPWhois(dev string) ExitInfo {
	out, err := curlViaIface(dev, "https://ipwho.is/")
	if err != nil {
		return ExitInfo{Error: trimCurlErr(out, err)}
	}
	var body struct {
		Success     bool   `json:"success"`
		IP          string `json:"ip"`
		Country     string `json:"country"`
		CountryCode string `json:"country_code"`
		Region      string `json:"region"`
		City        string `json:"city"`
		ISP         string `json:"isp"`
		Message     string `json:"message"`
	}
	if json.Unmarshal(out, &body) != nil || !body.Success {
		msg := strings.TrimSpace(body.Message)
		if msg == "" {
			msg = "ipwho.is lookup failed"
		}
		return ExitInfo{Error: msg}
	}
	country := strings.TrimSpace(body.Country)
	if country == "" {
		country = strings.TrimSpace(body.CountryCode)
	}
	return ExitInfo{
		IP:      strings.TrimSpace(body.IP),
		Country: country,
		City:    strings.TrimSpace(body.City),
		Region:  strings.TrimSpace(body.Region),
		Org:     strings.TrimSpace(body.ISP),
	}
}

func probeExitViaIPAPI(dev string) ExitInfo {
	out, err := curlViaIface(dev, "http://ip-api.com/json/?fields=status,message,country,countryCode,regionName,city,query,isp")
	if err != nil {
		return ExitInfo{Error: trimCurlErr(out, err)}
	}
	var body struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		Country string `json:"country"`
		Code    string `json:"countryCode"`
		Region  string `json:"regionName"`
		City    string `json:"city"`
		Query   string `json:"query"`
		ISP     string `json:"isp"`
	}
	if json.Unmarshal(out, &body) != nil || body.Status != "success" {
		msg := strings.TrimSpace(body.Message)
		if msg == "" {
			msg = "ip-api lookup failed"
		}
		return ExitInfo{Error: msg}
	}
	country := strings.TrimSpace(body.Country)
	if country == "" {
		country = strings.TrimSpace(body.Code)
	}
	return ExitInfo{
		IP:      strings.TrimSpace(body.Query),
		Country: country,
		City:    strings.TrimSpace(body.City),
		Region:  strings.TrimSpace(body.Region),
		Org:     strings.TrimSpace(body.ISP),
	}
}

func probeExitViaCloudflareTrace(dev string) ExitInfo {
	out, err := curlViaIface(dev, cloudflareTraceURL)
	if err != nil {
		return ExitInfo{Error: trimCurlErr(out, err)}
	}
	vars := map[string]string{}
	for _, line := range strings.Split(string(out), "\n") {
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
	return ExitInfo{
		IP:      ip,
		Country: strings.TrimSpace(vars["loc"]),
		Region:  strings.TrimSpace(vars["colo"]),
	}
}

func trimCurlErr(out []byte, err error) string {
	msg := strings.TrimSpace(string(out))
	if msg == "" && err != nil {
		msg = err.Error()
	}
	return msg
}
