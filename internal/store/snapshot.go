package store

import "encoding/json"

// CloneState 深拷贝 state（用于 netplan 失败回滚）
func CloneState(st State) (State, error) {
	b, err := json.Marshal(st)
	if err != nil {
		return State{}, err
	}
	var out State
	if err := json.Unmarshal(b, &out); err != nil {
		return State{}, err
	}
	return out, nil
}

// ReplaceState 用快照覆盖当前 state（不自动 Save）
func (s *Store) ReplaceState(st State) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.State = st
	s.ensureDefaultsLocked()
}
