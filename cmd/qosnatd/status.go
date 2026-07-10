package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/hk59775634/qosnat2/internal/api"
	"github.com/hk59775634/qosnat2/internal/netif"
	"github.com/hk59775634/qosnat2/internal/store"
)

func runStatus() {
	env := api.LoadEnv()
	st := store.New(env.StateFile)
	_ = st.Load()
	state := st.Get()

	user := strings.TrimSpace(env.AdminUser)
	if u := strings.TrimSpace(state.AdminUser); u != "" {
		user = u
	}
	if user == "" {
		user = "admin"
	}

	pass := strings.TrimSpace(env.AdminPass)
	if pass == "" {
		pass = readInitialAdminPass()
	}
	if pass == "" {
		if state.AdminPassHash != "" {
			pass = "(unavailable — stored as hash only; check /etc/qosnat2/initial-admin.txt if kept)"
		} else {
			pass = "(not set)"
		}
	}

	port := strings.TrimSpace(env.AdminPort)
	if port == "" {
		port = "8080"
	}

	tlsOn := state.System.TLSEnabled &&
		env.TLSCert != "" && env.TLSKey != "" &&
		fileExists(env.TLSCert) && fileExists(env.TLSKey)
	scheme := "http"
	if tlsOn {
		scheme = "https"
	}

	host := strings.TrimSpace(state.System.TLSDomain)
	if host == "" {
		host = primaryIPv4(env.DevWAN)
	}
	if host == "" {
		host = primaryIPv4(env.DevLAN)
	}
	if host == "" {
		host = "127.0.0.1"
	}

	url := fmt.Sprintf("%s://%s", scheme, net.JoinHostPort(host, port))
	listening := portListening(port)

	fmt.Printf("URL:      %s\n", url)
	fmt.Printf("Username: %s\n", user)
	fmt.Printf("Password: %s\n", pass)
	fmt.Printf("Listen:   :%s", port)
	if listening {
		fmt.Printf(" (listening)\n")
	} else {
		fmt.Printf(" (not listening)\n")
	}
}

func primaryIPv4(dev string) string {
	ip, err := netif.PrimaryIPv4(dev)
	if err != nil {
		return ""
	}
	return ip
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func readInitialAdminPass() string {
	f, err := os.Open("/etc/qosnat2/initial-admin.txt")
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if strings.TrimSpace(k) == "ADMIN_PASS" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func portListening(port string) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", port), 500*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func waitPortListen(port string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if portListening(port) {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func runStatusAfterRestart() {
	env := api.LoadEnv()
	port := strings.TrimSpace(env.AdminPort)
	if port == "" {
		port = "8080"
	}
	waitPortListen(port, 15*time.Second)
	runStatus()
}
