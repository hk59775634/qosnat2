package api

import (
	"log"
	"net/http"
	"strings"

	"github.com/hk59775634/qosnat2/internal/ebpf"
	"github.com/hk59775634/qosnat2/internal/store"
)

func (srv *Server) bpfReady() bool {
	return srv.bpf != nil && srv.bpf.Ready()
}

// syncProfileBPFMaps profile_lpm + /32 写入 host_exact（与 BPF lookup_rate 一致）
func (srv *Server) syncProfileBPFMaps(cidr string, rv ebpf.RateVal) error {
	if err := srv.bpf.UpdateProfile(cidr, rv); err != nil {
		return err
	}
	if ip, ok := store.ProfileHostIP(cidr); ok {
		return srv.bpf.UpdateHost(ip, rv)
	}
	return nil
}

func (srv *Server) rateVal(down, up string) (ebpf.RateVal, error) {
	d, err := store.MbitToBPS(down)
	if err != nil {
		return ebpf.RateVal{}, err
	}
	u, err := store.MbitToBPS(up)
	if err != nil {
		return ebpf.RateVal{}, err
	}
	return ebpf.RateVal{DownBPS: d, UpBPS: u}, nil
}

// upsertShaperProfile 写入 BPF/state；wizard 额外同步 policy_routes 与默认 policy_cidr
// refresh=false 时由调用方批量结束后统一 refreshShaperAfterChange
func (srv *Server) upsertShaperProfile(cidr, down, up string, mask int, device string, wizard bool, refresh bool) (added bool, err error) {
	dev, err := srv.normalizeProfileDevice(device)
	if err != nil {
		return false, err
	}
	if _, err := srv.rateVal(down, up); err != nil {
		return false, err
	}
	// 必须先更新内存 state 再装 TC/BPF；BPF 失败则不持久化。
	stBefore := srv.store.Get()
	backupProfiles := append([]store.ProfileEntry(nil), stBefore.Shaper.Profiles...)
	backupPolicyCIDR := stBefore.Shaper.PolicyCIDR
	backupDefault := stBefore.Shaper.DefaultProfile
	backupRoutes := append([]string(nil), stBefore.Nat.IPv4.PolicyRoutes...)
	_ = srv.store.Update(func(st *store.State) {
		existed := false
		for _, p := range st.Shaper.Profiles {
			if p.CIDR == cidr {
				existed = true
				break
			}
		}
		added = !existed
		srv.assignProfileOnAdd(st, cidr, down, up, mask, dev)
		if wizard {
			if st.Shaper.PolicyCIDR == "" {
				st.Shaper.PolicyCIDR = cidr
				st.Shaper.DefaultProfile = store.RateProfile{Down: down, Up: up, HostMask: mask}
			}
			hasRoute := false
			for _, c := range st.Nat.IPv4.PolicyRoutes {
				if c == cidr {
					hasRoute = true
					break
				}
			}
			if !hasRoute {
				st.Nat.IPv4.PolicyRoutes = append(st.Nat.IPv4.PolicyRoutes, cidr)
			}
		}
	})
	if !srv.shaperEnabled() {
		if err := srv.store.Save(); err != nil {
			return false, err
		}
		return added, nil
	}
	if !srv.bpfReady() {
		return false, errEbpfNotLoaded
	}
	rv, err := srv.rateVal(down, up)
	if err != nil {
		return false, err
	}
	if err := srv.syncProfileBPFMaps(cidr, rv); err != nil {
		_ = srv.store.Update(func(st *store.State) {
			st.Shaper.Profiles = backupProfiles
			st.Shaper.PolicyCIDR = backupPolicyCIDR
			st.Shaper.DefaultProfile = backupDefault
			if wizard {
				st.Nat.IPv4.PolicyRoutes = backupRoutes
			}
		})
		return false, err
	}
	if err := srv.store.Save(); err != nil {
		return false, err
	}
	if refresh {
		srv.refreshShaperAfterChange()
	}
	return added, nil
}

func (srv *Server) handleShaperProfiles(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		list, err := srv.listProfileItems()
		if err != nil {
			writeUnavailable(w, "", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, srv.shaperProfilesPayload(list))
	case http.MethodPut:
		var body struct {
			CIDR   string `json:"cidr"`
			Down   string `json:"down"`
			Up     string `json:"up"`
			Mask   int    `json:"mask"`
			Device string `json:"device"`
		}
		if err := readJSON(r, &body); err != nil {
			writeBadJSON(w)
			return
		}
		if strings.TrimSpace(body.CIDR) == "" {
			writeBadRequest(w, "cidr required")
			return
		}
		if body.Down == "" {
			body.Down = "8mbit"
		}
		if body.Up == "" {
			body.Up = "8mbit"
		}
		if body.Mask == 0 {
			body.Mask = 32
		}
		if srv.shaperEnabled() && !srv.bpfReady() {
			writeUnavailable(w, "", errEbpfNotLoaded.Error())
			return
		}
		added, err := srv.upsertShaperProfile(body.CIDR, body.Down, body.Up, body.Mask, body.Device, false, true)
		if err != nil {
			if err == errEbpfNotLoaded {
				writeUnavailable(w, "", err.Error())
				return
			}
			writeBadRequest(w, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "added": added, "cidr": body.CIDR})
	case http.MethodDelete:
		cidr := r.URL.Query().Get("cidr")
		if cidr == "" {
			writeBadRequest(w, "cidr query required")
			return
		}
		if srv.shaperEnabled() {
			if !srv.bpfReady() {
				writeUnavailable(w, "", errEbpfNotLoaded.Error())
				return
			}
			srv.teardownProfileShaper(cidr)
		}
		_ = srv.store.Update(func(st *store.State) {
			var out []store.ProfileEntry
			for _, p := range st.Shaper.Profiles {
				if p.CIDR != cidr {
					out = append(out, p)
				}
			}
			st.Shaper.Profiles = out
		})
		if !srv.persistState(w) {
			return
		}
		if srv.shaperEnabled() && srv.bpfReady() {
			if err := srv.bpf.DeleteProfile(cidr); err != nil {
				log.Printf("delete profile bpf %s: %v", cidr, err)
			}
			if ip, ok := store.ProfileHostIP(cidr); ok {
				_ = srv.bpf.DeleteHost(ip)
			}
			srv.refreshShaperAfterChange()
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		writeMethodNotAllowed(w)
	}
}

func (srv *Server) handleShaperWizard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	var body struct {
		CIDR   string `json:"cidr"`
		Down   string `json:"down"`
		Up     string `json:"up"`
		Mask   int    `json:"mask"`
		Device string `json:"device"`
	}
	if err := readJSON(r, &body); err != nil {
		writeBadJSON(w)
		return
	}
	if body.CIDR == "" {
		body.CIDR = "10.0.0.0/8"
	}
	if body.Down == "" {
		body.Down = "8mbit"
	}
	if body.Up == "" {
		body.Up = "8mbit"
	}
	if body.Mask == 0 {
		body.Mask = 32
	}
	backup := captureShaperWizardBackup(srv.store.Get())
	if srv.shaperEnabled() && !srv.bpfReady() {
		writeUnavailable(w, "EBPF_NOT_LOADED", errEbpfNotLoaded.Error())
		return
	}
	added, err := srv.upsertShaperProfile(body.CIDR, body.Down, body.Up, body.Mask, body.Device, true, false)
	if err != nil {
		if err == errEbpfNotLoaded {
			writeUnavailable(w, "EBPF_NOT_LOADED", err.Error())
			return
		}
		writeBadRequest(w, err.Error())
		return
	}
	if !srv.shaperEnabled() {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "added": added, "cidr": body.CIDR, "pending_apply": true})
		return
	}
	if err := srv.reloadNft(); err != nil {
		cidrToDrop := ""
		if added {
			cidrToDrop = body.CIDR
		}
		if revErr := srv.revertShaperWizard(backup, cidrToDrop); revErr != nil {
			writeApplyError(w, revErr)
			return
		}
		writeApplyError(w, err)
		return
	}
	srv.refreshShaperAfterChange()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "added": added, "cidr": body.CIDR})
}

func (srv *Server) handleShaperActive(w http.ResponseWriter, r *http.Request) {
	if !srv.bpfReady() {
		writeJSON(w, http.StatusOK, []ebpf.ActiveEntry{})
		return
	}
	list, err := srv.bpf.ListActive()
	if err != nil {
		writeUnavailable(w, "", err.Error())
		return
	}
	if list == nil {
		list = []ebpf.ActiveEntry{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (srv *Server) handleEbpfMaps(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, srv.bpfStatus())
}
