package api

// StopBackground 停止 metrics/流量采样/WARP watchdog/证书后台等 goroutine。
func (srv *Server) StopBackground() {
	if srv.bootApplyCancel != nil {
		srv.bootApplyCancel()
	}
	if srv.metricsCancel != nil {
		srv.metricsCancel()
		srv.metricsCancel = nil
	}
	if srv.ocservTrafficCancel != nil {
		srv.ocservTrafficCancel()
		srv.ocservTrafficCancel = nil
	}
	if srv.wgTrafficCancel != nil {
		srv.wgTrafficCancel()
		srv.wgTrafficCancel = nil
	}
	if srv.warpWatchCancel != nil {
		srv.warpWatchCancel()
		srv.warpWatchCancel = nil
	}
	if srv.proxyWatchCancel != nil {
		srv.proxyWatchCancel()
		srv.proxyWatchCancel = nil
	}
	if srv.serviceBgCancel != nil {
		srv.serviceBgCancel()
		srv.serviceBgCancel = nil
	}
}
