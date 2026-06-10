package acme

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge/http01"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/registration"
)

const (
	AccountKeyPath = "/var/lib/qosnat2/acme/account.key"
	AccountRegPath = "/var/lib/qosnat2/acme/account.json"
)

// Config ACME 申请参数
type Config struct {
	Domain  string
	Email   string
	Staging bool
}

// Result 证书 PEM
type Result struct {
	CertPEM  string
	KeyPEM   string
	NotAfter time.Time
}

type legoUser struct {
	email        string
	key          crypto.PrivateKey
	registration *registration.Resource
}

func (u *legoUser) GetEmail() string                        { return u.email }
func (u *legoUser) GetRegistration() *registration.Resource { return u.registration }
func (u *legoUser) GetPrivateKey() crypto.PrivateKey        { return u.key }

func NormalizeDomain(d string) (string, error) {
	d = strings.TrimSpace(strings.ToLower(d))
	d = strings.TrimPrefix(d, "https://")
	d = strings.TrimPrefix(d, "http://")
	if i := strings.IndexByte(d, '/'); i >= 0 {
		d = d[:i]
	}
	if i := strings.IndexByte(d, ':'); i >= 0 {
		d = d[:i]
	}
	if d == "" {
		return "", fmt.Errorf("domain required")
	}
	if strings.Contains(d, " ") || !strings.Contains(d, ".") {
		return "", fmt.Errorf("invalid domain")
	}
	return d, nil
}

// Obtain 通过 HTTP-01 申请证书（需公网 80 端口可达）
func Obtain(cfg Config) (*Result, error) {
	var res *certificate.Resource
	err := withHTTP01PortOpen(cfg.Domain, func() error {
		client, domain, err := setupClient(cfg)
		if err != nil {
			return err
		}
		res, err = client.Certificate.Obtain(certificate.ObtainRequest{
			Domains: []string{domain},
			Bundle:  true,
		})
		if err != nil {
			return err
		}
		_ = domain
		return nil
	})
	if err != nil {
		return nil, err
	}
	return resultFromResource(res)
}

// Renew 续期：对同一域名重新签发（HTTP-01）
func Renew(cfg Config, certPEM, keyPEM string) (*Result, error) {
	_ = certPEM
	_ = keyPEM
	return Obtain(cfg)
}

// DaysUntilExpiry 距证书到期天数（文件不存在返回 -1）
func DaysUntilExpiry(certPath string) (int, error) {
	b, err := os.ReadFile(certPath)
	if err != nil {
		return -1, err
	}
	block, _ := pem.Decode(b)
	if block == nil {
		return -1, fmt.Errorf("invalid cert pem")
	}
	c, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return -1, err
	}
	return int(time.Until(c.NotAfter).Hours() / 24), nil
}

func setupClient(cfg Config) (*lego.Client, string, error) {
	domain, err := NormalizeDomain(cfg.Domain)
	if err != nil {
		return nil, "", err
	}
	email := strings.TrimSpace(cfg.Email)
	if email == "" {
		return nil, "", fmt.Errorf("ACME 邮箱必填（Let's Encrypt 账户）")
	}
	client, err := newClient(email, cfg.Staging, false)
	if err != nil {
		return nil, "", err
	}
	if err := client.Challenge.SetHTTP01Provider(http01.NewProviderServer("", "80")); err != nil {
		return nil, "", fmt.Errorf("http-01 provider: %w", err)
	}
	return client, domain, nil
}

func resultFromResource(res *certificate.Resource) (*Result, error) {
	if res == nil || len(res.Certificate) == 0 || len(res.PrivateKey) == 0 {
		return nil, fmt.Errorf("empty certificate from ACME")
	}
	certPEM := string(res.Certificate)
	if len(res.IssuerCertificate) > 0 {
		certPEM = certPEM + "\n" + string(res.IssuerCertificate)
	}
	na := time.Time{}
	if certs, err := certcrypto.ParsePEMBundle(res.Certificate); err == nil && len(certs) > 0 {
		na = certs[0].NotAfter
	}
	return &Result{
		CertPEM:  certPEM,
		KeyPEM:   string(res.PrivateKey),
		NotAfter: na,
	}, nil
}

func newClient(email string, staging bool, disableCN bool) (*lego.Client, error) {
	if err := os.MkdirAll(filepath.Dir(AccountKeyPath), 0750); err != nil {
		return nil, err
	}
	accountKey, err := loadOrCreateAccountKey()
	if err != nil {
		return nil, err
	}
	user := &legoUser{email: email, key: accountKey}
	if reg, err := loadRegistration(staging); err == nil && reg != nil && reg.URI != "" {
		user.registration = reg
	}
	config := lego.NewConfig(user)
	config.Certificate.KeyType = certcrypto.RSA2048
	config.Certificate.DisableCommonName = disableCN
	if staging {
		config.CADirURL = lego.LEDirectoryStaging
	} else {
		config.CADirURL = lego.LEDirectoryProduction
	}
	client, err := lego.NewClient(config)
	if err != nil {
		return nil, err
	}
	if user.registration == nil || user.registration.URI == "" {
		reg, err := client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
		if err != nil {
			return nil, fmt.Errorf("acme register: %w", err)
		}
		user.registration = reg
		_ = saveRegistration(reg, staging)
	}
	return client, nil
}

func loadOrCreateAccountKey() (crypto.PrivateKey, error) {
	if b, err := os.ReadFile(AccountKeyPath); err == nil {
		block, _ := pem.Decode(b)
		if block != nil {
			return x509.ParseECPrivateKey(block.Bytes)
		}
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, err
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})
	if err := os.WriteFile(AccountKeyPath, pemBytes, 0600); err != nil {
		return nil, err
	}
	return key, nil
}

