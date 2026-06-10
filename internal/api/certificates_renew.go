package api

import (
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/hk59775634/qosnat2/internal/certs"
	"github.com/hk59775634/qosnat2/internal/store"
)

func (srv *Server) updateCertificateByID(id string, fn func(*store.ManagedCertificate)) {
	_ = srv.store.Update(func(s *store.State) {
		for i := range s.Certificates {
			if s.Certificates[i].ID == id {
				fn(&s.Certificates[i])
				break
			}
		}
	})
	if err := srv.store.Save(); err != nil {
		log.Printf("update certificate %s: save: %v", id, err)
	}
}

func (srv *Server) recordCertRenewFailure(id string, err error) {
	info := certs.ClassifyACMEError(err)
	srv.updateCertificateByID(id, func(c *store.ManagedCertificate) {
		c.AcmeLastError = info.Summary
		if info.PauseAutoRenew {
			c.AutoRenewPaused = true
			c.AutoRenewEnabled = false
			c.AutoRenewPauseReason = info.Summary
		}
	})
}

func (srv *Server) recordCertRenewSuccess(id string, renewed *store.ManagedCertificate) {
	srv.updateCertificateByID(id, func(c *store.ManagedCertificate) {
		*c = *renewed
		c.AutoRenewPaused = false
		c.AutoRenewPauseReason = ""
		c.AutoRenewEnabled = true
		c.AcmeLastError = ""
	})
}

type renewApplyMode int

func managedCertHTTP01Target(mc store.ManagedCertificate) string {
	if len(mc.Domains) > 0 {
		return mc.Domains[0]
	}
	return mc.Name
}

const (
	renewApplyNone renewApplyMode = iota
	renewApplyLibrary
	renewApplyService
)

func (srv *Server) executeManagedCertRenew(id string, mode renewApplyMode) error {
	st := srv.store.Get()
	var prev store.ManagedCertificate
	for _, c := range st.Certificates {
		if c.ID == id {
			prev = c
			break
		}
	}
	if prev.ID == "" {
		return errCertNotFound
	}
	if prev.Type != store.CertTypeACME {
		return errCertNotACME
	}
	var renewed *store.ManagedCertificate
	var err error
	target := managedCertHTTP01Target(prev)
	err = srv.withAcmeHTTP01Port80Open(target, func() error {
		certAcmeMu.Lock()
		defer certAcmeMu.Unlock()
		renewed, err = certs.RenewACME(prev)
		return err
	})
	if err != nil {
		srv.recordCertRenewFailure(id, err)
		return err
	}
	srv.recordCertRenewSuccess(id, renewed)
	switch mode {
	case renewApplyLibrary:
		srv.reloadOCServIfCertInUse(id)
		srv.applySystemTLSAfterCertRenewNotify(id)
	case renewApplyService, renewApplyNone:
		// 由调用方处理应用与通知
	}
	return nil
}

var (
	errCertNotFound = &certOpError{msg: "certificate not found"}
	errCertNotACME  = &certOpError{msg: "only ACME certificates can be renewed"}
)

type certOpError struct{ msg string }

func (e *certOpError) Error() string { return e.msg }

func (srv *Server) maybeRenewManagedCertificates() {
	if os.Getuid() != 0 {
		return
	}
	st := srv.store.Get()
	for _, c := range st.Certificates {
		if store.CertIsServiceBound(c.ID, st) {
			continue
		}
		if !store.CertShouldAutoRenew(c) {
			continue
		}
		days := certs.DaysUntilExpiry(c.CertPath)
		if days < 0 {
			continue
		}
		renewBefore := store.CertAcmeRenewBeforeDays(c)
		if days > renewBefore {
			continue
		}
		log.Printf("acme: auto-renew cert %s (%s) expires in %d days", c.ID, c.Name, days)
		if err := srv.executeManagedCertRenew(c.ID, renewApplyLibrary); err != nil {
			log.Printf("acme auto-renew %s: %v", c.ID, err)
			info := certs.ClassifyACMEError(err)
			msg := err.Error()
			if info.Summary != "" {
				msg = info.Summary
			}
			srv.pushUINotification("error", "证书库自动续期失败", c.Name+": "+msg, "#/system/certificates")
		}
	}
}

func (srv *Server) runCertificateAutoRenewTick() {
	srv.maybeRenewServiceBoundCertificates()
	srv.maybeRenewManagedCertificates()
}

func renewErrorResponse(err error) map[string]any {
	info := certs.ClassifyACMEError(err)
	return map[string]any{
		"ok":                  false,
		"code":                "CERT_ACME_FAILED",
		"error":               err.Error(),
		"pause_auto_renew":    info.PauseAutoRenew,
		"auto_renew_paused":   info.PauseAutoRenew,
		"renew_error_summary": info.Summary,
	}
}

func (srv *Server) handleCertificateAutoRenew(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	var body struct {
		ID      string `json:"id"`
		Enabled *bool  `json:"enabled"`
		Resume  bool   `json:"resume"`
	}
	if err := readJSON(r, &body); err != nil {
		writeBadJSON(w)
		return
	}
	id := strings.TrimSpace(body.ID)
	if id == "" {
		writeBadRequest(w, "id required")
		return
	}
	st := srv.store.Get()
	mc, ok := store.FindManagedCert(st.Certificates, id)
	if !ok {
		writeNotFound(w, "certificate not found")
		return
	}
	if mc.Type != store.CertTypeACME {
		writeBadRequest(w, "only ACME certificates support auto-renew")
		return
	}
	enable := true
	if body.Enabled != nil {
		enable = *body.Enabled
	}
	if body.Resume {
		enable = true
	}
	srv.updateCertificateByID(id, func(c *store.ManagedCertificate) {
		c.AutoRenewEnabled = enable
		if enable {
			c.AutoRenewPaused = false
			c.AutoRenewPauseReason = ""
		} else {
			c.AutoRenewPaused = true
			if c.AutoRenewPauseReason == "" {
				c.AutoRenewPauseReason = "已手动关闭自动续期"
			}
		}
	})
	srv.auditLog(r, "system.certificates.auto_renew", id)
	st2 := srv.store.Get()
	mc2, _ := store.FindManagedCert(st2.Certificates, id)
	pub := certificatePublic{ManagedCertificate: mc2}
	pub.DaysUntilExpiry = certs.DaysUntilExpiry(mc2.CertPath)
	pub.InUse = store.ManagedCertUsages(id, st2)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "certificate": pub})
}
