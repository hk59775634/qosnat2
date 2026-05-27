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
			delRules(p.CIDR, tbl, p.Priority)
		}
	}
	resolved := store.ResolveEgressPolicies(st, netif.PrimaryIPv4)
	for _, re := range resolved {
		if err := addRules(re.Policy.CIDR, re.Table, re.Priority); err != nil {
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
		if strings.TrimSpace(w.Gateway) == "" || strings.TrimSpace(w.Device) == "" {
			return fmt.Errorf("egress %s: wan link %q missing gateway or device", p.ID, p.WanLinkID)
		}
		return fmt.Errorf("egress %s: no SNAT IP on %s (set snat_ip or add IPv4 to interface)", p.ID, w.Device)
	}
	return nil
}

// DeletePolicy 删除单条出站策略对应的 ip rule（在从 state 移除前调用）
func DeletePolicy(p store.EgressPolicy, links []store.WanLink) {
	tbl := store.WanLinkRouteTable(p.WanLinkID, links)
	if tbl > 0 {
		delRules(p.CIDR, tbl, p.Priority)
	}
	flushRouteCache()
}

func addRules(cidr string, table, priority int) error {
	toPrio := priority - 1
	if toPrio < 1 {
		toPrio = 1
	}
	if out, err := exec.Command("ip", "rule", "add", "to", cidr, "lookup", "main", "priority", strconv.Itoa(toPrio)).CombinedOutput(); err != nil {
		msg := strings.TrimSpace(string(out))
		if !strings.Contains(msg, "File exists") {
			return fmt.Errorf("ip rule to %s: %s %w", cidr, msg, err)
		}
	}
	if out, err := exec.Command("ip", "rule", "add", "from", cidr, "lookup", strconv.Itoa(table), "priority", strconv.Itoa(priority)).CombinedOutput(); err != nil {
		msg := strings.TrimSpace(string(out))
		if !strings.Contains(msg, "File exists") {
			_ = exec.Command("ip", "rule", "del", "to", cidr, "lookup", "main", "priority", strconv.Itoa(toPrio)).Run()
			return fmt.Errorf("ip rule from %s: %s %w", cidr, msg, err)
		}
	}
	return nil
}

func delRules(cidr string, table, priority int) {
	toPrio := priority - 1
	if toPrio < 1 {
		toPrio = 1
	}
	for {
		if out, err := exec.Command("ip", "rule", "del", "from", cidr, "lookup", strconv.Itoa(table), "priority", strconv.Itoa(priority)).CombinedOutput(); err != nil {
			msg := strings.TrimSpace(string(out))
			if msg == "" || strings.Contains(strings.ToLower(msg), "not found") || strings.Contains(msg, "No such file") {
				break
			}
			break
		}
	}
	for {
		if out, err := exec.Command("ip", "rule", "del", "to", cidr, "lookup", "main", "priority", strconv.Itoa(toPrio)).CombinedOutput(); err != nil {
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
