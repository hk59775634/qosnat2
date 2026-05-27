package api

import (
	"encoding/json"

	"github.com/hk59775634/qosnat2/internal/ocserv"
	"github.com/hk59775634/qosnat2/internal/store"
)

// 与 ocserv 手册：非 reload 项变更后仅写盘 + reload 不足以让运行中进程加载，需完整重启。

func deepCopyOCServState(o store.OCServState) store.OCServState {
	data, err := json.Marshal(&o)
	if err != nil {
		return o
	}
	var c store.OCServState
	if err := json.Unmarshal(data, &c); err != nil {
		return o
	}
	return c
}

func (srv *Server) updateOcservRestartHints(prev, next store.OCServState) {
	srv.ocservRestartHintsMu.Lock()
	defer srv.ocservRestartHintsMu.Unlock()
	if !ocserv.InstallInfo().Active {
		srv.ocservRestartHints = nil
		return
	}
	certs := srv.store.Get().Certificates
	r := ocserv.NonReloadableChangeReasons(prev, next, certs)
	if len(r) == 0 {
		srv.ocservRestartHints = nil
	} else {
		srv.ocservRestartHints = append([]string(nil), r...)
	}
}

func (srv *Server) ocservRestartHintList() []string {
	srv.ocservRestartHintsMu.Lock()
	defer srv.ocservRestartHintsMu.Unlock()
	if len(srv.ocservRestartHints) == 0 {
		return nil
	}
	out := append([]string(nil), srv.ocservRestartHints...)
	return out
}

func (srv *Server) clearOcservRestartHints() {
	srv.ocservRestartHintsMu.Lock()
	defer srv.ocservRestartHintsMu.Unlock()
	srv.ocservRestartHints = nil
}
