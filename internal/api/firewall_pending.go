package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/hk59775634/qosnat2/internal/store"
)

type firewallChangesPayload struct {
	HasPendingChanges bool                     `json:"has_pending_changes"`
	CanApply          bool                     `json:"can_apply"`
	Issues            []store.FirewallChangeIssue `json:"issues,omitempty"`
	Diff              store.FilterRulesDiff    `json:"diff,omitempty"`
	NftValid          bool                     `json:"nft_valid"`
	NftError          string                   `json:"nft_error,omitempty"`
	NftHint           string                   `json:"nft_hint,omitempty"`
}

func (srv *Server) appliedFilterRules(st store.State) []store.FilterRule {
	return append([]store.FilterRule(nil), st.Firewall.FilterRules...)
}

func (srv *Server) syncFilterRulesForState(st store.State, rules []store.FilterRule) []store.FilterRule {
	patched := st
	patched.Firewall.FilterRules = rules
	patched = srv.syncedFirewallState(patched)
	return patched.Firewall.FilterRules
}

func (srv *Server) ensurePendingFilterDraft() {
	st := srv.store.Get()
	if st.Firewall.PendingFilterDraft {
		return
	}
	_ = srv.store.Update(func(st *store.State) {
		st.Firewall.PendingFilterRules = store.CloneFilterRules(st.Firewall.FilterRules)
		st.Firewall.PendingFilterDraft = true
	})
}

func (srv *Server) refreshPendingAutoRules() {
	st := srv.store.Get()
	if !st.Firewall.PendingFilterDraft {
		return
	}
	synced := srv.syncFilterRulesForState(st, st.Firewall.PendingFilterRules)
	_ = srv.store.Update(func(st *store.State) {
		st.Firewall.PendingFilterRules = synced
	})
}

func (srv *Server) workingFilterRules(st store.State) []store.FilterRule {
	if st.Firewall.PendingFilterDraft {
		return append([]store.FilterRule(nil), st.Firewall.PendingFilterRules...)
	}
	return srv.appliedFilterRules(st)
}

func (srv *Server) discardPendingFilterDraft() {
	_ = srv.store.Update(func(st *store.State) {
		st.Firewall.PendingFilterDraft = false
		st.Firewall.PendingFilterRules = nil
	})
}

func (srv *Server) setPendingFilterRules(rules []store.FilterRule) {
	_ = srv.store.Update(func(st *store.State) {
		st.Firewall.PendingFilterRules = rules
		st.Firewall.PendingFilterDraft = true
	})
}

func (srv *Server) buildFirewallChangesPayload(st store.State) firewallChangesPayload {
	applied := srv.appliedFilterRules(st)
	pending := srv.workingFilterRules(st)
	hasPending := st.Firewall.PendingFilterDraft && !store.FilterRulesEqual(applied, pending)
	out := firewallChangesPayload{HasPendingChanges: hasPending}
	if !hasPending {
		out.NftValid = true
		out.CanApply = false
		return out
	}
	issues, diff := store.AuditFilterRulesChange(applied, pending, st.Firewall.Aliases, srv.env.AdminPort, srv.env.DevLAN, srv.env.DevWAN)
	out.Issues = issues
	out.Diff = diff
	proposed := st
	proposed.Firewall.FilterRules = pending
	if err := srv.checkNftForState(proposed); err != nil {
		out.NftValid = false
		out.NftError = err.Error()
		out.NftHint = nftApplyHint(err)
		issues = append(issues, store.FirewallChangeIssue{
			Code:     "NFT_INVALID",
			Severity: "error",
			Message:  err.Error(),
			Hint:     out.NftHint,
		})
		out.Issues = issues
	} else {
		out.NftValid = true
	}
	out.CanApply = out.NftValid && !store.ChangesHaveErrors(out.Issues)
	return out
}

func nftApplyHint(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "syntax error"):
		return "check rule fields (proto, ports, CIDR); use dry-run preview on the rule form"
	case strings.Contains(msg, "unknown identifier"):
		return "referenced interface or alias may be missing; verify iif/oif and alias names"
	case strings.Contains(msg, "operation not permitted"):
		return "nft check skipped in this environment; apply on the gateway host to validate"
	default:
		return "fix reported nft error or discard pending changes and retry one rule at a time"
	}
}

func (srv *Server) stageFilterRulesResponse(w http.ResponseWriter, r *http.Request, auditAction string, auditDetail string) {
	st := srv.store.Get()
	changes := srv.buildFirewallChangesPayload(st)
	resp := map[string]any{
		"ok":      true,
		"staged":  true,
		"rules":   srv.workingFilterRules(st),
		"changes": changes,
	}
	if !changes.CanApply && len(changes.Issues) > 0 {
		resp["warning"] = "pending changes have compliance or nft issues; fix before Apply"
	}
	srv.auditLog(r, auditAction, auditDetail)
	writeJSON(w, http.StatusOK, resp)
}

func (srv *Server) handleFirewallApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	st := srv.store.Get()
	if !st.Firewall.PendingFilterDraft {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "applied": false, "message": "no pending changes"})
		return
	}
	srv.refreshPendingAutoRules()
	st = srv.store.Get()
	changes := srv.buildFirewallChangesPayload(st)
	if !changes.CanApply {
		writeAPIErrorWithExtra(w, http.StatusUnprocessableEntity, "FIREWALL_PENDING_INVALID",
			"pending firewall changes cannot be applied",
			map[string]any{"changes": changes})
		return
	}
	appliedBackup := store.CloneFilterRules(st.Firewall.FilterRules)
	pending := store.CloneFilterRules(st.Firewall.PendingFilterRules)
	_ = srv.store.Update(func(st *store.State) {
		st.Firewall.FilterRules = pending
		st.Firewall.PendingFilterDraft = false
		st.Firewall.PendingFilterRules = nil
	})
	if !srv.saveState(w) {
		srv.setFilterRules(appliedBackup)
		srv.setPendingFilterRules(pending)
		return
	}
	if err := srv.reloadNftWithFilterRevert(appliedBackup); err != nil {
		_ = srv.store.Update(func(st *store.State) {
			st.Firewall.FilterRules = appliedBackup
			st.Firewall.PendingFilterRules = pending
			st.Firewall.PendingFilterDraft = true
		})
		_ = srv.store.Save()
		writeApplyError(w, err)
		return
	}
	srv.auditLog(r, "firewall.apply", fmt.Sprintf("+%d ~%d -%d",
		len(changes.Diff.Added), len(changes.Diff.Modified), len(changes.Diff.Removed)))
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"applied": true,
		"changes": changes,
		"rules":   srv.store.Get().Firewall.FilterRules,
	})
}

func (srv *Server) handleFirewallDiscard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	st := srv.store.Get()
	if !st.Firewall.PendingFilterDraft {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "discarded": false})
		return
	}
	srv.discardPendingFilterDraft()
	if !srv.persistState(w) {
		return
	}
	srv.auditLog(r, "firewall.discard", "")
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":        true,
		"discarded": true,
		"rules":     srv.appliedFilterRules(srv.store.Get()),
		"changes":   srv.buildFirewallChangesPayload(srv.store.Get()),
	})
}
