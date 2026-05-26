package certs

import (
	"strings"

	"github.com/hk59775634/qosnat2/internal/store"
)

// FindBestCertForDomain 在证书库中查找同域名、剩余有效期最长的 ACME 证书（可排除当前 ID）
func FindBestCertForDomain(all []store.ManagedCertificate, domain, excludeID string) (store.ManagedCertificate, bool) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return store.ManagedCertificate{}, false
	}
	var best store.ManagedCertificate
	bestDays := -1
	for _, c := range all {
		if c.ID == excludeID || c.Type != store.CertTypeACME || c.CertPath == "" {
			continue
		}
		match := false
		for _, d := range c.Domains {
			if strings.EqualFold(strings.TrimSpace(d), domain) {
				match = true
				break
			}
		}
		if !match {
			continue
		}
		days := DaysUntilExpiry(c.CertPath)
		if best.ID == "" || days > bestDays {
			bestDays = days
			best = c
		}
	}
	return best, best.ID != ""
}

// IsFresherThan 候选证书是否明显新于当前证书（剩余天数更多）
func IsFresherThan(candidate, current store.ManagedCertificate) bool {
	if candidate.ID == "" || candidate.ID == current.ID {
		return false
	}
	cd := DaysUntilExpiry(candidate.CertPath)
	cur := DaysUntilExpiry(current.CertPath)
	return cd > cur+1
}
