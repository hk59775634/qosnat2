package api

import (
	"sync"
	"time"
)

const versionSwitchGrantTTL = 2 * time.Minute

// versionSwitchGrants 在弹窗内验证管理员密码后，短时允许同一会话发起版本切换（无需再次提交口令）。
type versionSwitchGrants struct {
	mu    sync.Mutex
	until map[string]time.Time
}

func newVersionSwitchGrants() *versionSwitchGrants {
	return &versionSwitchGrants{until: map[string]time.Time{}}
}

func (g *versionSwitchGrants) pruneLocked(now time.Time) {
	for tok, exp := range g.until {
		if !now.Before(exp) {
			delete(g.until, tok)
		}
	}
}

func (g *versionSwitchGrants) grant(sessionToken string) {
	if sessionToken == "" {
		return
	}
	now := time.Now()
	g.mu.Lock()
	defer g.mu.Unlock()
	g.pruneLocked(now)
	g.until[sessionToken] = now.Add(versionSwitchGrantTTL)
}

// consume 校验并一次性消费授权（防止重放）。
func (g *versionSwitchGrants) consume(sessionToken string) bool {
	if sessionToken == "" {
		return false
	}
	now := time.Now()
	g.mu.Lock()
	defer g.mu.Unlock()
	g.pruneLocked(now)
	exp, ok := g.until[sessionToken]
	if !ok || !now.Before(exp) {
		return false
	}
	delete(g.until, sessionToken)
	return true
}
