package api

import (
	"net/http"
	"os"
	"strings"

	"github.com/hk59775634/qosnat2/internal/ocserv"
)

func (srv *Server) handleOCServService(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if os.Getuid() != 0 {
		writeJSON(w, http.StatusForbidden, map[string]string{
			"error": "控制 ocserv 服务需要 root 运行 qosnatd（systemd 未降权或使用 sudo 启动服务）",
		})
		return
	}
	var body struct {
		Action string `json:"action"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
		return
	}
	act := strings.TrimSpace(strings.ToLower(body.Action))
	if err := ocserv.ServiceControl(act); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
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
