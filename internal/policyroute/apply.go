package policyroute

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/hk59775634/qosnat2/internal/netif"
	"github.com/hk59775634/qosnat2/internal/store"
)

// Apply 根据 state 安装/更新 ip rule（先清理当前策略条目，再写入启用的规则）
func Apply(st store.State) error {
	aliases := store.AliasByName(st.Firewall.Aliases)
	for _, p := range st.Network.EgressPolicies {
		tbl := store.WanLinkRouteTable(p.WanLinkID, st.Network.WanLinks)
		if tbl > 0 {
			deleteExpandedPolicy(p, tbl, aliases)
		}
	}
	resolved := store.ResolveEgressPolicies(st, netif.PrimaryIPv4)
	for _, re := range resolved {
		rules, err := store.ExpandEgressIPRules(re.Policy, re.Table, aliases)
		if err != nil {
			return fmt.Errorf("egress %s: %w", re.Policy.ID, err)
		}
		for _, r := range rules {
			if err := addExpandedRule(r); err != nil {
				return fmt.Errorf("egress %s: %w", re.Policy.ID, err)
			}
		}
	}
	if err := checkUnresolvedEgress(st, resolved); err != nil {
		return err
	}
	flushRouteCache()
	return nil
}

// checkUnresolvedEgress 已启用但无法解析（缺 WAN / 无 SNAT IP）时返回错误
func checkUnresolvedEgress(st store.State, resolved []store.ResolvedEgress) error {
	ok := map[string]struct{}{}
	for _, re := range resolved {
		ok[re.Policy.ID] = struct{}{}
	}
	for _, p := range store.EnabledEgressPolicies(st.Network.EgressPolicies) {
		if _, done := ok[p.ID]; done {
			continue
		}
		w, found := store.FindWanLink(st.Network.WanLinks, p.WanLinkID)
		if !found || !w.Enabled {
			return fmt.Errorf("egress %s: wan link %q not found or disabled", p.ID, p.WanLinkID)
		}
		if strings.TrimSpace(w.Device) == "" {
			return fmt.Errorf("egress %s: wan link %q missing device", p.ID, p.WanLinkID)
		}
		if store.IsWarpWanLink(w) {
			return fmt.Errorf("egress %s: unresolved warp wan link %q (warp tunnel not ready)", p.ID, p.WanLinkID)
		}
		if p.NoSNAT {
			if strings.TrimSpace(w.Gateway) == "" {
				return fmt.Errorf("egress %s: no_snat requires gateway on wan link %q (next-hop NAT server)", p.ID, p.WanLinkID)
			}
			return fmt.Errorf("egress %s: unresolved no_snat wan link %q", p.ID, p.WanLinkID)
		}
		return fmt.Errorf("egress %s: no SNAT IP on %s (set snat_ip or add IPv4 to interface)", p.ID, w.Device)
	}
	return nil
}

// DeletePolicy 删除单条出站策略对应的 ip rule（在从 state 移除前调用）
func DeletePolicy(p store.EgressPolicy, links []store.WanLink, aliases map[string]store.AliasSet) {
	tbl := store.WanLinkRouteTable(p.WanLinkID, links)
	if tbl > 0 {
		deleteExpandedPolicy(p, tbl, aliases)
	}
	flushRouteCache()
}

func deleteExpandedPolicy(p store.EgressPolicy, table int, aliases map[string]store.AliasSet) {
	rules, err := store.ExpandEgressIPRules(p, table, aliases)
	if err != nil {
		return
	}
	for _, r := range rules {
		delExpandedRule(r)
	}
}

func addExpandedRule(r store.EgressIPRule) error {
	toPrio := r.Priority - 1
	if toPrio < 1 {
		toPrio = 1
	}
	if err := addPolicyRuleLookup(r.From, r.To, r.Iif, strconv.Itoa(r.Table), r.Priority); err != nil {
		return err
	}
	return addMainBypass(r, toPrio)
}

func addPolicyRuleLookup(from, to, iif, lookup string, priority int) error {
	args := []string{"rule", "add"}
	if iif != "" {
		args = append(args, "iif", iif)
	}
	if from != "" {
		args = append(args, "from", from)
	}
	if to != "" {
		args = append(args, "to", to)
	}
	args = append(args, "lookup", lookup, "priority", strconv.Itoa(priority))
	if out, err := exec.Command("ip", args...).CombinedOutput(); err != nil {
		msg := strings.TrimSpace(string(out))
		if !strings.Contains(msg, "File exists") {
			return fmt.Errorf("ip %s: %s %w", strings.Join(args, " "), msg, err)
		}
	}
	return nil
}

func addMainBypass(r store.EgressIPRule, toPrio int) error {
	switch r.Mode {
	case "source":
		if r.From == "" && r.Iif == "" {
			return nil
		}
		if r.From != "" {
			return addPolicyRuleLookup("", r.From, r.Iif, "main", toPrio)
		}
		return addPolicyRuleLookup("", "", r.Iif, "main", toPrio)
	case "destination":
		if r.To == "" {
			return nil
		}
		return addPolicyRuleLookup(r.To, "", r.Iif, "main", toPrio)
	case "both":
		if r.From != "" && r.To != "" {
			return addPolicyRuleLookup(r.From, r.To, r.Iif, "main", toPrio)
		}
	}
	return nil
}

func delExpandedRule(r store.EgressIPRule) {
	toPrio := r.Priority - 1
	if toPrio < 1 {
		toPrio = 1
	}
	delRuleLoop(r.From, r.To, r.Iif, strconv.Itoa(r.Table), r.Priority)
	switch r.Mode {
	case "source":
		if r.From != "" {
			delRuleLoop("", r.From, r.Iif, "main", toPrio)
		} else if r.Iif != "" {
			delRuleLoop("", "", r.Iif, "main", toPrio)
		}
	case "destination":
		if r.To != "" {
			delRuleLoop(r.To, "", r.Iif, "main", toPrio)
		}
	case "both":
		if r.From != "" && r.To != "" {
			delRuleLoop(r.From, r.To, r.Iif, "main", toPrio)
		}
	}
}

func delRuleLoop(from, to, iif, lookup string, priority int) {
	for {
		args := []string{"rule", "del"}
		if iif != "" {
			args = append(args, "iif", iif)
		}
		if from != "" {
			args = append(args, "from", from)
		}
		if to != "" {
			args = append(args, "to", to)
		}
		args = append(args, "lookup", lookup, "priority", strconv.Itoa(priority))
		if out, err := exec.Command("ip", args...).CombinedOutput(); err != nil {
			msg := strings.TrimSpace(string(out))
			if msg == "" || strings.Contains(strings.ToLower(msg), "not found") || strings.Contains(msg, "No such file") {
				break
			}
			break
		}
	}
}

func flushRouteCache() {
	_ = exec.Command("ip", "route", "flush", "cache").Run()
}

// AddDestinationRules 为指定目的 CIDR 添加策略路由（match=destination）。
func AddDestinationRules(cidr string, table, priority int) error {
	return addExpandedRule(store.EgressIPRule{To: cidr, Table: table, Priority: priority, Mode: "destination"})
}

// DeleteDestinationRules 删除 AddDestinationRules 写入的规则。
func DeleteDestinationRules(cidr string, table, priority int) {
	delExpandedRule(store.EgressIPRule{To: cidr, Table: table, Priority: priority, Mode: "destination"})
}

// FlushRouteCache 刷新路由缓存。
func FlushRouteCache() {
	flushRouteCache()
}

// ListRules 返回当前 ip rule（JSON 解析，供 API/调试）
func ListRules() ([]map[string]any, error) {
	out, err := exec.Command("ip", "-json", "rule", "list").Output()
	if err != nil {
		return nil, err
	}
	var raw []map[string]any
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

// ruleDirections 保留供测试兼容旧 match 语义。
func ruleDirections(match string) (mainDir, policyDir string) {
	if match == "" {
		match = "source"
	}
	mainDir = "to"
	policyDir = "from"
	if match == "destination" {
		mainDir = "from"
		policyDir = "to"
	}
	return mainDir, policyDir
}
