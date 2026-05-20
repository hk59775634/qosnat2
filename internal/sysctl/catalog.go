package sysctl

import (
	"os/exec"
	"strings"
)

// Entry 可调内核参数说明（供 Web UI 展示）
type Entry struct {
	Key         string `json:"key"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Default     string `json:"default"`
	Performance string `json:"performance,omitempty"`
}

// Catalog 有序列表：QoS/NAT 网关相关 sysctl
var Catalog = []Entry{
	// 转发 / NAT
	{Key: "net.ipv4.ip_forward", Category: "NAT / 转发", Description: "IPv4 转发（网关必需）", Default: "1"},
	{Key: "net.ipv4.conf.all.forwarding", Category: "NAT / 转发", Description: "所有接口 IPv4 转发", Default: "1", Performance: "1"},
	{Key: "net.ipv4.conf.default.forwarding", Category: "NAT / 转发", Description: "新接口默认转发", Default: "1", Performance: "1"},
	{Key: "net.ipv4.conf.all.rp_filter", Category: "NAT / 转发", Description: "关闭反向路径过滤，避免非对称路由丢包", Default: "0"},
	{Key: "net.ipv4.conf.default.rp_filter", Category: "NAT / 转发", Description: "新接口默认 rp_filter", Default: "0", Performance: "0"},
	// 套接字 / 队列缓冲
	{Key: "net.core.rmem_max", Category: "缓冲区", Description: "套接字接收缓冲区上限（字节）", Default: "134217728", Performance: "134217728"},
	{Key: "net.core.wmem_max", Category: "缓冲区", Description: "套接字发送缓冲区上限", Default: "134217728", Performance: "134217728"},
	{Key: "net.core.rmem_default", Category: "缓冲区", Description: "默认接收缓冲区", Default: "212992", Performance: "1048576"},
	{Key: "net.core.wmem_default", Category: "缓冲区", Description: "默认发送缓冲区", Default: "212992", Performance: "1048576"},
	{Key: "net.core.optmem_max", Category: "缓冲区", Description: "辅助缓冲区上限（timestamps 等）", Default: "20480", Performance: "65536"},
	{Key: "net.core.netdev_max_backlog", Category: "缓冲区", Description: "网卡入队 backlog，高 PPS 时增大", Default: "1000", Performance: "250000"},
	{Key: "net.core.somaxconn", Category: "缓冲区", Description: "listen 队列上限", Default: "4096", Performance: "65535"},
	{Key: "net.core.netdev_budget", Category: "缓冲区", Description: "每轮 NAPI 处理包数预算", Default: "300", Performance: "600"},
	{Key: "fs.file-max", Category: "缓冲区", Description: "系统最大文件句柄数（大量连接时提高）", Default: "100000", Performance: "2097152"},
	// conntrack / NAT 会话
	{Key: "net.netfilter.nf_conntrack_max", Category: "连接跟踪", Description: "最大 conntrack 条目", Default: "262144", Performance: "2097152"},
	{Key: "net.netfilter.nf_conntrack_buckets", Category: "连接跟踪", Description: "conntrack 哈希桶（通常为 max/4）", Default: "65536", Performance: "524288"},
	{Key: "net.netfilter.nf_conntrack_tcp_timeout_established", Category: "连接跟踪", Description: "ESTABLISHED TCP 超时（秒）", Default: "432000", Performance: "7200"},
	{Key: "net.netfilter.nf_conntrack_tcp_timeout_time_wait", Category: "连接跟踪", Description: "TIME_WAIT 超时（秒）", Default: "120", Performance: "30"},
	{Key: "net.netfilter.nf_conntrack_udp_timeout", Category: "连接跟踪", Description: "UDP 会话超时（秒）", Default: "30", Performance: "60"},
	{Key: "net.netfilter.nf_conntrack_udp_timeout_stream", Category: "连接跟踪", Description: "UDP stream 超时", Default: "180", Performance: "120"},
	// TCP / 网关
	{Key: "net.ipv4.tcp_fin_timeout", Category: "TCP", Description: "FIN_WAIT 超时（秒）", Default: "60", Performance: "30"},
	{Key: "net.ipv4.tcp_tw_reuse", Category: "TCP", Description: "TIME_WAIT 套接字复用", Default: "0", Performance: "1"},
	{Key: "net.ipv4.tcp_max_syn_backlog", Category: "TCP", Description: "SYN 半连接队列", Default: "4096", Performance: "16384"},
	{Key: "net.ipv4.tcp_syncookies", Category: "TCP", Description: "SYN 洪水防护", Default: "1", Performance: "1"},
	{Key: "net.ipv4.tcp_slow_start_after_idle", Category: "TCP", Description: "空闲后慢启动（0=高吞吐网关常用）", Default: "1", Performance: "0"},
	{Key: "net.ipv4.ip_local_port_range", Category: "TCP", Description: "本地临时端口范围", Default: "32768 60999", Performance: "1024 65535"},
	// 邻居表（大量内网主机）
	{Key: "net.ipv4.neigh.default.gc_thresh1", Category: "邻居表", Description: "ARP 缓存软阈值", Default: "128", Performance: "4096"},
	{Key: "net.ipv4.neigh.default.gc_thresh2", Category: "邻居表", Description: "ARP 缓存硬阈值", Default: "512", Performance: "8192"},
	{Key: "net.ipv4.neigh.default.gc_thresh3", Category: "邻居表", Description: "ARP 缓存最大条目", Default: "1024", Performance: "16384"},
	// RPS / 多队列
	{Key: "net.core.rps_sock_flow_entries", Category: "RPS / 多队列", Description: "RPS 流表大小（启用 RPS 时设置）", Default: "0", Performance: "32768"},
}

// PerformancePreset 内置高性能 sysctl 覆盖
var PerformancePreset = map[string]string{
	"net.ipv4.conf.all.forwarding":                       "1",
	"net.ipv4.conf.default.forwarding":                   "1",
	"net.ipv4.conf.default.rp_filter":                    "0",
	"net.core.rmem_default":                              "1048576",
	"net.core.wmem_default":                              "1048576",
	"net.core.optmem_max":                                "65536",
	"net.core.netdev_max_backlog":                        "250000",
	"net.core.somaxconn":                                 "65535",
	"net.core.netdev_budget":                             "600",
	"net.netfilter.nf_conntrack_tcp_timeout_established": "7200",
	"net.netfilter.nf_conntrack_tcp_timeout_time_wait":   "30",
	"net.netfilter.nf_conntrack_udp_timeout":             "60",
	"net.ipv4.tcp_fin_timeout":                           "30",
	"net.ipv4.tcp_tw_reuse":                              "1",
	"net.ipv4.tcp_max_syn_backlog":                       "16384",
	"net.ipv4.tcp_slow_start_after_idle":                 "0",
	"net.ipv4.ip_local_port_range":                       "1024 65535",
	"net.ipv4.neigh.default.gc_thresh1":                   "4096",
	"net.ipv4.neigh.default.gc_thresh2":                   "8192",
	"net.ipv4.neigh.default.gc_thresh3":                   "16384",
	"net.core.rps_sock_flow_entries":                     "32768",
	"fs.file-max":                                        "2097152",
}

// Merge 合并 Defaults、性能预设与用户覆盖，后者优先
func Merge(extra map[string]string, usePerformance bool) map[string]string {
	out := make(map[string]string, len(Defaults)+len(PerformancePreset)+len(extra))
	for k, v := range Defaults {
		out[k] = v
	}
	if usePerformance {
		for k, v := range PerformancePreset {
			out[k] = v
		}
	}
	for k, v := range extra {
		if v == "" {
			delete(out, k)
			continue
		}
		out[k] = v
	}
	return out
}

// ReadLive 读取当前内核中的值（sysctl -n）
func ReadLive(keys []string) map[string]string {
	out := make(map[string]string, len(keys))
	for _, k := range keys {
		out[k] = readOne(k)
	}
	return out
}

func readOne(key string) string {
	out, err := exec.Command("sysctl", "-n", key).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
