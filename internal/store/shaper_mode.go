package store

import "strings"

const ShaperModeEDT = "edt"

// MigrateLegacyShaperMode 升级旧 state：htb 等非 edt 值一律清除（省略 mode 即 EDT）。
func MigrateLegacyShaperMode(sh *ShaperState) {
	if sh == nil {
		return
	}
	mode := strings.ToLower(strings.TrimSpace(sh.Mode))
	if mode == "" || mode == ShaperModeEDT {
		sh.Mode = ""
		return
	}
	sh.Mode = ""
}
