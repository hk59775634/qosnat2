package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/hk59775634/qosnat2/internal/store"
)

// handleFirewallPresetGoogleWebRTCBlock 一键拦截 Google WebRTC（STUN/TURN/媒体 UDP），
// 用于缓解 VPN+NAT 下 WebRTC 泄漏导致 Gemini 等服务异常。
//
// POST /api/v1/firewall/presets/google-webrtc-block
// Query: apply=0 仅暂存防火墙规则；默认 apply=1 在可应用时立即生效。
func (srv *Server) handleFirewallPresetGoogleWebRTCBlock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	applyNow := !strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("apply")), "0")

	presetAliases := store.GoogleWebRTCBlockAliases()
	var resolveWarns []string
	for i := range presetAliases {
		if err := store.NormalizeAlias(&presetAliases[i]); err != nil {
			writeBadRequest(w, err.Error())
			return
		}
		if presetAliases[i].Type == "fqdn" {
			warn, err := store.RefreshAliasDynamic(&presetAliases[i])
			if err != nil {
				writeBadRequest(w, fmt.Sprintf("resolve %s: %v", presetAliases[i].Name, err))
				return
			}
			if warn != "" {
				resolveWarns = append(resolveWarns, warn)
			}
			if len(presetAliases[i].Members) == 0 {
				writeBadRequest(w, fmt.Sprintf("alias %s resolved empty; check DNS then retry", presetAliases[i].Name))
				return
			}
		}
	}

	presetRules := store.GoogleWebRTCBlockRules()
	for i := range presetRules {
		if err := store.NormalizeFilterRule(&presetRules[i]); err != nil {
			writeBadRequest(w, err.Error())
			return
		}
	}

	st := srv.store.Get()
	newAliases, aliasAdded, aliasUpdated := store.UpsertAliasesByName(st.Firewall.Aliases, presetAliases)
	for _, rule := range presetRules {
		if err := store.ValidateFilterRuleAliases(rule, newAliases); err != nil {
			writeBadRequest(w, err.Error())
			return
		}
		if err := store.ValidateFilterRulePortAliases(rule, newAliases); err != nil {
			writeBadRequest(w, err.Error())
			return
		}
	}

	srv.ensurePendingFilterDraft()
	st = srv.store.Get()
	pending, ruleAdded, ruleUpdated := store.UpsertFilterRulesByID(st.Firewall.PendingFilterRules, presetRules)
	pending = srv.syncFilterRulesForState(st, pending)

	proposed := st
	proposed.Firewall.Aliases = newAliases
	proposed.Firewall.FilterRules = pending
	if err := srv.checkNftForState(proposed); err != nil {
		writeNftApplyError(w, err)
		return
	}

	backupAliases := cloneAliases(st.Firewall.Aliases)
	backupApplied := cloneFilterRules(st.Firewall.FilterRules)
	backupPending := cloneFilterRules(st.Firewall.PendingFilterRules)
	backupDraft := st.Firewall.PendingFilterDraft

	srv.setAliases(newAliases)
	srv.setPendingFilterRules(pending)
	if !srv.saveState(w) {
		srv.setAliases(backupAliases)
		_ = srv.store.Update(func(st *store.State) {
			st.Firewall.FilterRules = backupApplied
			st.Firewall.PendingFilterRules = backupPending
			st.Firewall.PendingFilterDraft = backupDraft
		})
		return
	}

	applied := false
	if applyNow {
		st = srv.store.Get()
		changes := srv.buildFirewallChangesPayload(st)
		if changes.CanApply {
			_ = srv.store.Update(func(st *store.State) {
				st.Firewall.FilterRules = store.CloneFilterRules(st.Firewall.PendingFilterRules)
				st.Firewall.PendingFilterDraft = false
				st.Firewall.PendingFilterRules = nil
			})
			if !srv.saveState(w) {
				srv.setAliases(backupAliases)
				_ = srv.store.Update(func(st *store.State) {
					st.Firewall.FilterRules = backupApplied
					st.Firewall.PendingFilterRules = backupPending
					st.Firewall.PendingFilterDraft = backupDraft
				})
				_ = srv.persistStateOrLog("google-webrtc-block revert")
				return
			}
			applied = true
		}
	}

	if err := srv.withNftApply(func() error {
		if err := srv.applyNftLocked(); err != nil {
			srv.setAliases(backupAliases)
			_ = srv.store.Update(func(st *store.State) {
				st.Firewall.FilterRules = backupApplied
				st.Firewall.PendingFilterRules = backupPending
				st.Firewall.PendingFilterDraft = backupDraft
			})
			_ = srv.persistStateOrLog("google-webrtc-block nft revert")
			_ = srv.applyEgressPolicyRoutes()
			return srv.revertReloadError("google-webrtc-block", err)
		}
		return nil
	}); err != nil {
		writeApplyError(w, err)
		return
	}

	if applied {
		srv.applyFirewallDerivedSideEffects()
	}

	st = srv.store.Get()
	changes := srv.buildFirewallChangesPayload(st)
	detail := fmt.Sprintf("aliases +%d ~%d; rules +%d ~%d; applied=%v",
		aliasAdded, aliasUpdated, ruleAdded, ruleUpdated, applied)
	srv.auditLog(r, "firewall.preset.google_webrtc_block", detail)

	resp := map[string]any{
		"ok":            true,
		"applied":       applied,
		"staged":        changes.HasPendingChanges,
		"alias_added":   aliasAdded,
		"alias_updated": aliasUpdated,
		"rule_added":    ruleAdded,
		"rule_updated":  ruleUpdated,
		"aliases": []string{
			store.AliasGoogleWebRTCStun,
			store.AliasGoogleWebRTCMedia,
			store.AliasGoogleWebRTCPorts,
		},
		"rule_ids": []string{
			store.RuleIDGoogleWebRTCStun,
			store.RuleIDGoogleWebRTCMedia,
		},
		"rules":   srv.workingFilterRules(st),
		"changes": changes,
	}
	if len(resolveWarns) > 0 {
		resp["resolve_warnings"] = resolveWarns
	}
	if applyNow && !applied && changes.HasPendingChanges {
		resp["warning_code"] = "PENDING_HAS_ISSUES"
		resp["message"] = "aliases updated; firewall rules staged but not applied (pending changes need review)"
	}
	writeJSON(w, http.StatusOK, resp)
}
