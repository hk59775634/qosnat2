package nft

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// IncrementalEnabled 为 true 时防火墙单条增删可尝试 nft CLI 增量，失败则回退全表 reload。
func IncrementalEnabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("QOSNAT_NFT_INCREMENTAL")))
	return v == "1" || v == "true" || v == "yes"
}

// WriteRulesFile 将完整 ruleset 写入磁盘（不加载内核）。
func WriteRulesFile(body string) error {
	if err := os.MkdirAll(filepath.Dir(RulesPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(RulesPath, []byte(body), 0644)
}

// ReplaceFilterRuleByID 在同一 nft 脚本内 delete+add，缩短 PATCH 窗口。
func ReplaceFilterRuleByID(chain, id, newLine string) error {
	chain = strings.ToLower(strings.TrimSpace(chain))
	id = strings.TrimSpace(id)
	newLine = strings.TrimSpace(newLine)
	if chain == "" || id == "" || newLine == "" {
		return fmt.Errorf("empty chain, id or rule line")
	}
	marker := "qosnat2:rid:" + id
	out, err := exec.Command("nft", "-a", "list", "chain", "inet", TableName, chain).CombinedOutput()
	if err != nil {
		return fmt.Errorf("nft list chain: %s %w", strings.TrimSpace(string(out)), err)
	}
	handle := findRuleHandle(string(out), marker)
	if handle == "" {
		return fmt.Errorf("rule handle not found for %s", marker)
	}
	script := fmt.Sprintf("delete rule inet %s %s handle %s\nadd rule inet %s %s %s\n",
		TableName, chain, handle, TableName, chain, newLine)
	return runNftScript(script)
}

// AddFilterRuleLine 向链末尾追加一条 filter 规则（须已通过 CheckRuleset 全表校验）。
func AddFilterRuleLine(chain, line string) error {
	chain = strings.ToLower(strings.TrimSpace(chain))
	line = strings.TrimSpace(line)
	if chain == "" || line == "" {
		return fmt.Errorf("empty chain or rule line")
	}
	script := fmt.Sprintf("add rule inet %s %s %s\n", TableName, chain, line)
	return runNftScript(script)
}

// DeleteFilterRuleByID 按 qosnat2:rid:<id> 注释标记删除规则；未找到 handle 时返回 error。
func DeleteFilterRuleByID(chain, id string) error {
	chain = strings.ToLower(strings.TrimSpace(chain))
	id = strings.TrimSpace(id)
	if chain == "" || id == "" {
		return fmt.Errorf("empty chain or id")
	}
	marker := "qosnat2:rid:" + id
	out, err := exec.Command("nft", "-a", "list", "chain", "inet", TableName, chain).CombinedOutput()
	if err != nil {
		return fmt.Errorf("nft list chain: %s %w", strings.TrimSpace(string(out)), err)
	}
	handle := findRuleHandle(string(out), marker)
	if handle == "" {
		return fmt.Errorf("rule handle not found for %s", marker)
	}
	script := fmt.Sprintf("delete rule inet %s %s handle %s\n", TableName, chain, handle)
	return runNftScript(script)
}

func runNftScript(script string) error {
	f, err := os.CreateTemp("", "qosnat-nft-incr-*.nft")
	if err != nil {
		return err
	}
	path := f.Name()
	defer os.Remove(path)
	if _, err := f.WriteString(script); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	out, err := exec.Command("nft", "-f", path).CombinedOutput()
	if err != nil {
		return fmt.Errorf("nft incremental: %s %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func findRuleHandle(listing, marker string) string {
	for _, line := range strings.Split(listing, "\n") {
		if !strings.Contains(line, marker) {
			continue
		}
		if i := strings.LastIndex(line, "# handle "); i >= 0 {
			return strings.TrimSpace(line[i+len("# handle "):])
		}
	}
	return ""
}
