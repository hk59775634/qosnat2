package store

import (
	"crypto/sha256"
	"encoding/hex"
)

// HashAPIKey SHA-256 hex（持久化仅存哈希）
func HashAPIKey(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

// APIKeyPrefix 列表展示用前缀
func APIKeyPrefix(plain string) string {
	if len(plain) <= 8 {
		return plain
	}
	return plain[:8] + "…"
}

// migrateAPIKeysLocked 明文 key 迁入 key_hash（调用方已持 store 锁）
func migrateAPIKeysLocked(keys *[]APIKey) {
	for i := range *keys {
		k := &(*keys)[i]
		if k.KeyHash != "" {
			k.Key = ""
			continue
		}
		if k.Key == "" {
			continue
		}
		k.KeyHash = HashAPIKey(k.Key)
		if k.KeyPrefix == "" {
			k.KeyPrefix = APIKeyPrefix(k.Key)
		}
		k.Key = ""
	}
}
