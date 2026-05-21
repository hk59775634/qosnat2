package api

import (
	"net"
	"net/http"
	"strings"

	"github.com/hk59775634/qosnat2/internal/ebpf"
	"github.com/hk59775634/qosnat2/internal/store"
)

func (srv *Server) handleShaperHosts(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/shaper/hosts")
	path = strings.Trim(path, "/")
	if path == "" {
		switch r.Method {
		case http.MethodGet:
			srv.listShaperHosts(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}
	ip := path
	switch r.Method {
	case http.MethodGet:
		srv.getShaperHost(w, r, ip)
	case http.MethodPut:
		srv.putShaperHost(w, r, ip)
	case http.MethodDelete:
		srv.deleteShaperHost(w, r, ip)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (srv *Server) listShaperHosts(w http.ResponseWriter, _ *http.Request) {
	if !srv.bpfReady() {
		writeJSON(w, http.StatusOK, []hostListItem{})
		return
	}
	bpfHosts, err := srv.bpf.ListHosts()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}
	bpfMap := map[string]ebpf.HostEntry{}
	for _, h := range bpfHosts {
		bpfMap[h.IP] = h
	}
	st := srv.store.Get()
	seen := map[string]bool{}
	var out []hostListItem
	for _, p := range st.Shaper.Profiles {
		ip, ok := store.ProfileHostIP(p.CIDR)
		if !ok {
			continue
		}
		item := hostListItem{
			IP:     ip,
			Down:   p.Down,
			Up:     p.Up,
			Device: srv.profileDevice(p, st),
		}
		if bh, ok := bpfMap[ip]; ok {
			item.DownBPS = bh.DownBPS
			item.UpBPS = bh.UpBPS
		} else if rv, e := srv.rateVal(p.Down, p.Up); e == nil {
			item.DownBPS = rv.DownBPS
			item.UpBPS = rv.UpBPS
		}
		out = append(out, item)
		seen[ip] = true
	}
	for _, h := range bpfHosts {
		if seen[h.IP] {
			continue
		}
		out = append(out, hostListItem{IP: h.IP, DownBPS: h.DownBPS, UpBPS: h.UpBPS})
	}
	if out == nil {
		out = []hostListItem{}
	}
	writeJSON(w, http.StatusOK, out)
}

type hostListItem struct {
	IP      string `json:"ip"`
	Down    string `json:"down,omitempty"`
	Up      string `json:"up,omitempty"`
	Device  string `json:"device,omitempty"`
	DownBPS uint64 `json:"down_bps,omitempty"`
	UpBPS   uint64 `json:"up_bps,omitempty"`
}

func (srv *Server) getShaperHost(w http.ResponseWriter, _ *http.Request, ip string) {
	if net.ParseIP(ip) == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid ip"})
		return
	}
	if !srv.bpfReady() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": errEbpfNotLoaded.Error()})
		return
	}
	down, up, ok := srv.bpf.LookupRates(ip)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "host not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ip":       ip,
		"down_bps": down,
		"up_bps":   up,
	})
}

func (srv *Server) putShaperHost(w http.ResponseWriter, r *http.Request, ip string) {
	if net.ParseIP(ip) == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid ip"})
		return
	}
	var body struct {
		Down   string `json:"down"`
		Up     string `json:"up"`
		Device string `json:"device"`
	}
	if err := readJSON(r, &body); err != nil || body.Down == "" || body.Up == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "down/up required"})
		return
	}
	if err := srv.upsertShaperHost(ip, body.Down, body.Up, body.Device); err != nil {
		if err == errEbpfNotLoaded {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	srv.auditLog(r, "shaper.host.put", ip+" down="+body.Down+" up="+body.Up)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (srv *Server) deleteShaperHost(w http.ResponseWriter, r *http.Request, ip string) {
	if net.ParseIP(ip) == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid ip"})
		return
	}
	if err := srv.removeShaperHost(ip); err != nil && err != errEbpfNotLoaded {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	srv.auditLog(r, "shaper.host.delete", ip)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (srv *Server) upsertShaperHost(ip, down, up, device string) error {
	if !srv.bpfReady() {
		return errEbpfNotLoaded
	}
	rv, err := srv.rateVal(down, up)
	if err != nil {
		return err
	}
	if err := srv.bpf.UpdateHost(ip, rv); err != nil {
		return err
	}
	dev, err := srv.normalizeProfileDevice(device)
	if err != nil {
		return err
	}
	srv.ensureShaperDevice(dev)
	_ = srv.hosts.EnsureHostOnDevice(ip, rv.DownBPS, rv.UpBPS, rv.ClassMinor, dev)
	srv.reattachShaperDataPath()
	cidr := store.Host32ProfileCIDR(ip)
	_ = srv.store.Update(func(st *store.State) {
		srv.assignProfileOnAdd(st, cidr, down, up, 32, dev)
	})
	_ = srv.store.Save()
	srv.refreshShaperAfterChange()
	return nil
}

func (srv *Server) removeShaperHost(ip string) error {
	st := srv.store.Get()
	dev := srv.shaperDefaultDevice(st)
	for _, p := range st.Shaper.Profiles {
		if pip, ok := store.ProfileHostIP(p.CIDR); ok && pip == ip {
			dev = srv.profileDevice(p, st)
			break
		}
	}
	_ = srv.hosts.DeleteHostOnDevice(ip, dev)
	if srv.bpfReady() {
		_ = srv.bpf.DeleteHost(ip)
	}
	cidr := store.Host32ProfileCIDR(ip)
	_ = srv.store.Update(func(st *store.State) {
		var out []store.ProfileEntry
		for _, p := range st.Shaper.Profiles {
			if p.CIDR != cidr {
				out = append(out, p)
			}
		}
		st.Shaper.Profiles = out
	})
	_ = srv.store.Save()
	if srv.bpfReady() {
		_ = srv.bpf.ReplayState(srv.store.Get())
	}
	srv.refreshShaperAfterChange()
	return nil
}
