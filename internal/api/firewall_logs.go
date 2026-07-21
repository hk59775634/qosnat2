package api

import (
	"bufio"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	fwLogRIDRe   = regexp.MustCompile(`qosnat2:rid:([A-Za-z0-9_-]+)`)
	fwCounterRe  = regexp.MustCompile(`counter packets (\d+) bytes (\d+)`)
	fwCommentRe  = regexp.MustCompile(`comment "([^"]*)"`)
)

func (srv *Server) handleFirewallLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	entries := readFirewallKernelLogs(limit)
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries, "limit": limit})
}

type firewallLogEntry struct {
	Time    string `json:"time,omitempty"`
	Line    string `json:"line"`
	RuleID  string `json:"rule_id,omitempty"`
	Message string `json:"message,omitempty"`
}

func readFirewallKernelLogs(limit int) []firewallLogEntry {
	// journalctl 优先；失败则 dmesg
	cmd := exec.Command("journalctl", "-k", "-n", strconv.Itoa(limit*4), "-o", "short-iso", "--no-pager")
	out, err := cmd.CombinedOutput()
	text := string(out)
	if err != nil || !strings.Contains(text, "qosnat2-fw") {
		cmd2 := exec.Command("dmesg", "-T")
		out2, err2 := cmd2.CombinedOutput()
		if err2 == nil {
			text = string(out2)
		}
	}
	var entries []firewallLogEntry
	sc := bufio.NewScanner(strings.NewReader(text))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if !strings.Contains(line, "qosnat2-fw") {
			continue
		}
		e := firewallLogEntry{Line: line, Message: line}
		if m := fwLogRIDRe.FindStringSubmatch(line); len(m) > 1 {
			e.RuleID = m[1]
		}
		// short-iso: 2024-01-02T03:04:05+00:00 host kernel: ...
		if i := strings.IndexByte(line, ' '); i > 0 {
			e.Time = line[:i]
		}
		entries = append(entries, e)
	}
	if len(entries) > limit {
		entries = entries[len(entries)-limit:]
	}
	// 最新在前
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}
	return entries
}

func (srv *Server) handleFirewallCounters(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	counters := readFirewallCounters()
	writeJSON(w, http.StatusOK, map[string]any{
		"counters":   counters,
		"checked_at": time.Now().UTC().Format(time.RFC3339),
	})
}

type firewallCounterEntry struct {
	RuleID  string `json:"rule_id,omitempty"`
	Chain   string `json:"chain,omitempty"`
	Packets uint64 `json:"packets"`
	Bytes   uint64 `json:"bytes"`
	Line    string `json:"line,omitempty"`
}

func readFirewallCounters() []firewallCounterEntry {
	cmd := exec.Command("nft", "-a", "list", "table", "inet", "qosnat")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil
	}
	var (
		entries []firewallCounterEntry
		chain   string
	)
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "chain ") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				chain = fields[1]
			}
			continue
		}
		if !strings.Contains(line, "counter packets") {
			continue
		}
		m := fwCounterRe.FindStringSubmatch(line)
		if len(m) < 3 {
			continue
		}
		pkts, _ := strconv.ParseUint(m[1], 10, 64)
		bytes, _ := strconv.ParseUint(m[2], 10, 64)
		e := firewallCounterEntry{Chain: chain, Packets: pkts, Bytes: bytes, Line: line}
		if cm := fwCommentRe.FindStringSubmatch(line); len(cm) > 1 {
			if rm := fwLogRIDRe.FindStringSubmatch(cm[1]); len(rm) > 1 {
				e.RuleID = rm[1]
			}
		}
		entries = append(entries, e)
	}
	return entries
}
