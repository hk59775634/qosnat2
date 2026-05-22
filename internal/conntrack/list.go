package conntrack

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// Entry 单条 conntrack 连接（由 conntrack -L 或 /proc 解析）
type Entry struct {
	L3Proto    string `json:"l3_proto,omitempty"`
	Protocol   string `json:"protocol"`
	TimeoutSec int    `json:"timeout_sec"`
	State      string `json:"state,omitempty"`
	Src        string `json:"src"`
	Dst        string `json:"dst"`
	Sport      int    `json:"sport,omitempty"`
	Dport      int    `json:"dport,omitempty"`
	ReplySrc   string `json:"reply_src,omitempty"`
	ReplyDst   string `json:"reply_dst,omitempty"`
	ReplySport int    `json:"reply_sport,omitempty"`
	ReplyDport int    `json:"reply_dport,omitempty"`
	Mark       uint32 `json:"mark,omitempty"`
	Flags      string `json:"flags,omitempty"`
	Raw        string `json:"raw,omitempty"`
}

// ListResult 列表响应
type ListResult struct {
	Count     int     `json:"count"`
	Limit     int     `json:"limit"`
	Truncated bool    `json:"truncated"`
	Entries   []Entry `json:"entries"`
}

// Count 当前连接表条目数（/proc 或 sysctl）
func Count() int {
	b, err := os.ReadFile("/proc/sys/net/netfilter/nf_conntrack_count")
	if err != nil {
		out, err2 := exec.Command("sysctl", "-n", "net.netfilter.nf_conntrack_count").Output()
		if err2 != nil {
			return 0
		}
		b = out
	}
	n, _ := strconv.Atoi(strings.TrimSpace(string(b)))
	return n
}

// List 解析连接表，优先流式读 /proc 并在达到 limit 后停止
func List(limit int, filter string) (ListResult, error) {
	if limit <= 0 {
		limit = 200
	}
	if limit > 2000 {
		limit = 2000
	}
	filter = strings.ToLower(strings.TrimSpace(filter))
	total := Count()

	if entries, err := listFromProc(limit, filter); err == nil {
		return ListResult{
			Count:     total,
			Limit:     limit,
			Truncated: total > len(entries) || len(entries) >= limit,
			Entries:   entries,
		}, nil
	}
	return listFromConntrackCLI(limit, filter, total)
}

func listFromProc(limit int, filter string) ([]Entry, error) {
	f, err := os.Open("/proc/net/nf_conntrack")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var entries []Entry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || !strings.Contains(line, "src=") {
			continue
		}
		e, ok := parseLine(line)
		if !ok {
			continue
		}
		if filter != "" {
			hay := strings.ToLower(e.Src + " " + e.Dst + " " + e.ReplySrc + " " + e.ReplyDst)
			if !strings.Contains(hay, filter) {
				continue
			}
		}
		entries = append(entries, e)
		if len(entries) >= limit {
			break
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

func listFromConntrackCLI(limit int, filter string, total int) (ListResult, error) {
	if _, err := exec.LookPath("conntrack"); err != nil {
		return ListResult{}, fmt.Errorf("conntrack-tools not installed")
	}
	out, err := exec.Command("conntrack", "-L").CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return ListResult{}, err
		}
		if !strings.Contains(msg, "src=") {
			return ListResult{}, fmt.Errorf("conntrack -L: %s", msg)
		}
		out = []byte(msg)
	}
	var entries []Entry
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, "src=") {
			continue
		}
		e, ok := parseLine(line)
		if !ok {
			continue
		}
		if filter != "" {
			hay := strings.ToLower(e.Src + " " + e.Dst + " " + e.ReplySrc + " " + e.ReplyDst)
			if !strings.Contains(hay, filter) {
				continue
			}
		}
		entries = append(entries, e)
		if len(entries) >= limit {
			break
		}
	}
	return ListResult{
		Count:     total,
		Limit:     limit,
		Truncated: total > len(entries) || len(entries) >= limit,
		Entries:   entries,
	}, nil
}

func parseLine(line string) (Entry, bool) {
	fields := strings.Fields(line)
	if len(fields) < 4 {
		return Entry{}, false
	}
	i := 0
	e := Entry{Raw: line}
	if fields[0] == "ipv4" || fields[0] == "ipv6" {
		e.L3Proto = fields[0]
		i = 3
		if i >= len(fields) {
			return Entry{}, false
		}
	}
	e.Protocol = fields[i]
	i++
	if i < len(fields) && isDigits(fields[i]) {
		i++
	}
	if i < len(fields) {
		if n, err := strconv.Atoi(fields[i]); err == nil {
			e.TimeoutSec = n
			i++
		}
	}
	if i < len(fields) && !strings.Contains(fields[i], "=") {
		e.State = fields[i]
		i++
	}
	var srcCount int
	for ; i < len(fields); i++ {
		f := fields[i]
		if strings.HasPrefix(f, "[") {
			e.Flags = strings.Trim(f, "[]")
			continue
		}
		if !strings.Contains(f, "=") {
			continue
		}
		parts := strings.SplitN(f, "=", 2)
		k, v := parts[0], parts[1]
		switch k {
		case "src":
			if srcCount == 0 {
				e.Src = v
			} else {
				e.ReplySrc = v
			}
			srcCount++
		case "dst":
			if e.ReplySrc == "" && e.Dst == "" && srcCount == 1 {
				e.Dst = v
			} else if e.ReplyDst == "" {
				e.ReplyDst = v
			} else {
				e.Dst = v
			}
		case "sport":
			if e.Sport == 0 {
				e.Sport, _ = strconv.Atoi(v)
			} else {
				e.ReplySport, _ = strconv.Atoi(v)
			}
		case "dport":
			if e.Dport == 0 {
				e.Dport, _ = strconv.Atoi(v)
			} else {
				e.ReplyDport, _ = strconv.Atoi(v)
			}
		case "mark":
			u, _ := strconv.ParseUint(v, 10, 32)
			e.Mark = uint32(u)
		}
	}
	if e.Src == "" {
		return Entry{}, false
	}
	return e, true
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
