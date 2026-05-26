package api

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/hk59775634/qosnat2/internal/acme"
	"github.com/hk59775634/qosnat2/internal/store"
)

var acmeOpMu sync.Mutex

func acmeRenewBeforeDays(st store.SystemState) int {
	if st.TLSAcmeRenewDays > 0 {
		return st.TLSAcmeRenewDays
	}
	return 30
}

func (srv *Server) tlsStatusWithAcme() TLSStatus {
	s := srv.tlsStatus()
	st := srv.store.Get().System
	s.AcmeEnabled = st.TLSAcmeEnabled
	s.Domain = st.TLSDomain
	s.AcmeEmail = st.TLSAcmeEmail
	s.AcmeStaging = st.TLSAcmeStaging
	s.AcmeRenewDays = acmeRenewBeforeDays(st)
	s.AcmeLastOK = st.TLSAcmeLastOK
	s.AcmeLastError = st.TLSAcmeLastError
	s.ManagedCertID = strings.TrimSpace(st.TLSManagedCertID)
	return s
}

func (srv *Server) acmeConfigFromStore() (acme.Config, error) {
	st := srv.store.Get().System
	domain, err := acme.NormalizeDomain(st.TLSDomain)
	if err != nil {
		return acme.Config{}, err
	}
	return acme.Config{
		Domain:  domain,
		Email:   strings.TrimSpace(st.TLSAcmeEmail),
		Staging: st.TLSAcmeStaging,
	}, nil
}

func (srv *Server) recordAcmeResult(err error) {
	now := time.Now().UTC().Format(time.RFC3339)
	_ = srv.store.Update(func(s *store.State) {
		if err != nil {
			s.System.TLSAcmeLastError = err.Error()
		} else {
			s.System.TLSAcmeLastError = ""
			s.System.TLSAcmeLastOK = now
		}
	})
	_ = srv.store.Save()
}

func (srv *Server) runACMEObtain() error {
	acmeOpMu.Lock()
	defer acmeOpMu.Unlock()
	cfg, err := srv.acmeConfigFromStore()
	if err != nil {
		return err
	}
	res, err := acme.Obtain(cfg)
	if err != nil {
		srv.recordAcmeResult(err)
		return err
	}
	if _, err := srv.applyTLS(true, res.CertPEM, res.KeyPEM); err != nil {
		srv.recordAcmeResult(err)
		return err
	}
	if _, err := srv.upsertSystemTLSManagedCert(res.CertPEM, res.KeyPEM, true, cfg.Domain, cfg.Email, cfg.Staging); err != nil {
		log.Printf("upsert system tls cert: %v", err)
	}
	srv.recordAcmeResult(nil)
	return nil
}

func (srv *Server) runACMERenew() error {
	acmeOpMu.Lock()
	defer acmeOpMu.Unlock()
	st := srv.store.Get()
	if id := strings.TrimSpace(st.System.TLSManagedCertID); id != "" {
		if mc, ok := store.FindManagedCert(st.Certificates, id); ok && mc.Type == store.CertTypeACME {
			return srv.renewSystemManagedCertACME()
		}
	}
	cfg, err := srv.acmeConfigFromStore()
	if err != nil {
		return err
	}
	certPEM, err := os.ReadFile(defaultTLSCertPath)
	if err != nil {
		return fmt.Errorf("read cert: %w", err)
	}
	keyPEM, err := os.ReadFile(defaultTLSKeyPath)
	if err != nil {
		return fmt.Errorf("read key: %w", err)
	}
	res, err := acme.Renew(cfg, string(certPEM), string(keyPEM))
	if err != nil {
		srv.recordAcmeResult(err)
		return err
	}
	if _, err := srv.applyTLS(true, res.CertPEM, res.KeyPEM); err != nil {
		srv.recordAcmeResult(err)
		return err
	}
	if _, err := srv.upsertSystemTLSManagedCert(res.CertPEM, res.KeyPEM, true, cfg.Domain, cfg.Email, cfg.Staging); err != nil {
		log.Printf("upsert system tls cert: %v", err)
	}
	srv.recordAcmeResult(nil)
	return nil
}

func (srv *Server) maybeAutoRenewACME() {
	st := srv.store.Get()
	sys := st.System
	if !sys.TLSEnabled || !sys.TLSAcmeEnabled {
		return
	}
	if strings.TrimSpace(sys.TLSManagedCertID) != "" {
		return
	}
	if strings.TrimSpace(sys.TLSDomain) == "" {
		return
	}
	if !tlsFileExists(defaultTLSCertPath) {
		return
	}
	days, err := acme.DaysUntilExpiry(defaultTLSCertPath)
	if err != nil || days < 0 {
		return
	}
	renewBefore := store.ServiceAutoRenewDays
	if sys.TLSAcmeRenewDays > 0 && sys.TLSAcmeRenewDays < renewBefore {
		renewBefore = sys.TLSAcmeRenewDays
	}
	if days > renewBefore {
		return
	}
	log.Printf("acme: legacy system TLS expires in %d days, renewing %s", days, sys.TLSDomain)
	if err := srv.runACMERenew(); err != nil {
		log.Printf("acme renew: %v", err)
	}
}

func (srv *Server) startACMEBackground() {
	go func() {
		time.Sleep(15 * time.Second)
		srv.maybeAutoRenewACME()
		t := time.NewTicker(12 * time.Hour)
		defer t.Stop()
		for range t.C {
			srv.maybeAutoRenewACME()
		}
	}()
}

func (srv *Server) handleTLSAcme(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if os.Getuid() != 0 {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "ACME 需要 root 运行 qosnatd"})
		return
	}
	var body struct {
		CurrentPassword string `json:"current_password"`
		Action          string `json:"action"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
		return
	}
	st := srv.store.Get()
	if !srv.verifyAdmin(st.AdminUser, body.CurrentPassword) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "current password required"})
		return
	}
	action := strings.TrimSpace(body.Action)
	if action == "" {
		action = strings.TrimSpace(r.URL.Query().Get("action"))
	}
	if action == "" {
		action = "obtain"
	}
	var runErr error
	switch action {
	case "obtain":
		runErr = srv.runACMEObtain()
	case "renew":
		runErr = srv.runACMERenew()
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "action must be obtain or renew"})
		return
	}
	if runErr != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": runErr.Error(),
			"tls":   srv.tlsStatusWithAcme(),
		})
		return
	}
	srv.auditLog(r, "system.tls.acme", action)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"message": "ACME 证书已更新，qosnatd 正在重启",
		"tls":     srv.tlsStatusWithAcme(),
	})
}
