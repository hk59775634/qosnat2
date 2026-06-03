package api

import (
	"github.com/hk59775634/qosnat2/internal/netif"
	"github.com/hk59775634/qosnat2/internal/store"
)

func (srv *Server) applyNetplan() (bool, error) {
	st := srv.store.Get()
	applied, err := netif.ApplyNetplan(st.Network)
	if err != nil {
		return applied, err
	}
	// 同步 VLAN 逻辑名
	_ = srv.store.Update(func(st *store.State) {
		for i := range st.Network.VLANs {
			v := &st.Network.VLANs[i]
			if v.Name == "" && v.Parent != "" && v.VID > 0 {
				v.Name = netif.VLANName(v.Parent, v.VID)
			}
		}
		for i := range st.Network.VXLANTunnels {
			t := &st.Network.VXLANTunnels[i]
			if t.Name == "" && t.VNI > 0 {
				t.Name = store.VXLANIfaceName(t.VNI)
			}
		}
	})
	if err := srv.store.Save(); err != nil {
		return applied, err
	}
	return applied, nil
}
