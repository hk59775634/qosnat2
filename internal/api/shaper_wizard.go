package api

import (
	"github.com/hk59775634/qosnat2/internal/store"
)

type shaperWizardBackup struct {
	profiles     []store.ProfileEntry
	policyCIDR   string
	policyRoutes []string
}

func captureShaperWizardBackup(st store.State) shaperWizardBackup {
	return shaperWizardBackup{
		profiles:     append([]store.ProfileEntry(nil), st.Shaper.Profiles...),
		policyCIDR:   st.Shaper.PolicyCIDR,
		policyRoutes: append([]string(nil), st.Nat.IPv4.PolicyRoutes...),
	}
}

func (srv *Server) revertShaperWizard(b shaperWizardBackup, addedCIDR string) error {
	_ = srv.store.Update(func(st *store.State) {
		st.Shaper.Profiles = append([]store.ProfileEntry(nil), b.profiles...)
		st.Shaper.PolicyCIDR = b.policyCIDR
		st.Nat.IPv4.PolicyRoutes = append([]string(nil), b.policyRoutes...)
	})
	if addedCIDR != "" && srv.bpfReady() {
		_ = srv.bpf.DeleteProfile(addedCIDR)
		if ip, ok := store.ProfileHostIP(addedCIDR); ok {
			_ = srv.bpf.DeleteHost(ip)
		}
	}
	if err := srv.store.Save(); err != nil {
		return err
	}
	if err := srv.reloadNft(); err != nil {
		return err
	}
	srv.refreshShaperAfterChange()
	return nil
}
