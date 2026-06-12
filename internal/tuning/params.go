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
		Key: "shaper.fq_flows", Category: "QoS 整形", Description: "根 fq 流表大小（0=内核默认 1024；大规模建议 16384）",
		Type: "number", Min: 0, Max: 65536,
	},
	{
		Key: "shaper.fq_quantum", Category: "QoS 整形", Description: "根 fq quantum 字节数（0=内核默认）",
		Type: "number", Min: 0, Max: 65536,
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
		"shaper.fq_flows":         st.Shaper.FQFlows,
		"shaper.fq_quantum":       st.Shaper.FQQuantum,
		"system.txqueuelen_lan":   st.System.TxQueueLenLAN,
		"system.txqueuelen_wan":   st.System.TxQueueLenWAN,
		"system.rps_lan":          st.System.RpsLAN,
		"system.rps_wan":          st.System.RpsWAN,
		"system.perf_preset":      st.System.PerfPreset,
	}
}

// ApplyAppValues 解析 PUT 中的 app 字段
func ApplyAppValues(st *store.State, app map[string]any) {
	if app == nil {
		return
	}
	if v, ok := app["shaper.fq_flows"].(float64); ok && v >= 0 {
		st.Shaper.FQFlows = int(v)
	}
	if v, ok := app["shaper.fq_quantum"].(float64); ok && v >= 0 {
		st.Shaper.FQQuantum = int(v)
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
