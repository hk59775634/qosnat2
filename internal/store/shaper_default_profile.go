package store

// MigrateStripDefaultProfileRates 清除 default_profile 隐式速率；QoS 仅以 profiles 为准。
func MigrateStripDefaultProfileRates(sh *ShaperState) bool {
	if sh == nil {
		return false
	}
	if sh.DefaultProfile.Down == "" && sh.DefaultProfile.Up == "" {
		return false
	}
	sh.DefaultProfile.Down = ""
	sh.DefaultProfile.Up = ""
	return true
}
