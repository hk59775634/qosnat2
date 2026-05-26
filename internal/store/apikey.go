package store

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// HashAPIKey 使用 bcrypt 存储 API Key（新密钥）。
func HashAPIKey(plain string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

// LegacyAPIKeyHash 旧版 SHA-256 hex（仅用于校验与自动升级）。
func LegacyAPIKeyHash(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

// IsLegacyAPIKeyHash 是否为 SHA-256 遗留哈希。
func IsLegacyAPIKeyHash(stored string) bool {
	if strings.HasPrefix(stored, "$2a$") || strings.HasPrefix(stored, "$2b$") || strings.HasPrefix(stored, "$2y$") {
		return false
	}
	return len(stored) == 64
}

// VerifyAPIKey 校验 API Key；兼容 bcrypt 与遗留 SHA-256。
func VerifyAPIKey(plain, stored string) bool {
	if stored == "" || plain == "" {
		return false
	}
	if strings.HasPrefix(stored, "$2") {
		return bcrypt.CompareHashAndPassword([]byte(stored), []byte(plain)) == nil
	}
	want := LegacyAPIKeyHash(plain)
	return subtle.ConstantTimeCompare([]byte(stored), []byte(want)) == 1
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
		if h, err := HashAPIKey(k.Key); err == nil {
			k.KeyHash = h
		} else {
			k.KeyHash = LegacyAPIKeyHash(k.Key)
		}
		if k.KeyPrefix == "" {
			k.KeyPrefix = APIKeyPrefix(k.Key)
		}
		k.Key = ""
	}
}
