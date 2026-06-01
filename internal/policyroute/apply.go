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
	for _, p := range st.Network.EgressPolicies {
		tbl := store.WanLinkRouteTable(p.WanLinkID, st.Network.WanLinks)
		if tbl > 0 {
			delRules(p.CIDR, p.Match, tbl, p.Priority)
		}
	}
	resolved := store.ResolveEgressPolicies(st, netif.PrimaryIPv4)
	for _, re := range resolved {
		if err := addRules(re.Policy.CIDR, re.Policy.Match, re.Table, re.Priority); err != nil {
			return fmt.Errorf("egress %s: %w", re.Policy.ID, err)
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
		return fmt.Errorf("egress %s: no SNAT IP on %s (set snat_ip or add IPv4 to interface)", p.ID, w.Device)
	}
	return nil
}

// DeletePolicy 删除单条出站策略对应的 ip rule（在从 state 移除前调用）
func DeletePolicy(p store.EgressPolicy, links []store.WanLink) {
	tbl := store.WanLinkRouteTable(p.WanLinkID, links)
	if tbl > 0 {
		delRules(p.CIDR, p.Match, tbl, p.Priority)
	}
	flushRouteCache()
}

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

func addRules(cidr, match string, table, priority int) error {
	toPrio := priority - 1
	if toPrio < 1 {
		toPrio = 1
	}
	mainDir, policyDir := ruleDirections(match)
	if out, err := exec.Command("ip", "rule", "add", mainDir, cidr, "lookup", "main", "priority", strconv.Itoa(toPrio)).CombinedOutput(); err != nil {
		msg := strings.TrimSpace(string(out))
		if !strings.Contains(msg, "File exists") {
			return fmt.Errorf("ip rule to %s: %s %w", cidr, msg, err)
		}
	}
	if out, err := exec.Command("ip", "rule", "add", policyDir, cidr, "lookup", strconv.Itoa(table), "priority", strconv.Itoa(priority)).CombinedOutput(); err != nil {
		msg := strings.TrimSpace(string(out))
		if !strings.Contains(msg, "File exists") {
			_ = exec.Command("ip", "rule", "del", mainDir, cidr, "lookup", "main", "priority", strconv.Itoa(toPrio)).Run()
			return fmt.Errorf("ip rule from %s: %s %w", cidr, msg, err)
		}
	}
	return nil
}

func delRules(cidr, match string, table, priority int) {
	toPrio := priority - 1
	if toPrio < 1 {
		toPrio = 1
	}
	mainDir, policyDir := ruleDirections(match)
	delRuleLoop(policyDir, cidr, strconv.Itoa(table), priority)
	delRuleLoop(mainDir, cidr, "main", toPrio)
}

func delRuleLoop(dir, cidr, lookup string, priority int) {
	for {
		if out, err := exec.Command("ip", "rule", "del", dir, cidr, "lookup", lookup, "priority", strconv.Itoa(priority)).CombinedOutput(); err != nil {
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
	return addRules(cidr, "destination", table, priority)
}

// DeleteDestinationRules 删除 AddDestinationRules 写入的规则。
func DeleteDestinationRules(cidr string, table, priority int) {
	delRules(cidr, "destination", table, priority)
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
