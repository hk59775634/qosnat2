package api

import (
	"fmt"
	"os"
	"strings"

	"github.com/hk59775634/qosnat2/internal/certs"
	"github.com/hk59775634/qosnat2/internal/store"
)

// InstallACMEIPSSL 安装阶段：申请 IP 短期证书并写入 state/env，供首次启动即启用 HTTPS。
func InstallACMEIPSSL(st *store.Store, env Env, ip, email string, staging bool) error {
	if os.Getuid() != 0 {
		return fmt.Errorf("ACME IP 证书申请需要 root")
	}
	mc, err := certs.ObtainACMEIP(ip, email, staging)
	if err != nil {
		return err
	}
	if err := store.NormalizeManagedCertificate(mc); err != nil {
		_ = certs.RemoveDir(mc.ID)
		return err
	}
	certPEM, err := os.ReadFile(mc.CertPath)
	if err != nil {
		return err
	}
	keyPEM, err := os.ReadFile(mc.KeyPath)
	if err != nil {
		return err
	}
	if err := writeTLSCertFiles(string(certPEM), string(keyPEM)); err != nil {
		return err
	}
	env.TLSCert = defaultTLSCertPath
	env.TLSKey = defaultTLSKeyPath
	if err := writeRuntimeEnvMerged(env); err != nil {
		return err
	}
	_ = st.Update(func(s *store.State) {
		found := false
		for i := range s.Certificates {
			if s.Certificates[i].ID == mc.ID {
				s.Certificates[i] = *mc
				found = true
				break
			}
		}
		if !found {
			s.Certificates = append(s.Certificates, *mc)
		}
		s.System.TLSManagedCertID = mc.ID
		s.System.TLSEnabled = true
		s.System.TLSDomain = ip
		s.System.TLSAcmeEnabled = true
		s.System.TLSAcmeEmail = strings.TrimSpace(email)
		s.System.TLSAcmeStaging = staging
		s.System.TLSAcmeRenewDays = store.DefaultIPAcmeAutoRenewDays
		s.System.TLSAcmeLastOK = mc.AcmeLastOK
		s.System.TLSAcmeLastError = ""
	})
	if err := st.Save(); err != nil {
		return err
	}
	return nil
}
