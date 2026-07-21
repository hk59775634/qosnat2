package store

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Schedule 防火墙时间表（按时段决定规则是否生效）。
type Schedule struct {
	ID      string          `json:"id"`
	Name    string          `json:"name"`
	Ranges  []ScheduleRange `json:"ranges"`
	Enabled bool            `json:"enabled"`
	Comment string          `json:"comment,omitempty"`
}

// ScheduleRange 一周内的时间窗口（本地时区）。
// Weekdays: 逗号分隔 0-6（0=周日）或 mon,tue,...；空表示每天。
// Start/End: HH:MM（24h）；若 End <= Start 则跨午夜。
type ScheduleRange struct {
	Weekdays string `json:"weekdays,omitempty"`
	Start    string `json:"start"`
	End      string `json:"end"`
}

// NormalizeSchedule 校验时间表。
func NormalizeSchedule(s *Schedule) error {
	if s == nil {
		return fmt.Errorf("schedule nil")
	}
	if strings.TrimSpace(s.ID) == "" {
		b := make([]byte, 6)
		_, _ = rand.Read(b)
		s.ID = "sch-" + hex.EncodeToString(b)
	}
	s.Name = strings.TrimSpace(s.Name)
	if s.Name == "" {
		return fmt.Errorf("name required")
	}
	s.Comment = strings.TrimSpace(s.Comment)
	if len(s.Ranges) == 0 {
		return fmt.Errorf("ranges required")
	}
	out := make([]ScheduleRange, 0, len(s.Ranges))
	for i, r := range s.Ranges {
		nr, err := normalizeScheduleRange(r)
		if err != nil {
			return fmt.Errorf("ranges[%d]: %w", i, err)
		}
		out = append(out, nr)
	}
	s.Ranges = out
	return nil
}

func normalizeScheduleRange(r ScheduleRange) (ScheduleRange, error) {
	startM, err := parseHHMM(r.Start)
	if err != nil {
		return r, fmt.Errorf("start: %w", err)
	}
	endM, err := parseHHMM(r.End)
	if err != nil {
		return r, fmt.Errorf("end: %w", err)
	}
	wd, err := normalizeWeekdays(r.Weekdays)
	if err != nil {
		return r, err
	}
	r.Start = formatHHMM(startM)
	r.End = formatHHMM(endM)
	r.Weekdays = wd
	return r, nil
}

func parseHHMM(s string) (int, error) {
	s = strings.TrimSpace(s)
	hStr, mStr, ok := strings.Cut(s, ":")
	if !ok {
		return 0, fmt.Errorf("must be HH:MM")
	}
	h, err := strconv.Atoi(strings.TrimSpace(hStr))
	if err != nil || h < 0 || h > 23 {
		return 0, fmt.Errorf("invalid hour")
	}
	m, err := strconv.Atoi(strings.TrimSpace(mStr))
	if err != nil || m < 0 || m > 59 {
		return 0, fmt.Errorf("invalid minute")
	}
	return h*60 + m, nil
}

func formatHHMM(mins int) string {
	return fmt.Sprintf("%02d:%02d", mins/60, mins%60)
}

func normalizeWeekdays(raw string) (string, error) {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" || raw == "*" || raw == "all" {
		return "", nil
	}
	alias := map[string]int{
		"sun": 0, "sunday": 0,
		"mon": 1, "monday": 1,
		"tue": 2, "tues": 2, "tuesday": 2,
		"wed": 3, "wednesday": 3,
		"thu": 4, "thur": 4, "thurs": 4, "thursday": 4,
		"fri": 5, "friday": 5,
		"sat": 6, "saturday": 6,
	}
	seen := map[int]struct{}{}
	var days []int
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if n, ok := alias[part]; ok {
			if _, exists := seen[n]; exists {
				continue
			}
			seen[n] = struct{}{}
			days = append(days, n)
			continue
		}
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 || n > 6 {
			return "", fmt.Errorf("invalid weekday %q", part)
		}
		if _, exists := seen[n]; exists {
			continue
		}
		seen[n] = struct{}{}
		days = append(days, n)
	}
	if len(days) == 0 {
		return "", nil
	}
	parts := make([]string, len(days))
	for i, d := range days {
		parts[i] = strconv.Itoa(d)
	}
	return strings.Join(parts, ","), nil
}

// ScheduleActiveAt 判断时间表在 t（本地）是否处于任一窗口内。
// 禁用的时间表视为「始终不激活」（规则不生效）。
func ScheduleActiveAt(s Schedule, t time.Time) bool {
	if !s.Enabled {
		return false
	}
	if len(s.Ranges) == 0 {
		return false
	}
	for _, r := range s.Ranges {
		if scheduleRangeActive(r, t) {
			return true
		}
	}
	return false
}

func scheduleRangeActive(r ScheduleRange, t time.Time) bool {
	startM, err1 := parseHHMM(r.Start)
	endM, err2 := parseHHMM(r.End)
	if err1 != nil || err2 != nil {
		return false
	}
	wdOK := true
	if wd := strings.TrimSpace(r.Weekdays); wd != "" {
		wdOK = false
		want := int(t.Weekday())
		for _, p := range strings.Split(wd, ",") {
			n, err := strconv.Atoi(strings.TrimSpace(p))
			if err == nil && n == want {
				wdOK = true
				break
			}
		}
	}
	if !wdOK {
		return false
	}
	nowM := t.Hour()*60 + t.Minute()
	if endM <= startM {
		// 跨午夜：例如 22:00-06:00
		return nowM >= startM || nowM < endM
	}
	return nowM >= startM && nowM < endM
}

// FindSchedule 按 ID 查找。
func FindSchedule(list []Schedule, id string) (Schedule, bool) {
	id = strings.TrimSpace(id)
	for _, s := range list {
		if s.ID == id {
			return s, true
		}
	}
	return Schedule{}, false
}

// RuleEffectivelyEnabled 结合时间表判断规则此刻是否应写入 nft。
func RuleEffectivelyEnabled(r FilterRule, schedules []Schedule, now time.Time) bool {
	if !r.Enabled {
		return false
	}
	sid := strings.TrimSpace(r.ScheduleID)
	if sid == "" {
		return true
	}
	s, ok := FindSchedule(schedules, sid)
	if !ok {
		// 引用丢失：保守起见不生效，避免误放行
		return false
	}
	return ScheduleActiveAt(s, now)
}

// ScheduleReferencedByRules 是否有规则引用该时间表。
func ScheduleReferencedByRules(rules []FilterRule, id string) bool {
	id = strings.TrimSpace(id)
	if id == "" {
		return false
	}
	for _, r := range rules {
		if strings.TrimSpace(r.ScheduleID) == id {
			return true
		}
	}
	return false
}
