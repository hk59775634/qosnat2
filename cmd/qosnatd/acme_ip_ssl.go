package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/hk59775634/qosnat2/internal/acme"
	"github.com/hk59775634/qosnat2/internal/api"
	"github.com/hk59775634/qosnat2/internal/store"
)

func runAcmeIPSSL() {
	fs := flag.NewFlagSet("acme-ip-ssl", flag.ExitOnError)
	ip := fs.String("ip", "", "public IPv4 for certificate (default: auto-detect)")
	email := fs.String("email", os.Getenv("ACME_EMAIL"), "Let's Encrypt account email (or set ACME_EMAIL)")
	staging := fs.Bool("staging", os.Getenv("ACME_STAGING") == "1", "use Let's Encrypt staging")
	_ = fs.Parse(os.Args[2:])

	addr := strings.TrimSpace(*ip)
	if addr == "" {
		var err error
		addr, err = detectInstallIPv4()
		if err != nil {
			log.Fatalf("detect ip: %v (use --ip)", err)
		}
	}
	if _, err := acme.NormalizeIP(addr); err != nil {
		log.Fatalf("ip: %v", err)
	}
	mail := strings.TrimSpace(*email)
	if mail == "" {
		log.Fatal("ACME email required: --email or ACME_EMAIL")
	}

	env := api.LoadEnv()
	if err := api.ValidateEnv(env); err != nil {
		log.Fatalf("config: %v", err)
	}
	st := store.New(env.StateFile)
	if err := st.Load(); err != nil {
		log.Fatalf("load state: %v", err)
	}
	log.Printf("requesting Let's Encrypt IP certificate for %s (profile shortlived, HTTP-01 on :80)", addr)
	if err := api.InstallACMEIPSSL(st, env, addr, mail, *staging); err != nil {
		log.Fatalf("acme ip ssl: %v", err)
	}
	fmt.Printf("OK ip=%s cert=/etc/qosnat2/tls.crt\n", addr)
}

func detectInstallIPv4() (string, error) {
	if v := strings.TrimSpace(os.Getenv("PUBLIC_IP")); v != "" {
		return acme.NormalizeIP(v)
	}
	return "", fmt.Errorf("set --ip or PUBLIC_IP")
}
