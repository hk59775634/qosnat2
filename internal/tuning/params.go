package tuning

import "github.com/hk59775634/qosnat2/internal/store"

// AppParam 非 sysctl 的 QoS/NAT 可调项（Web 表单）
type AppParam struct {
	Key         string   `json:"key"`
	Category    string   `json:"category"`
	Description string   `json:"description"`
	Type        string   `json:"type"` // number, select, bool
	Options     []string `json:"options,omitempty"`
	Min         int      `json:"min,omitempty"`
	Max         int      `json:"max,omitempty"`
}

// AppCatalog 应用层 QoS/NAT 参数列表
var AppCatalog = []AppParam{
	{
		Key: "shaper.leaf", Category: "QoS 整形", Description: "HTB 叶子队列调度器（下行/IFB 根）",
		Type: "select", Options: []string{"fq_codel", "fq"},
	},
	{
		Key: "shaper.idle_timeout_sec", Category: "QoS 整形", Description: "Per-IP 动态队列空闲回收时间（秒）",
		Type: "number", Min: 30, Max: 3600,
	},
	{
		Key: "system.txqueuelen_lan", Category: "网卡", Description: "LAN 发送队列长度（0=应用默认 5000）",
		Type: "number", Min: 0, Max: 10000,
	},
	{
		Key: "system.txqueuelen_wan", Category: "网卡", Description: "WAN 发送队列长度",
		Type: "number", Min: 0, Max: 10000,
	},
	{
		Key: "system.rps_lan", Category: "网卡", Description: "LAN 启用 RPS 多核收包分担",
		Type: "bool",
	},
	{
		Key: "system.rps_wan", Category: "网卡", Description: "WAN 启用 RPS",
		Type: "bool",
	},
	{
		Key: "system.perf_preset", Category: "内核", Description: "合并内置高性能 sysctl 预设（与用户覆盖叠加）",
		Type: "bool",
	},
}

// AppValues 从 state 读出当前应用层参数
func AppValues(st store.State) map[string]any {
	return map[string]any{
		"shaper.leaf":               leafOrDefault(st.Shaper.Leaf),
		"shaper.idle_timeout_sec":   idleOrDefault(st.Shaper.IdleTimeoutSec),
		"system.txqueuelen_lan":     st.System.TxQueueLenLAN,
		"system.txqueuelen_wan":     st.System.TxQueueLenWAN,
		"system.rps_lan":            st.System.RpsLAN,
		"system.rps_wan":            st.System.RpsWAN,
		"system.perf_preset":        st.System.PerfPreset,
	}
}

func leafOrDefault(s string) string {
	if s == "" {
		return "fq_codel"
	}
	return s
}

func idleOrDefault(n int) int {
	if n <= 0 {
		return 300
	}
	return n
}

// ApplyAppValues 解析 PUT 中的 app 字段
func ApplyAppValues(st *store.State, app map[string]any) {
	if app == nil {
		return
	}
	if v, ok := app["shaper.leaf"].(string); ok && v != "" {
		st.Shaper.Leaf = v
	}
	if v, ok := app["shaper.idle_timeout_sec"].(float64); ok {
		st.Shaper.IdleTimeoutSec = int(v)
	}
	if v, ok := app["system.txqueuelen_lan"].(float64); ok {
		st.System.TxQueueLenLAN = int(v)
	}
	if v, ok := app["system.txqueuelen_wan"].(float64); ok {
		st.System.TxQueueLenWAN = int(v)
	}
	if v, ok := app["system.rps_lan"].(bool); ok {
		st.System.RpsLAN = v
	}
	if v, ok := app["system.rps_wan"].(bool); ok {
		st.System.RpsWAN = v
	}
	if v, ok := app["system.perf_preset"].(bool); ok {
		st.System.PerfPreset = v
	}
}
