package api

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/hk59775634/qosnat2/internal/dnsmasq"
	"github.com/hk59775634/qosnat2/internal/nft"
	"github.com/hk59775634/qosnat2/internal/route"
	"github.com/hk59775634/qosnat2/internal/shaper"
	"github.com/hk59775634/qosnat2/internal/store"
)

func (srv *Server) handleSetupStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	complete := srv.setupComplete()
	st := srv.store.Get()
	user := srv.env.AdminUser
	if user == "" {
		user = defaultAdminUser
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"setup_required": !complete,
		"setup_complete": complete,
		"admin_user":     user,
		"admin_port":     srv.env.AdminPort,
		"has_admin":      st.AdminPassHash != "",
	})
}

func (srv *Server) handleSetupInterfaces(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if srv.setupComplete() {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "setup already complete"})
		return
	}
	ifaces, err := dnsmasq.ListInterfaces()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"interfaces": ifaces})
}

func (srv *Server) handleSetupComplete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if srv.setupComplete() {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "setup already complete"})
		return
	}
	var body struct {
		AdminUser      string   `json:"admin_user"`
		AdminPass      string   `json:"admin_pass"`
		DevLAN         string   `json:"dev_lan"`
		DevWAN         string   `json:"dev_wan"`
		PolicyRoutes   []string `json:"policy_routes"`
		SharedIPs      []string `json:"shared_ips"`
		Hostname       string   `json:"hostname"`
		EnableDHCP     bool     `json:"enable_dhcp"`
		ApplyDataplane bool     `json:"apply_dataplane"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
		return
	}
	body.AdminUser = strings.TrimSpace(body.AdminUser)
	body.DevLAN = strings.TrimSpace(body.DevLAN)
	body.DevWAN = strings.TrimSpace(body.DevWAN)
	if body.AdminUser == "" {
		body.AdminUser = strings.TrimSpace(srv.env.AdminUser)
		if body.AdminUser == "" {
			body.AdminUser = defaultAdminUser
		}
	}
	passToHash := body.AdminPass
	if len(passToHash) < 8 {
		if p := srv.env.AdminPass; len(p) >= 8 {
			passToHash = p
		} else {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "admin_pass must be at least 8 characters (or set a new password in wizard)"})
			return
		}
	}
	if body.DevWAN == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "dev_wan required"})
		return
	}
	if body.DevLAN != "" && body.DevLAN == body.DevWAN {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "dev_lan and dev_wan must differ"})
		return
	}
	if body.DevLAN != "" && !route.LinkExists(body.DevLAN) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("dev_lan: interface %q not found", body.DevLAN)})
		return
	}
	if !route.LinkExists(body.DevWAN) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("dev_wan: interface %q not found", body.DevWAN)})
		return
	}
	hash, err := hashPassword(passToHash)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "session error"})
		return
	}
	if len(body.PolicyRoutes) == 0 {
		body.PolicyRoutes = []string{"10.0.0.0/8"}
	}
	for i, cidr := range body.PolicyRoutes {
		if err := store.ValidateCIDR(cidr); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "policy_routes: " + err.Error()})
			return
		}
		body.PolicyRoutes[i] = strings.TrimSpace(cidr)
	}
	for _, ip := range body.SharedIPs {
		ip = strings.TrimSpace(ip)
		if ip == "" {
			continue
		}
		tmp := store.DefaultState()
		if err := nft.AddSharedIP(&tmp, ip); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "shared_ips: " + err.Error()})
			return
		}
	}

	passHash := string(hash)
	_ = srv.store.Update(func(st *store.State) {
		st.AdminUser = body.AdminUser
		st.AdminPassHash = passHash
		st.Nat.IPv4.PolicyRoutes = body.PolicyRoutes
		if body.SharedIPs != nil {
			st.Nat.IPv4.SharedIPs = body.SharedIPs
		}
		if body.Hostname != "" {
			st.System.Hostname = body.Hostname
		}
		st.Shaper.PolicyCIDR = body.PolicyRoutes[0]
		if body.DevLAN != "" {
			st.DHCP.Interface = body.DevLAN
			if body.EnableDHCP {
				st.DHCP.Enabled = true
				st.DHCP.DNSEnabled = true
			}
		}
	})
	if err := srv.store.Save(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	autoRec := srv.applyAutoTuningOnSetup(false)
	_ = srv.store.Save()

	srv.env.AdminUser = body.AdminUser
	srv.env.DevLAN = body.DevLAN
	srv.env.DevWAN = body.DevWAN
	envWrite := srv.env
	envWrite.AdminPass = ""
	if err := WriteRuntimeEnv(envWrite); err != nil {
		log.Printf("write env: %v", err)
	}
	if err := WriteDevRoles(body.DevLAN, body.DevWAN); err != nil {
		log.Printf("write dev roles: %v", err)
	}
	_ = ClearAdminPassFromEnv()
	srv.env.AdminPass = ""
	srv.reloadEnv()

	var applyErr string
	dataplaneOK := true
	if body.ApplyDataplane {
		_ = srv.applyAutoTuningOnSetup(true)
		_ = srv.setupPrepareTC()
		if err := srv.ApplyAll(); err != nil {
			applyErr = err.Error()
			dataplaneOK = false
			log.Printf("setup apply: %v", err)
		}
	}
	if dataplaneOK {
		_ = srv.store.Update(func(st *store.State) {
			st.SetupComplete = true
		})
		_ = srv.store.Save()
		_ = enableDataplaneOneshot()
	}

	tok, err := srv.sessions.create()
	if err == nil && dataplaneOK {
		srv.setSessionCookie(w, r, tok)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":             dataplaneOK,
		"setup_complete": dataplaneOK,
		"dev_lan":        body.DevLAN,
		"dev_wan":        body.DevWAN,
		"tuning_tier":    autoRec.Tier,
		"tuning_applied": true,
		"apply_error":    applyErr,
		"nft_skipped": func() bool {
			ips, _ := nft.ResolveSharedIPs(nft.Config{DevLAN: body.DevLAN, DevWAN: body.DevWAN}, srv.store.Get())
			return len(ips) == 0
		}(),
	})
}

func enableDataplaneOneshot() error {
	if os.Getuid() != 0 {
		return nil
	}
	_ = exec.Command("systemctl", "daemon-reload").Run()
	if out, err := exec.Command("systemctl", "enable", "qos-nat.service").CombinedOutput(); err != nil {
		log.Printf("enable qos-nat: %s %v", strings.TrimSpace(string(out)), err)
	}
	if out, err := exec.Command("systemctl", "start", "qos-nat.service").CombinedOutput(); err != nil {
		log.Printf("start qos-nat: %s %v", strings.TrimSpace(string(out)), err)
		return err
	}
	return nil
}

func (srv *Server) setupPrepareTC() error {
	if os.Getuid() != 0 || srv.env.DevLAN == "" {
		return nil
	}
	st := srv.store.Get()
	return shaper.SetupP0(shaper.Config{
		DevLAN:    srv.env.DevLAN,
		Leaf:      st.Shaper.Leaf,
		FQFlows:   st.Shaper.FQFlows,
		FQQuantum: st.Shaper.FQQuantum,
	})
}
