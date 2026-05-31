package api

import (
	"net/http"
	"strings"

	"github.com/hk59775634/qosnat2/internal/ebpf"
	"github.com/hk59775634/qosnat2/internal/store"
)

// ProfileListItem API 返回的网段模板（含 id，越小优先级越高）
type ProfileListItem struct {
	CIDR    string `json:"cidr"`
	Down    string `json:"down"`
	Up      string `json:"up"`
	Mask    int    `json:"mask,omitempty"`
	ID      int    `json:"id"`
	Device  string `json:"device,omitempty"`
	DownBPS uint64 `json:"down_bps"`
	UpBPS   uint64 `json:"up_bps"`
}

func (srv *Server) listProfileItems() ([]ProfileListItem, error) {
	st := srv.store.Get()
	bpfRates := map[string]ebpf.ProfileEntry{}
	if srv.bpf != nil && srv.bpf.Ready() {
		list, err := srv.bpf.ListProfiles()
		if err != nil {
			return nil, err
		}
		for _, e := range list {
			bpfRates[e.CIDR] = e
		}
	}
	ordered := store.SortProfilesByID(st.Shaper.Profiles)
	out := make([]ProfileListItem, 0, len(ordered))
	for _, p := range ordered {
		item := ProfileListItem{
			CIDR:   p.CIDR,
			Down:   p.Down,
			Up:     p.Up,
			Mask:   p.Mask,
			ID:     p.ID,
			Device: srv.profileDevice(p, st),
		}
		if br, ok := bpfRates[p.CIDR]; ok {
			item.DownBPS = br.DownBPS
			item.UpBPS = br.UpBPS
		} else if rv, err := srv.rateVal(p.Down, p.Up); err == nil {
			item.DownBPS = rv.DownBPS
			item.UpBPS = rv.UpBPS
		}
		out = append(out, item)
	}
	return out, nil
}

func (srv *Server) handleShaperProfilesOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Order []string `json:"order"`
	}
	if err := readJSON(r, &body); err != nil || len(body.Order) == 0 {
		writeBadRequest(w, "order[] required")
		return
	}
	var reordered []store.ProfileEntry
	var err error
	_ = srv.store.Update(func(st *store.State) {
		reordered, err = store.ReorderProfiles(st.Shaper.Profiles, body.Order)
		if err != nil {
			return
		}
		st.Shaper.Profiles = reordered
	})
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	if srv.bpf != nil && srv.bpf.Ready() {
		for _, p := range store.SortProfilesByID(reordered) {
			rv, e := srv.rateVal(p.Down, p.Up)
			if e != nil {
				continue
			}
			_ = srv.syncProfileBPFMaps(p.CIDR, rv)
		}
		srv.rebuildShaperDataPlane()
	}
	if !srv.persistState(w) {
		return
	}
	list, _ := srv.listProfileItems()
	writeJSON(w, http.StatusOK, srv.shaperProfilesPayload(list))
}

func (srv *Server) profileEntryByCIDR(cidr string) (store.ProfileEntry, bool) {
	st := srv.store.Get()
	for _, p := range st.Shaper.Profiles {
		if p.CIDR == cidr {
			return p, true
		}
	}
	return store.ProfileEntry{}, false
}

func (srv *Server) applyProfileHTBProfile(p store.ProfileEntry, rv ebpf.RateVal) {
	_ = p
	_ = rv
	srv.rebuildShaperDataPlane()
}

func (srv *Server) purgeLegacyHostExact(st store.State) {
	if srv.bpf == nil || !srv.bpf.Ready() {
		return
	}
	keep := store.ProfileHost32IPs(st.Shaper.Profiles)
	list, err := srv.bpf.ListHosts()
	if err != nil {
		return
	}
	for _, h := range list {
		if keep[h.IP] {
			continue
		}
		_ = srv.bpf.DeleteHost(h.IP)
	}
}

func (srv *Server) removeProfileHTB(cidr string) {
	st := srv.store.Get()
	var dev string
	for _, p := range st.Shaper.Profiles {
		if p.CIDR == cidr {
			dev = srv.profileDevice(p, st)
			break
		}
	}
	if ip, ok := store.ProfileHostIP(cidr); ok {
		_ = srv.hosts.DeleteHostOnDevice(ip, dev)
		if srv.bpf != nil && srv.bpf.Ready() {
			_ = srv.bpf.DeleteHost(ip)
		}
		return
	}
	for _, p := range st.Shaper.Profiles {
		if p.CIDR == cidr && srv.hosts != nil {
			srv.hosts.RemoveProfileSubnetFromIFB(cidr, p.ID)
			break
		}
	}
}

func (srv *Server) assignProfileOnAdd(st *store.State, cidr, down, up string, mask int, device string) {
	dev := strings.TrimSpace(device)
	found := false
	for i, p := range st.Shaper.Profiles {
		if p.CIDR == cidr {
			st.Shaper.Profiles[i].Down = down
			st.Shaper.Profiles[i].Up = up
			if mask > 0 {
				st.Shaper.Profiles[i].Mask = mask
			}
			if dev != "" {
				st.Shaper.Profiles[i].Device = dev
			}
			found = true
			break
		}
	}
	if !found {
		st.Shaper.Profiles = append(st.Shaper.Profiles, store.ProfileEntry{
			CIDR:   cidr,
			Down:   down,
			Up:     up,
			Mask:   mask,
			Device: dev,
			ID:     store.NextProfileID(st.Shaper.Profiles),
		})
	}
}
