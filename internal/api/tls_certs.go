package api

import (
	"log"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/hk59775634/qosnat2/internal/certs"
	"github.com/hk59775634/qosnat2/internal/store"
)

// upsertSystemTLSManagedCert 将 qosnat2 HTTPS 证书写入证书库并关联 System.TLSManagedCertID
func (srv *Server) upsertSystemTLSManagedCert(certPEM, keyPEM string, acme bool, domain, email string, staging bool) (string, error) {
	st := srv.store.Get()
	id := strings.TrimSpace(st.System.TLSManagedCertID)
	var mc store.ManagedCertificate
	if id != "" {
		if c, ok := store.FindManagedCert(st.Certificates, id); ok {
			mc = c
		}
	}
	if mc.ID == "" {
		mc = store.ManagedCertificate{
			ID:   store.NewManagedCertID(),
			Name: "qosnat2 HTTPS",
			Type: store.CertTypeManual,
		}
	}
	if d := strings.TrimSpace(domain); d != "" {
		mc.Name = d
		mc.Domains = []string{d}
	}
	if acme {
		mc.Type = store.CertTypeACME
		mc.AcmeEmail = strings.TrimSpace(email)
		mc.AcmeStaging = staging
		if mc.AcmeRenewDays <= 0 {
			mc.AcmeRenewDays = store.DefaultAcmeAutoRenewDays
		}
		mc.AutoRenewEnabled = true
		mc.AutoRenewPaused = false
		mc.AutoRenewPauseReason = ""
		mc.AcmeLastOK = time.Now().UTC().Format(time.RFC3339)
		mc.AcmeLastError = ""
	} else {
		mc.Type = store.CertTypeManual
	}
	certPath, keyPath, caPath, err := certs.SavePEM(mc.ID, certPEM, keyPEM, "")
	if err != nil {
		return "", err
	}
	mc.CertPath, mc.KeyPath, mc.CaPath = certPath, keyPath, caPath
	mc.Subject, mc.NotAfter = certs.ParseMeta(certPath)
	if err := store.NormalizeManagedCertificate(&mc); err != nil {
		return "", err
	}

	_ = srv.store.Update(func(s *store.State) {
		found := false
		for i := range s.Certificates {
			if s.Certificates[i].ID == mc.ID {
				s.Certificates[i] = mc
				found = true
				break
			}
		}
		if !found {
			s.Certificates = append(s.Certificates, mc)
		}
		s.System.TLSManagedCertID = mc.ID
		if acme {
			if domain != "" {
				s.System.TLSDomain = strings.TrimSpace(domain)
			}
			if email != "" {
				s.System.TLSAcmeEmail = strings.TrimSpace(email)
			}
			s.System.TLSAcmeEnabled = true
			s.System.TLSAcmeStaging = staging
		}
	})
	if err := srv.store.Save(); err != nil {
		log.Printf("save state: %v", err)
	}
	return mc.ID, nil
}

func (srv *Server) applyTLSFromManagedCertID(certID string) (needsRestart bool, err error) {
	st := srv.store.Get()
	mc, ok := store.FindManagedCert(st.Certificates, certID)
	if !ok {
		return false, fmt.Errorf("certificate not found: %s", certID)
	}
	certPEM, err := os.ReadFile(mc.CertPath)
	if err != nil {
		return false, fmt.Errorf("read certificate: %w", err)
	}
	keyPEM, err := os.ReadFile(mc.KeyPath)
	if err != nil {
		return false, fmt.Errorf("read private key: %w", err)
	}
	needsRestart, err = srv.applyTLS(true, string(certPEM), string(keyPEM))
	if err != nil {
		return false, err
	}
	_ = srv.store.Update(func(s *store.State) {
		s.System.TLSManagedCertID = certID
		s.System.TLSEnabled = true
		if mc.Type == store.CertTypeACME {
			s.System.TLSAcmeEnabled = true
			if len(mc.Domains) > 0 {
				s.System.TLSDomain = mc.Domains[0]
			}
			s.System.TLSAcmeEmail = mc.AcmeEmail
			s.System.TLSAcmeStaging = mc.AcmeStaging
			if mc.AcmeRenewDays > 0 {
				s.System.TLSAcmeRenewDays = mc.AcmeRenewDays
			}
		}
	})
	if err := srv.store.Save(); err != nil {
		log.Printf("save state: %v", err)
	}
	return needsRestart, nil
}

// ensureSystemTLSLinkedToLibrary 将已有 /etc/qosnat2/tls.* 关联进证书库（升级兼容）
func (srv *Server) ensureSystemTLSLinkedToLibrary() {
	st := srv.store.Get()
	if strings.TrimSpace(st.System.TLSManagedCertID) != "" {
		return
	}
	if !tlsFileExists(defaultTLSCertPath) || !tlsFileExists(defaultTLSKeyPath) {
		return
	}
	certPEM, err := os.ReadFile(defaultTLSCertPath)
	if err != nil {
		return
	}
	keyPEM, err := os.ReadFile(defaultTLSKeyPath)
	if err != nil {
		return
	}
	_, _ = srv.upsertSystemTLSManagedCert(
		string(certPEM), string(keyPEM),
		st.System.TLSAcmeEnabled,
		st.System.TLSDomain,
		st.System.TLSAcmeEmail,
		st.System.TLSAcmeStaging,
	)
}

func (srv *Server) renewSystemManagedCertACME() error {
	st := srv.store.Get()
	id := strings.TrimSpace(st.System.TLSManagedCertID)
	if id == "" {
		return fmt.Errorf("no managed certificate linked")
	}
	mc, ok := store.FindManagedCert(st.Certificates, id)
	if !ok || mc.Type != store.CertTypeACME {
		return fmt.Errorf("linked certificate is not ACME")
	}
	if err := srv.executeManagedCertRenew(id, renewApplyService); err != nil {
		srv.recordAcmeResult(err)
		return err
	}
	srv.recordAcmeResult(nil)
	srv.applyOCServCertAfterChange(id, "")
	srv.applySystemTLSAfterCertRenewNotify(id)
	return nil
}

func (srv *Server) applySystemTLSAfterCertRenew(certID string) {
	srv.applySystemTLSAfterCertRenewNotify(certID)
}
