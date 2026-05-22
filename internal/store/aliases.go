package store

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
)

// AliasSet nft 对象组（ipv4 地址/网段集合，asn 为 ASN 前缀集合）
type AliasSet struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"` // ipv4_addr | asn
	ASN     int      `json:"asn,omitempty"`
	Members []string `json:"members"`
	Comment string   `json:"comment,omitempty"`
}

// NormalizeAlias 校验别名
func NormalizeAlias(a *AliasSet) error {
	if a == nil {
		return fmt.Errorf("alias nil")
	}
	name := strings.TrimSpace(a.Name)
	if name == "" {
		b := make([]byte, 4)
		_, _ = rand.Read(b)
		name = "alias_" + hex.EncodeToString(b)
	}
	if !isValidAliasName(name) {
		return fmt.Errorf("invalid alias name %q", name)
	}
	a.Name = name
	typ := strings.ToLower(strings.TrimSpace(a.Type))
	if typ == "" {
		typ = "ipv4_addr"
	}
	switch typ {
	case "ipv4_addr", "asn":
	default:
		return fmt.Errorf("type must be ipv4_addr or asn")
	}
	a.Type = typ
	if typ == "asn" {
		if a.ASN <= 0 || a.ASN > 4294967295 {
			return fmt.Errorf("asn number required for type asn")
		}
	}
	var members []string
	for _, m := range a.Members {
		m = strings.TrimSpace(m)
		if m != "" {
			members = append(members, m)
		}
	}
	if len(members) == 0 {
		return fmt.Errorf("members required")
	}
	a.Members = members
	a.Comment = strings.TrimSpace(a.Comment)
	return nil
}

func isValidAliasName(s string) bool {
	if len(s) == 0 || len(s) > 32 {
		return false
	}
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
			continue
		}
		return false
	}
	return true
}

// NftSetName 生成 nft set 标识符
func (a AliasSet) NftSetName() string {
	return "alias_" + a.Name
}
