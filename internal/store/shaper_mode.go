package store

import "strings"

const (
	ShaperModeHTB = "htb" // 旧版 IFB + HTB + ringbuf
	ShaperModeEDT = "edt" // Per-IP EDT + token bucket（默认）
)

// EffectiveShaperMode 返回数据面模式；空或未识别时默认 edt。
func EffectiveShaperMode(sh ShaperState) string {
	switch strings.ToLower(strings.TrimSpace(sh.Mode)) {
	case ShaperModeHTB:
		return ShaperModeHTB
	default:
		return ShaperModeEDT
	}
}

func (sh ShaperState) UsesEDT() bool {
	return EffectiveShaperMode(sh) == ShaperModeEDT
}
