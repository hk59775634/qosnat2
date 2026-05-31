package api

import (
	"net/http"
	"os"
	"strings"

	"github.com/hk59775634/qosnat2/internal/ocserv"
)

func (srv *Server) handleOCServService(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	if os.Getuid() != 0 {
		writeForbidden(w, "ROOT_REQUIRED", "控制 ocserv 服务需要 root 运行 qosnatd（systemd 未降权或使用 sudo 启动服务）")
		return
	}
	var body struct {
		Action string `json:"action"`
	}
	if err := readJSON(r, &body); err != nil {
		writeBadJSON(w)
		return
	}
	act := strings.TrimSpace(strings.ToLower(body.Action))
	if err := ocserv.ServiceControl(act); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	srv.clearOcservRestartHints()
	nftWarn := srv.tryReloadNft()
	srv.auditLog(r, "vpn.ocserv.service", act)
	resp := map[string]any{"ok": true, "action": act}
	if nftWarn != "" {
		resp["nft_warning"] = nftWarn
	}
	writeJSON(w, http.StatusOK, resp)
}
