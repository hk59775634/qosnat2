package api

import (
	"fmt"
	"strings"
	"time"

	"github.com/hk59775634/qosnat2/internal/store"
)

func (srv *Server) validateFilterRuleExtras(r store.FilterRule, st store.State) error {
	if sid := strings.TrimSpace(r.ScheduleID); sid != "" {
		if _, ok := store.FindSchedule(st.Firewall.Schedules, sid); !ok {
			return fmt.Errorf("schedule_id %q not found", sid)
		}
	}
	if wid := strings.TrimSpace(r.WanLinkID); wid != "" {
		if _, ok := store.FindWanLink(st.Network.WanLinks, wid); !ok {
			return fmt.Errorf("wan_link_id %q not found", wid)
		}
	}
	return nil
}

// syncFirewallDerivedPolicies 在防火墙规则落地后同步自动 egress 与规则 QoS profiles。
func (srv *Server) syncFirewallDerivedPolicies() (egressChanged, shaperChanged bool) {
	_ = srv.store.Update(func(st *store.State) {
		egressChanged = store.SyncFirewallGatewayEgress(st)
		shaperChanged = store.SyncFirewallShaperProfiles(st)
	})
	return egressChanged, shaperChanged
}

func (srv *Server) applyFirewallDerivedSideEffects() {
	egressChanged, shaperChanged := srv.syncFirewallDerivedPolicies()
	_ = srv.store.Save()
	if egressChanged {
		if err := srv.applyWanLinkDataPlane(); err != nil {
			// 不回滚防火墙；记录即可
			_ = err
		}
	}
	if shaperChanged {
		srv.refreshShaperAfterChange()
	}
}

func firewallNftLines(rules []store.FilterRule, schedules []store.Schedule) map[string]string {
	now := time.Now()
	out := make(map[string]string, len(rules))
	for _, r := range rules {
		if line := r.NftRuleLineAt(schedules, now); line != "" {
			out[r.ID] = line
		}
	}
	return out
}
