package store

// ReplayPolicyCIDRToBPF 是否将 policy_cidr + default_profile 写入 BPF。
// profiles 为空时不写入，避免开启 QoS 但未添加策略时仍对整网段隐式限速。
func ReplayPolicyCIDRToBPF(sh ShaperState) bool {
	if len(sh.Profiles) == 0 {
		return false
	}
	if sh.PolicyCIDR == "" || RateProfileUnlimited(sh.DefaultProfile) {
		return false
	}
	return true
}
