package api

import (
	"crypto/tls"
	"os"
	"sync"
)

// tlsCertReloader 按文件 mtime 加载证书，供 TLS 握手使用（续期后无需重启进程）
type tlsCertReloader struct {
	certPath string
	keyPath  string
	mu       sync.Mutex
	cached   *tls.Certificate
	modCert  int64
	modKey   int64
}

func newTLSCertReloader(certPath, keyPath string) *tlsCertReloader {
	return &tlsCertReloader{certPath: certPath, keyPath: keyPath}
}

func fileModUnix(path string) int64 {
	fi, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return fi.ModTime().UnixNano()
}

func (r *tlsCertReloader) certificate() (*tls.Certificate, error) {
	mc := fileModUnix(r.certPath)
	mk := fileModUnix(r.keyPath)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cached != nil && r.modCert == mc && r.modKey == mk && mc != 0 {
		return r.cached, nil
	}
	cert, err := tls.LoadX509KeyPair(r.certPath, r.keyPath)
	if err != nil {
		return nil, err
	}
	r.cached = &cert
	r.modCert = mc
	r.modKey = mk
	return &cert, nil
}

func (r *tlsCertReloader) tlsConfig() *tls.Config {
	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
			return r.certificate()
		},
	}
}
