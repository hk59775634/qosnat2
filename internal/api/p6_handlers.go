package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hk59775634/qosnat2/internal/capture"
	"github.com/hk59775634/qosnat2/internal/conntrack"
)

func (srv *Server) captures() *capture.Manager {
	if srv.pcap == nil {
		srv.pcap = capture.New("")
	}
	return srv.pcap
}

func (srv *Server) handleCaptures(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/v1/diagnostics/captures/") {
		id := strings.TrimPrefix(r.URL.Path, "/api/v1/diagnostics/captures/")
		if strings.HasSuffix(id, "/download") {
			id = strings.TrimSuffix(id, "/download")
			srv.handleCaptureDownload(w, r, id)
			return
		}
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, srv.captures().List())
	case http.MethodPost:
		var body struct {
			Device      string `json:"device"`
			Filter      string `json:"filter"`
			DurationSec int    `json:"duration_sec"`
		}
		if err := readJSON(r, &body); err != nil {
			writeBadJSON(w)
			return
		}
		dev := body.Device
		if dev == "" {
			dev = srv.env.DevLAN
		}
		if err := validateCaptureDevice(dev); err != nil {
			writeBadRequest(w, err.Error())
			return
		}
		if err := sanitizeTcpdumpFilter(body.Filter); err != nil {
			writeBadRequest(w, err.Error())
			return
		}
		s, err := srv.captures().Start(dev, body.Filter, body.DurationSec)
		if err != nil {
			writeInternalError(w, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, s)
	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			writeBadRequest(w, "id required")
			return
		}
		_ = srv.captures().Stop(id)
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (srv *Server) handleCaptureDownload(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s, err := srv.captures().Get(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if _, err := os.Stat(s.File); err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/vnd.tcpdump.pcap")
	w.Header().Set("Content-Disposition", "attachment; filename="+filepath.Base(s.File))
	http.ServeFile(w, r, s.File)
}

func (srv *Server) handleConntrack(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	limit := 200
	if s := r.URL.Query().Get("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			limit = n
		}
	}
	filter := r.URL.Query().Get("filter")
	res, err := conntrack.List(limit, filter)
	if err != nil {
		writeUnavailable(w, "", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}
