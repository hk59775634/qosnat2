package releasecatalog

import "strings"

// 版本切换下载线路（与 Web/API 的 download_route 字段一致）。
const (
	RouteDirect     = "direct"
	RouteGHProxyV4  = "gh_proxy_v4"
	RouteGHProxyCDN = "gh_proxy_cdn"
	RouteWan1       = "wan_1"
	RouteWan2       = "wan_2"
)

// ValidDownloadRoute 是否为支持的下载线路。
func ValidDownloadRoute(route string) bool {
	switch strings.TrimSpace(route) {
	case "", RouteDirect, RouteGHProxyV4, RouteGHProxyCDN, RouteWan1, RouteWan2:
		return true
	default:
		return false
	}
}

// NormalizeDownloadRoute 空值视为直连。
func NormalizeDownloadRoute(route string) string {
	route = strings.TrimSpace(route)
	if route == "" {
		return RouteDirect
	}
	return route
}

// URLsForRoute 按所选线路构造下载 URL 列表（不再自动串联全部镜像）。
func URLsForRoute(directURL, route string) []string {
	directURL = strings.TrimSpace(directURL)
	if directURL == "" {
		return nil
	}
	route = NormalizeDownloadRoute(route)
	switch route {
	case RouteGHProxyV4:
		return []string{ghProxyPrefixes[0] + directURL}
	case RouteGHProxyCDN:
		return []string{ghProxyPrefixes[1] + directURL}
	case RouteDirect, RouteWan1, RouteWan2:
		return []string{directURL}
	default:
		return MirrorURLs(directURL)
	}
}

// QosnatDownloadURLsForRoute 按线路返回 release 包下载地址。
func QosnatDownloadURLsForRoute(versionID, route string) []string {
	return URLsForRoute(QosnatDownloadURL(versionID), route)
}

// ManifestURLsForRoute 按线路返回 manifest 地址（切换过程中若需重拉清单）。
func ManifestURLsForRoute(product, route string) []string {
	return URLsForRoute(ManifestURL(product), route)
}

// DownloadHostnames 版本切换下载可能访问的域名（用于多 WAN 临时目的地址策略）。
func DownloadHostnames(route string) []string {
	route = NormalizeDownloadRoute(route)
	hosts := []string{
		"github.com",
		"objects.githubusercontent.com",
		"raw.githubusercontent.com",
	}
	switch route {
	case RouteGHProxyV4:
		hosts = append(hosts, "v4.gh-proxy.org")
	case RouteGHProxyCDN:
		hosts = append(hosts, "cdn.gh-proxy.org")
	case RouteDirect, RouteWan1, RouteWan2:
		// 直连 / 多 WAN 出口仍可能经 GitHub 重定向，保留 gh-proxy 域名以便解析
		hosts = append(hosts, "v4.gh-proxy.org", "cdn.gh-proxy.org")
	}
	return hosts
}

// UsesWanEgress 是否需要为多 WAN 添加临时目的地址出口策略。
func UsesWanEgress(route string) bool {
	route = NormalizeDownloadRoute(route)
	return route == RouteWan1 || route == RouteWan2
}

// WanRouteIndex 多 WAN 线路对应的出口序号（1 或 2）。
func WanRouteIndex(route string) int {
	switch NormalizeDownloadRoute(route) {
	case RouteWan1:
		return 1
	case RouteWan2:
		return 2
	default:
		return 0
	}
}
