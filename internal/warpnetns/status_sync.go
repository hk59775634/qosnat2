package warpnetns

import "strings"

// RefreshConnectedState 根据 netns 运行时探测同步 state.Connected，并返回当前是否已连接。
// 供 status API 使用；勿调用完整 Reconcile()（会误清 connected 或重置 netns）。
func RefreshConnectedState() bool {
	if OpActive() {
		return probeConnectedRuntime()
	}
	if !netnsUsable() {
		if loadState().Connected {
			clearConnectedState()
		}
		return false
	}
	connected := probeConnectedRuntime()
	st := loadState()
	if connected {
		changed := !st.Connected || strings.TrimSpace(st.HostIface) != VethHost
		st.Connected = true
		st.HostIface = VethHost
		if strings.TrimSpace(st.UplinkDev) == "" {
			st.UplinkDev = mainUplinkDev()
		}
		if changed {
			saveState(st)
		}
		return true
	}
	if st.Connected {
		clearConnectedState()
	}
	return false
}

func probeConnectedRuntime() bool {
	if !NetnsHealthy() || !ServiceRunning() || !linkExists(VethHost) {
		return false
	}
	out, err := netnsExec(warpCLI, "--accept-tos", "status")
	if err != nil || !WarpStatusConnected(string(out)) {
		return false
	}
	return warpIfaceInNetns() != ""
}
