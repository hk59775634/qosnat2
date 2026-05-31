package api

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/hk59775634/qosnat2/internal/store"
)

const (
	defaultTLSCertPath = "/etc/qosnat2/tls.crt"
	defaultTLSKeyPath  = "/etc/qosnat2/tls.key"
)

// TLSStatus Web 展示用 HTTPS 状态（不含私钥内容）
type TLSStatus struct {
	Enabled       bool   `json:"tls_enabled"`
	Active        bool   `json:"tls_active"`
	CertPath      string `json:"cert_path"`
	KeyPath       string `json:"key_path"`
	HasCertFile   bool   `json:"has_cert_file"`
	HasKeyFile    bool   `json:"has_key_file"`
	CertSubject   string `json:"cert_subject,omitempty"`
	CertNotAfter  string `json:"cert_not_after,omitempty"`
	AcmeEnabled   bool   `json:"acme_enabled,omitempty"`
	Domain        string `json:"domain,omitempty"`
	AcmeEmail     string `json:"acme_email,omitempty"`
	AcmeStaging   bool   `json:"acme_staging,omitempty"`
	AcmeRenewDays int    `json:"acme_renew_days,omitempty"`
	AcmeLastOK    string `json:"acme_last_ok,omitempty"`
	AcmeLastError string `json:"acme_last_error,omitempty"`
	ManagedCertID string `json:"managed_cert_id,omitempty"`
}

func (srv *Server) tlsStatus() TLSStatus {
	st := srv.store.Get()
	certPath := defaultTLSCertPath
	keyPath := defaultTLSKeyPath
	if srv.env.TLSCert != "" {
		certPath = srv.env.TLSCert
	}
	if srv.env.TLSKey != "" {
		keyPath = srv.env.TLSKey
	}
	s := TLSStatus{
		Enabled:     st.System.TLSEnabled,
		Active:      srv.env.TLSCert != "" && srv.env.TLSKey != "",
		CertPath:    certPath,
		KeyPath:     keyPath,
		HasCertFile: tlsFileExists(certPath),
		HasKeyFile:  tlsFileExists(keyPath),
	}
	if s.HasCertFile {
		if subj, na := parseCertMeta(certPath); subj != "" {
			s.CertSubject = subj
			s.CertNotAfter = na
		}
	}
	return s
}

func tlsFileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func parseCertMeta(certPath string) (subject, notAfter string) {
	b, err := os.ReadFile(certPath)
	if err != nil {
		return "", ""
	}
	block, _ := pem.Decode(b)
	if block == nil {
		return "", ""
	}
	c, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", ""
	}
	subject = c.Subject.String()
	notAfter = c.NotAfter.UTC().Format(time.RFC3339)
	return subject, notAfter
}

func validateCertKeyPair(certPEM, keyPEM string) error {
	certPEM = strings.TrimSpace(certPEM)
	keyPEM = strings.TrimSpace(keyPEM)
	if certPEM == "" || keyPEM == "" {
		return fmt.Errorf("certificate and private key PEM are required")
	}
	if _, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM)); err != nil {
		return fmt.Errorf("invalid certificate or key: %w", err)
	}
	return nil
}

func writeTLSCertFiles(certPEM, keyPEM string) error {
	if err := os.MkdirAll("/etc/qosnat2", 0750); err != nil {
		return err
	}
	certPEM = normalizePEM(certPEM)
	keyPEM = normalizePEM(keyPEM)
	if err := os.WriteFile(defaultTLSCertPath, []byte(certPEM), 0640); err != nil {
		return err
	}
	return os.WriteFile(defaultTLSKeyPath, []byte(keyPEM), 0600)
}

func normalizePEM(s string) string {
	return strings.TrimSpace(s) + "\n"
}

func removeTLSFiles() {
	_ = os.Remove(defaultTLSCertPath)
	_ = os.Remove(defaultTLSKeyPath)
}

// applyTLS 写入证书、更新 env，并热切换 HTTP/HTTPS 监听（不重启 qosnatd）。
// 仅更新证书文件且模式不变时，tlsCertReloader 按 mtime 加载，可跳过监听重建。
func (srv *Server) applyTLS(enabled bool, certPEM, keyPEM string) (needsRestart bool, err error) {
	wasActive := srv.tlsActive()
	if enabled {
		if certPEM != "" && keyPEM != "" {
			if err := validateCertKeyPair(certPEM, keyPEM); err != nil {
				return false, err
			}
			if err := writeTLSCertFiles(certPEM, keyPEM); err != nil {
				return false, err
			}
		} else if !tlsFileExists(defaultTLSCertPath) || !tlsFileExists(defaultTLSKeyPath) {
			return false, fmt.Errorf("certificate and private key PEM are required")
		}
		srv.env.TLSCert = defaultTLSCertPath
		srv.env.TLSKey = defaultTLSKeyPath
	} else {
		srv.env.TLSCert = ""
		srv.env.TLSKey = ""
	}
	if err := writeRuntimeEnvMerged(srv.env); err != nil {
		return false, err
	}
	_ = srv.store.Update(func(st *store.State) {
		st.System.TLSEnabled = enabled
	})
	if err := srv.store.Save(); err != nil {
		return false, err
	}
	srv.reloadEnv()
	nowActive := srv.tlsActive()
	if !enabled && !wasActive {
		return false, nil
	}
	if enabled && wasActive && nowActive {
		return false, nil
	}
	if err := srv.reloadHTTPListener(); err != nil {
		return false, err
	}
	return false, nil
}

// scheduleQoSnatdRestart 在响应返回后重启，使 TLS 监听生效
func scheduleQoSnatdRestart() {
	go func() {
		time.Sleep(300 * time.Millisecond)
		if err := restartQoSnatd(); err != nil {
			log.Printf("restart qosnatd: %v", err)
		}
	}()
}

func restartQoSnatd() error {
	if os.Getuid() != 0 {
		return fmt.Errorf("TLS 已写入，请手动执行: systemctl restart qosnatd")
	}
	out, err := exec.Command("systemctl", "restart", "qosnatd").CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl restart qosnatd: %s %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func writeRuntimeEnvMerged(e Env) error {
	if err := os.MkdirAll("/etc/qosnat2", 0750); err != nil {
		return err
	}
	m := readEnvFileMap()
	if e.AdminUser != "" {
		m["ADMIN_USER"] = e.AdminUser
	}
	if e.AdminPass != "" {
		m["ADMIN_PASS"] = e.AdminPass
	}
	if e.AdminPort != "" {
		m["ADMIN_PORT"] = e.AdminPort
	}
	if e.DevLAN != "" {
		m["DEV_LAN"] = e.DevLAN
	}
	if e.DevWAN != "" {
		m["DEV_WAN"] = e.DevWAN
	}
	if e.StateFile != "" {
		m["STATE_FILE"] = e.StateFile
	}
	if e.SessionFile != "" {
		m["SESSION_FILE"] = e.SessionFile
	}
	if e.OpenAPIPath != "" {
		m["OPENAPI_PATH"] = e.OpenAPIPath
	}
	if e.WebRoot != "" {
		m["WEB_ROOT"] = e.WebRoot
	}
	if e.TLSCert != "" && e.TLSKey != "" {
		m["TLS_CERT"] = e.TLSCert
		m["TLS_KEY"] = e.TLSKey
	} else {
		delete(m, "TLS_CERT")
		delete(m, "TLS_KEY")
	}
	return writeEnvMap(m)
}

func readEnvFileMap() map[string]string {
	m := map[string]string{}
	b, err := os.ReadFile(defaultEnvPath)
	if err != nil {
		return m
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		i := strings.IndexByte(line, '=')
		if i <= 0 {
			continue
		}
		m[strings.TrimSpace(line[:i])] = strings.TrimSpace(line[i+1:])
	}
	return m
}

func writeEnvMap(m map[string]string) error {
	order := []string{"ADMIN_USER", "ADMIN_PASS", "ADMIN_PORT", "DEV_LAN", "DEV_WAN",
		"STATE_FILE", "SESSION_FILE", "OPENAPI_PATH", "WEB_ROOT", "TLS_CERT", "TLS_KEY"}
	var b strings.Builder
	b.WriteString("# Generated by qosnat2 — do not edit unless you know what you are doing\n")
	written := map[string]bool{}
	for _, k := range order {
		if v, ok := m[k]; ok && v != "" {
			fmt.Fprintf(&b, "%s=%s\n", k, v)
			written[k] = true
		}
	}
	for k, v := range m {
		if written[k] || v == "" {
			continue
		}
		fmt.Fprintf(&b, "%s=%s\n", k, v)
	}
	return os.WriteFile(defaultEnvPath, []byte(b.String()), 0600)
}
