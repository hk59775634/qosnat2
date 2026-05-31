package api

import (
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/hk59775634/qosnat2/internal/acme"
	"github.com/hk59775634/qosnat2/internal/certs"
	"github.com/hk59775634/qosnat2/internal/ocserv"
	"github.com/hk59775634/qosnat2/internal/store"
)

var certAcmeMu sync.Mutex

type certificatePublic struct {
	store.ManagedCertificate
	DaysUntilExpiry int               `json:"days_until_expiry,omitempty"`
	InUse           []store.CertUsage `json:"in_use,omitempty"`
}

func (srv *Server) reloadOCServIfCertInUse(id string) {
	st := srv.store.Get()
	if len(store.ManagedCertUsages(id, st)) == 0 {
		return
	}
	o := st.VPN.OCServ
	if !ocserv.InstallInfo().Active {
		return
	}
	if err := ocserv.Reload(o); err != nil {
		log.Printf("ocserv reload after cert %s renew: %v", id, err)
	}
}

func certificatePublicList(st store.State) []certificatePublic {
	out := make([]certificatePublic, 0, len(st.Certificates))
	for _, c := range st.Certificates {
		p := certificatePublic{ManagedCertificate: c}
		p.DaysUntilExpiry = certs.DaysUntilExpiry(c.CertPath)
		p.InUse = store.ManagedCertUsages(c.ID, st)
		out = append(out, p)
	}
	return out
}

func (srv *Server) handleCertificates(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		srv.migrateCertificateAutoRenew()
		st := srv.store.Get()
		writeJSON(w, http.StatusOK, map[string]any{"certificates": certificatePublicList(st)})
	case http.MethodPost:
		if os.Getuid() != 0 {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "证书操作需要 root 运行 qosnatd"})
			return
		}
		var body struct {
			Type    string `json:"type"`
			Name    string `json:"name"`
			Domain  string `json:"domain"`
			Email   string `json:"email"`
			Staging bool   `json:"staging"`
			CertPEM string `json:"cert_pem"`
			KeyPEM  string `json:"key_pem"`
			CaPEM   string `json:"ca_pem"`
		}
		if err := readJSON(r, &body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
			return
		}
		typ := strings.ToLower(strings.TrimSpace(body.Type))
		var mc store.ManagedCertificate
		switch typ {
		case store.CertTypeManual:
			mc = store.ManagedCertificate{
				Name: body.Name,
				Type: store.CertTypeManual,
			}
			if err := store.NormalizeManagedCertificate(&mc); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			certPath, keyPath, caPath, err := certs.SavePEM(mc.ID, body.CertPEM, body.KeyPEM, body.CaPEM)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			mc.CertPath, mc.KeyPath, mc.CaPath = certPath, keyPath, caPath
			mc.Subject, mc.NotAfter = certs.ParseMeta(certPath)
		case store.CertTypeACME:
			var mcPtr *store.ManagedCertificate
			var err error
			err = srv.withAcmeHTTP01Port80Open(func() error {
				certAcmeMu.Lock()
				defer certAcmeMu.Unlock()
				if _, ipErr := acme.NormalizeIP(body.Domain); ipErr == nil {
					mcPtr, err = certs.ObtainACMEIP(body.Domain, body.Email, body.Staging)
				} else {
					mcPtr, err = certs.ObtainACME(body.Domain, body.Email, body.Staging)
				}
				return err
			})
			if err != nil {
				writeJSON(w, http.StatusBadRequest, renewErrorResponse(err))
				return
			}
			mc = *mcPtr
			if n := strings.TrimSpace(body.Name); n != "" {
				mc.Name = n
			}
			if err := store.NormalizeManagedCertificate(&mc); err != nil {
				_ = certs.RemoveDir(mc.ID)
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			mc.AutoRenewEnabled = true
		default:
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "type must be manual or acme"})
			return
		}
		_ = srv.store.Update(func(s *store.State) {
			s.Certificates = append(s.Certificates, mc)
		})
		if !srv.persistState(w) {
			return
		}
		st := srv.store.Get()
		srv.auditLog(r, "system.certificates.create", mc.ID)
		pub := certificatePublic{ManagedCertificate: mc}
		pub.DaysUntilExpiry = certs.DaysUntilExpiry(mc.CertPath)
		pub.InUse = store.ManagedCertUsages(mc.ID, st)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "certificate": pub})
	case http.MethodDelete:
		id := strings.TrimSpace(r.URL.Query().Get("id"))
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
			return
		}
		st := srv.store.Get()
		if u := store.ManagedCertUsages(id, st); len(u) > 0 {
			writeJSON(w, http.StatusConflict, map[string]any{
				"error":  "certificate in use",
				"in_use": u,
			})
			return
		}
		found := false
		_ = srv.store.Update(func(s *store.State) {
			var out []store.ManagedCertificate
			for _, c := range s.Certificates {
				if c.ID == id {
					found = true
					continue
				}
				out = append(out, c)
			}
			s.Certificates = out
		})
		if !found {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "certificate not found"})
			return
		}
		if !srv.persistState(w) {
			return
		}
		_ = certs.RemoveDir(id)
		srv.auditLog(r, "system.certificates.delete", id)
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (srv *Server) handleCertificateRenew(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if os.Getuid() != 0 {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "ACME 续签需要 root 运行 qosnatd"})
		return
	}
	var body struct {
		ID string `json:"id"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
		return
	}
	id := strings.TrimSpace(body.ID)
	if id == "" {
		id = strings.TrimSpace(r.URL.Query().Get("id"))
	}
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
		return
	}
	if err := srv.executeManagedCertRenew(id, renewApplyNone); err != nil {
		writeJSON(w, http.StatusBadRequest, renewErrorResponse(err))
		return
	}
	srv.applyOCServCertAfterChange(id, "")
	srv.applySystemTLSAfterCertRenewNotify(id)
	srv.auditLog(r, "system.certificates.renew", id)
	st2 := srv.store.Get()
	mc2, _ := store.FindManagedCert(st2.Certificates, id)
	pub := certificatePublic{ManagedCertificate: mc2}
	pub.DaysUntilExpiry = certs.DaysUntilExpiry(mc2.CertPath)
	pub.InUse = store.ManagedCertUsages(id, st2)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "certificate": pub})
}

func (srv *Server) migrateCertificateAutoRenew() {
	changed := false
	_ = srv.store.Update(func(s *store.State) {
		for i := range s.Certificates {
			c := &s.Certificates[i]
			if c.Type != store.CertTypeACME {
				continue
			}
			if c.AcmeRenewDays <= 0 {
				c.AcmeRenewDays = store.DefaultAcmeAutoRenewDays
				changed = true
			} else if !store.CertIsServiceBound(c.ID, *s) && c.AcmeRenewDays < store.DefaultAcmeAutoRenewDays {
				c.AcmeRenewDays = store.DefaultAcmeAutoRenewDays
				changed = true
			}
			if !c.AutoRenewPaused && !c.AutoRenewEnabled {
				c.AutoRenewEnabled = true
				changed = true
			}
		}
	})
	if changed {
		_ = srv.persistStateOrLog("migrate managed certs defaults")
	}
}

func (srv *Server) startManagedCertsBackground() {
	go func() {
		time.Sleep(20 * time.Second)
		srv.runCertificateAutoRenewTick()
		t := time.NewTicker(12 * time.Hour)
		defer t.Stop()
		for range t.C {
			srv.runCertificateAutoRenewTick()
		}
	}()
}
