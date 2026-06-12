package store

import "encoding/json"

// ShaperImpliesEnabled 旧版 state 无 enabled 字段时，若已有 QoS 配置则视为启用。
func ShaperImpliesEnabled(sh ShaperState) bool {
	if len(sh.Profiles) > 0 {
		return true
	}
	if len(sh.Tenants) > 0 {
		return true
	}
	return false
}

// MigrateShaperEnabled 从磁盘 JSON 迁移：仅当 shaper 块存在且未写入 enabled 时推断。
func MigrateShaperEnabled(rawShaper json.RawMessage, sh *ShaperState) {
	if len(rawShaper) == 0 || sh == nil {
		return
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(rawShaper, &m); err != nil {
		return
	}
	if _, ok := m["enabled"]; ok {
		return
	}
	if ShaperImpliesEnabled(*sh) {
		sh.Enabled = true
	}
}
