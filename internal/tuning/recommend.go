package tuning

import (
	"fmt"
	"strconv"

	"github.com/hk59775634/qosnat2/internal/store"
)

// QoSParams 与 QoS/NAT 数据面相关的应用层参数（存 state.shaper / system）
type QoSParams struct {
	FQFlows   int `json:"fq_flows"`
	FQQuantum int `json:"fq_quantum"`
}

// Result 推荐调优结果
type Result struct {
	Tier           Tier              `json:"tier"`
	Sysctl         map[string]string `json:"sysctl"`
	PerfPreset     bool              `json:"perf_preset"`
	TxQueueLenLAN  int               `json:"txqueuelen_lan"`
	TxQueueLenWAN  int               `json:"txqueuelen_wan"`
	RpsLAN         bool              `json:"rps_lan"`
	RpsWAN         bool              `json:"rps_wan"`
	QoS            QoSParams         `json:"qos"`
	MemoryBudget   MemoryBudget      `json:"memory_budget"`
}

// Recommend 根据宿主机容量生成 QoS/NAT 推荐值
func Recommend(h HostInfo) Result {
	tier := ClassifyTier(h)
	budget := ConntrackBudget(h)
	r := Result{
		Tier:         tier,
		Sysctl:       map[string]string{},
		MemoryBudget: budget,
	}
	switch tier {
	case TierLow:
		r.PerfPreset = false
		r.TxQueueLenLAN = 1000
		r.TxQueueLenWAN = 1000
		r.RpsLAN = false
		r.RpsWAN = false
		r.QoS = QoSParams{FQFlows: 0, FQQuantum: 0}
		r.Sysctl = lowSysctl(h, budget)
	case TierMedium:
		r.PerfPreset = true
		r.TxQueueLenLAN = 2500
		r.TxQueueLenWAN = 2500
		r.RpsLAN = true
		r.RpsWAN = true // NAT 转发下行经 WAN 收包，须与 LAN 同样启用 RPS
		r.QoS = QoSParams{FQFlows: 0, FQQuantum: 0}
		r.Sysctl = mediumSysctl(h, budget)
	default:
		r.PerfPreset = true
		r.TxQueueLenLAN = 5000
		r.TxQueueLenWAN = 5000
		r.RpsLAN = true
		r.RpsWAN = true
		r.QoS = QoSParams{FQFlows: 16384, FQQuantum: 0}
		r.Sysctl = highSysctl(h, budget)
	}
	return r
}

func lowSysctl(h HostInfo, budget MemoryBudget) map[string]string {
	m := map[string]string{
		"net.core.netdev_max_backlog":                        "65536",
		"net.core.somaxconn":                                 "8192",
		"net.core.netdev_budget":                             "300",
		"net.netfilter.nf_conntrack_tcp_timeout_established": "3600",
		"net.netfilter.nf_conntrack_tcp_timeout_time_wait":   "60",
		"net.ipv4.tcp_fin_timeout":                           "45",
		"net.ipv4.tcp_max_syn_backlog":                       "4096",
	}
	applyConntrackSysctl(m, budget)
	scaleNeigh(m, budget.ConntrackMax)
	return m
}

func mediumSysctl(h HostInfo, budget MemoryBudget) map[string]string {
	m := map[string]string{
		"net.core.netdev_max_backlog":                        "100000",
		"net.core.somaxconn":                                 "32768",
		"net.core.netdev_budget":                             "450",
		"net.core.rps_sock_flow_entries":                     "16384",
		"net.netfilter.nf_conntrack_tcp_timeout_established": "14400",
		"net.netfilter.nf_conntrack_tcp_timeout_time_wait":   "45",
		"net.ipv4.tcp_fin_timeout":                           "30",
		"net.ipv4.tcp_tw_reuse":                              "1",
		"net.ipv4.tcp_max_syn_backlog":                       "8192",
		"net.ipv4.ip_local_port_range":                       "1024 65535",
	}
	applyConntrackSysctl(m, budget)
	scaleNeigh(m, budget.ConntrackMax)
	return m
}

func highSysctl(h HostInfo, budget MemoryBudget) map[string]string {
	m := map[string]string{
		"net.core.netdev_max_backlog":                        "250000",
		"net.core.somaxconn":                                 "65535",
		"net.core.netdev_budget":                             "600",
		"net.core.rmem_max":                                  scaleSocketBuf(h.MemMB, 134217728),
		"net.core.wmem_max":                                  scaleSocketBuf(h.MemMB, 134217728),
		"net.core.rmem_default":                              "1048576",
		"net.core.wmem_default":                              "1048576",
		"net.core.rps_sock_flow_entries":                     scaleRPSFlow(h.MemMB),
		"net.netfilter.nf_conntrack_tcp_timeout_established": "7200",
		"net.netfilter.nf_conntrack_tcp_timeout_time_wait":     "30",
		"net.ipv4.tcp_fin_timeout":                           "30",
		"net.ipv4.tcp_tw_reuse":                              "1",
		"net.ipv4.tcp_max_syn_backlog":                       scaleSynBacklog(h.MemMB),
		"net.ipv4.tcp_slow_start_after_idle":                 "0",
		"net.ipv4.ip_local_port_range":                       "1024 65535",
	}
	applyConntrackSysctl(m, budget)
	scaleNeigh(m, budget.ConntrackMax)
	return m
}

func scaleSocketBuf(memMB int64, def int) string {
	if memMB >= 16384 {
		return strconv.Itoa(268435456)
	}
	if memMB >= 8192 {
		return strconv.Itoa(134217728)
	}
	return strconv.Itoa(def)
}

func scaleRPSFlow(memMB int64) string {
	if memMB >= 16384 {
		return "32768"
	}
	if memMB >= 4096 {
		return "16384"
	}
	return "8192"
}

func scaleSynBacklog(memMB int64) string {
	if memMB >= 16384 {
		return "32768"
	}
	if memMB >= 8192 {
		return "16384"
	}
	return "8192"
}

// ApplyResult 将推荐写入 state（不覆盖用户已手动保存的 sysctl 时由 onlyIfEmpty 控制）
func ApplyResult(st *store.State, rec Result, onlyIfEmpty bool) bool {
	if onlyIfEmpty && st.System.TuningAutoApplied {
		return false
	}
	if st.System.Sysctl == nil {
		st.System.Sysctl = map[string]string{}
	}
	if onlyIfEmpty && len(st.System.Sysctl) == 0 {
		for k, v := range rec.Sysctl {
			st.System.Sysctl[k] = v
		}
	} else if !onlyIfEmpty {
		for k, v := range rec.Sysctl {
			st.System.Sysctl[k] = v
		}
	}
	st.System.PerfPreset = rec.PerfPreset
	if onlyIfEmpty || st.System.TxQueueLenLAN == 0 {
		st.System.TxQueueLenLAN = rec.TxQueueLenLAN
	}
	if onlyIfEmpty || st.System.TxQueueLenWAN == 0 {
		st.System.TxQueueLenWAN = rec.TxQueueLenWAN
	}
	if onlyIfEmpty {
		st.System.RpsLAN = rec.RpsLAN
		st.System.RpsWAN = rec.RpsWAN
	}
	if rec.QoS.FQFlows >= 0 {
		st.Shaper.FQFlows = rec.QoS.FQFlows
	}
	if rec.QoS.FQQuantum >= 0 {
		st.Shaper.FQQuantum = rec.QoS.FQQuantum
	}
	st.System.TuningAutoApplied = true
	st.System.TuningTier = string(rec.Tier)
	return true
}

// AutoApplyOnSetup 初次引导完成时按硬件写入推荐值
func AutoApplyOnSetup(st *store.State) Result {
	h := DetectHost()
	rec := Recommend(h)
	ApplyResult(st, rec, true)
	return rec
}

// TierLabel 中文档位说明
func TierLabel(t Tier) string {
	switch t {
	case TierLow:
		return fmt.Sprintf("低配（≤2 核或 <2GB 内存）")
	case TierMedium:
		return fmt.Sprintf("中配（≤4 核或 <8GB 内存）")
	default:
		return fmt.Sprintf("高配")
	}
}
