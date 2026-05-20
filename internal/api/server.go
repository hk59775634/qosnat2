package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hk59775634/qosnat2/internal/capture"
	"github.com/hk59775634/qosnat2/internal/ebpf"
	"github.com/hk59775634/qosnat2/internal/nft"
	"github.com/hk59775634/qosnat2/internal/shaper"
	"github.com/hk59775634/qosnat2/internal/store"
	"github.com/hk59775634/qosnat2/internal/stats"
	"github.com/hk59775634/qosnat2/internal/sysctl"
)

// Env 运行配置
type Env struct {
	AdminUser    string
	AdminPass    string
	AdminPort    string
	DevLAN       string
	DevWAN       string
	StateFile    string
	SessionFile  string
	OpenAPIPath  string
	WebRoot      string
}

// Server HTTP API + 控制面编排
type Server struct {
	env        Env
	store      *store.Store
	sessions   *sessionStore
	bpf        *ebpf.Manager
	hosts      *shaper.HostShaper
	metrics    *stats.Collector
	pcap       *capture.Manager
	ringCancel context.CancelFunc
	mux        *http.ServeMux
}

func New(env Env, st *store.Store, bpfM *ebpf.Manager) *Server {
	if env.AdminPort == "" {
		env.AdminPort = "8080"
	}
	if env.StateFile == "" {
		env.StateFile = "/var/lib/qosnat2/state.json"
	}
	if env.SessionFile == "" {
		env.SessionFile = "/var/lib/qosnat2/sessions.json"
	}
	s := &Server{
		env:      env,
		store:    st,
		bpf:      bpfM,
		sessions: newSessionStore(env.SessionFile),
		hosts:    shaper.NewHostShaper(env.DevLAN, st.Get().Shaper.Leaf),
	}
	s.mux = http.NewServeMux()
	s.routes()
	return s
}

func (srv *Server) routes() {
	m := srv.mux
	m.HandleFunc("/api/v1/health", srv.handleHealth)
	m.HandleFunc("/api/v1/login", srv.handleLogin)
	m.HandleFunc("/api/v1/session", srv.requireAuth(srv.handleSession))
	m.HandleFunc("/api/v1/logout", srv.requireAuth(srv.handleLogout))

	m.HandleFunc("/api/v1/nat/policy-routes", srv.requireAuth(srv.handlePolicyRoutes))
	m.HandleFunc("/api/v1/nat/shared-ips", srv.requireAuth(srv.handleSharedIPs))
	m.HandleFunc("/api/v1/nat/static-mappings", srv.requireAuth(srv.handleStaticMappings))
	m.HandleFunc("/api/v1/nat/prefix-mappings", srv.requireAuth(srv.handlePrefixMappings))
	m.HandleFunc("/api/v1/nat/wan-forwards", srv.requireAuth(srv.handleWanForwards))

	m.HandleFunc("/api/v1/shaper/profiles", srv.requireAuth(srv.handleShaperProfiles))
	m.HandleFunc("/api/v1/shaper/hosts", srv.requireAuth(srv.handleShaperHostsList))
	m.HandleFunc("/api/v1/shaper/hosts/", srv.requireAuth(srv.handleShaperHost))
	m.HandleFunc("/api/v1/shaper/wizard", srv.requireAuth(srv.handleShaperWizard))
	m.HandleFunc("/api/v1/shaper/active", srv.requireAuth(srv.handleShaperActive))

	m.HandleFunc("/api/v1/stats", srv.requireAuth(srv.handleStats))
	m.HandleFunc("/api/v1/stats/dashboard", srv.requireAuth(srv.handleStatsDashboard))
	m.HandleFunc("/api/v1/ebpf/maps", srv.requireAuth(srv.handleEbpfMaps))
	m.HandleFunc("/api/v1/ebpf/programs", srv.requireAuth(srv.handleEbpfPrograms))
	m.HandleFunc("/api/v1/ebpf/reload", srv.requireAuth(srv.handleEbpfReload))
	m.HandleFunc("/api/v1/system/mark-policy", srv.requireAuth(srv.handleMarkPolicy))
	m.HandleFunc("/api/v1/interfaces/queues", srv.requireAuth(srv.handleIfaceQueues))

	m.HandleFunc("/api/v1/vpn/wireguard/keys", srv.requireAuth(srv.handleWireGuardKeys))
	m.HandleFunc("/api/v1/vpn/wireguard/apply", srv.requireAuth(srv.handleWireGuardApply))
	m.HandleFunc("/api/v1/vpn/wireguard/peers/", srv.requireAuth(srv.handleWireGuardPeers))
	m.HandleFunc("/api/v1/vpn/wireguard/peers", srv.requireAuth(srv.handleWireGuardPeers))
	m.HandleFunc("/api/v1/vpn/wireguard", srv.requireAuth(srv.handleWireGuard))

	m.HandleFunc("/api/v1/diagnostics/captures/", srv.requireAuth(srv.handleCaptures))
	m.HandleFunc("/api/v1/diagnostics/captures", srv.requireAuth(srv.handleCaptures))
	m.HandleFunc("/api/v1/diagnostics/conntrack", srv.requireAuth(srv.handleConntrack))

	m.HandleFunc("/openapi.yaml", srv.serveOpenAPI)
	m.HandleFunc("/", srv.serveStatic)
}

func (srv *Server) Handler() http.Handler { return srv.mux }

// ApplyAll 启动/回放：sysctl → tc → nft
func (srv *Server) ApplyAll() error {
	st := srv.store.Get()
	sysctl.ApplyFast(st.System.Sysctl)
	if err := sysctl.Apply(st.System.Sysctl); err != nil {
		log.Printf("sysctl: %v", err)
	}
	if err := shaper.SetupP0(shaper.Config{DevLAN: srv.env.DevLAN, Leaf: st.Shaper.Leaf}); err != nil {
		return fmt.Errorf("shaper: %w", err)
	}
	cfg := nft.Config{DevLAN: srv.env.DevLAN, DevWAN: srv.env.DevWAN}
	if len(st.SharedIPs) == 0 {
		log.Printf("warn: shared_ips empty, skip nft until configured via API")
		return nil
	}
	if err := nft.Apply(cfg, st); err != nil {
		return fmt.Errorf("nft: %w", err)
	}
	if srv.bpf != nil && srv.bpf.Ready() {
		if err := srv.bpf.ReplayState(st); err != nil {
			log.Printf("ebpf replay: %v", err)
		}
		if err := srv.bpf.AttachTC(srv.env.DevLAN); err != nil {
			log.Printf("ebpf attach: %v", err)
		} else {
			ebpf.ReplayHostClasses(st, srv.hosts)
		}
		srv.setupWGShaper()
	}
	return nil
}

// StartBackground ringbuf + 空闲 GC
func (srv *Server) StartBackground() {
	if srv.bpf == nil || !srv.bpf.Ready() || srv.ringCancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	srv.ringCancel = cancel
	if err := srv.bpf.StartRingbuf(ctx, srv.hosts); err != nil {
		log.Printf("ringbuf: %v", err)
		cancel()
		srv.ringCancel = nil
		return
	}
	gc := &shaper.GCRunner{
		Hosts:   srv.hosts,
		BPF:     srv.bpf,
		Timeout: srv.idleTimeout(),
		KeepVIP: srv.gcKeepVIP,
	}
	interval := srv.idleTimeout() / 2
	if interval < time.Minute {
		interval = time.Minute
	}
	go shaper.StartLoop(ctx.Done(), interval, gc)
}

func (srv *Server) reloadNft() error {
	return nft.Apply(nft.Config{DevLAN: srv.env.DevLAN, DevWAN: srv.env.DevWAN}, srv.store.Get())
}

func (srv *Server) nftCfg() nft.Config {
	return nft.Config{DevLAN: srv.env.DevLAN, DevWAN: srv.env.DevWAN}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func readJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func (srv *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	phase := "P0"
	if srv.bpf != nil && srv.bpf.Ready() {
		phase = "P1"
		if srv.bpf.AttachedDev() != "" {
			phase = "P6"
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"service":  "qosnatd",
		"phase":    phase,
		"dev_lan":  srv.env.DevLAN,
		"dev_wan":  srv.env.DevWAN,
		"bpf":      srv.bpf != nil && srv.bpf.Ready(),
		"tc_attach": srv.bpf != nil && srv.bpf.AttachedDev() != "",
	})
}

func (srv *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		User string `json:"user"`
		Pass string `json:"pass"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
		return
	}
	if body.User != srv.env.AdminUser || body.Pass != srv.env.AdminPass {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}
	tok, err := srv.sessions.create()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    tok,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionTTL.Seconds()),
	})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (srv *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "user": srv.env.AdminUser})
}

func (srv *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		srv.sessions.delete(c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (srv *Server) handlePolicyRoutes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, srv.store.Get().PolicyRoutes)
	case http.MethodPost:
		var body struct {
			CIDR string `json:"cidr"`
		}
		if err := readJSON(r, &body); err != nil || body.CIDR == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cidr required"})
			return
		}
		if err := srv.store.Update(func(st *store.State) {
			for _, c := range st.PolicyRoutes {
				if c == body.CIDR {
					return
				}
			}
			st.PolicyRoutes = append(st.PolicyRoutes, body.CIDR)
		}); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		_ = srv.store.Save()
		if err := srv.reloadNft(); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case http.MethodDelete:
		cidr := r.URL.Query().Get("cidr")
		if cidr == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cidr query required"})
			return
		}
		if err := srv.store.Update(func(st *store.State) {
			var out []string
			for _, c := range st.PolicyRoutes {
				if c != cidr {
					out = append(out, c)
				}
			}
			st.PolicyRoutes = out
		}); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		_ = srv.store.Save()
		_ = srv.reloadNft()
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (srv *Server) handleSharedIPs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, srv.store.Get().SharedIPs)
	case http.MethodPost:
		var body struct {
			IP string `json:"ip"`
		}
		if err := readJSON(r, &body); err != nil || body.IP == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ip required"})
			return
		}
		if err := srv.store.Update(func(st *store.State) {
			_ = nft.AddSharedIP(st, body.IP)
		}); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		_ = srv.store.Save()
		if err := srv.reloadNft(); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (srv *Server) handleStaticMappings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, srv.store.Get().StaticMappings)
	case http.MethodPost:
		var body struct {
			Inner string `json:"inner"`
			Outer string `json:"outer"`
		}
		if err := readJSON(r, &body); err != nil || body.Inner == "" || body.Outer == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "inner/outer required"})
			return
		}
		_ = srv.store.Update(func(st *store.State) {
			st.StaticMappings[body.Inner] = body.Outer
		})
		_ = srv.store.Save()
		_ = srv.reloadNft()
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (srv *Server) handlePrefixMappings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, srv.store.Get().PrefixMappings)
	case http.MethodPost:
		var body struct {
			Inner  string `json:"inner"`
			Outer  string `json:"outer"`
		}
		if err := readJSON(r, &body); err != nil || body.Inner == "" || body.Outer == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "inner/outer required"})
			return
		}
		_ = srv.store.Update(func(st *store.State) {
			st.PrefixMappings[body.Inner] = body.Outer
		})
		_ = srv.store.Save()
		_ = srv.reloadNft()
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (srv *Server) handleWanForwards(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, srv.store.Get().Firewall.WanPortForwards)
	case http.MethodPost:
		var f store.WanPortForward
		if err := readJSON(r, &f); err != nil || f.WanPort == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid forward"})
			return
		}
		if f.Proto == "" {
			f.Proto = "tcp"
		}
		_ = srv.store.Update(func(st *store.State) {
			st.Firewall.WanPortForwards = append(st.Firewall.WanPortForwards, f)
		})
		_ = srv.store.Save()
		_ = srv.reloadNft()
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (srv *Server) handleShaperProfiles(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		list, err := srv.bpf.ListProfiles()
		if err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, list)
	case http.MethodPut:
		var body struct {
			CIDR string `json:"cidr"`
			Down string `json:"down"`
			Up   string `json:"up"`
		}
		if err := readJSON(r, &body); err != nil || body.CIDR == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cidr/down/up required"})
			return
		}
		rv, err := srv.rateVal(body.Down, body.Up)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if err := srv.bpf.UpdateProfile(body.CIDR, rv); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		_ = srv.store.Update(func(st *store.State) {
			found := false
			for i, p := range st.Shaper.Profiles {
				if p.CIDR == body.CIDR {
					st.Shaper.Profiles[i].Down = body.Down
					st.Shaper.Profiles[i].Up = body.Up
					found = true
					break
				}
			}
			if !found {
				st.Shaper.Profiles = append(st.Shaper.Profiles, store.ProfileEntry{
					CIDR: body.CIDR, Down: body.Down, Up: body.Up,
				})
			}
		})
		_ = srv.store.Save()
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case http.MethodDelete:
		cidr := r.URL.Query().Get("cidr")
		if cidr == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cidr query required"})
			return
		}
		if err := srv.bpf.DeleteProfile(cidr); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		_ = srv.store.Update(func(st *store.State) {
			var out []store.ProfileEntry
			for _, p := range st.Shaper.Profiles {
				if p.CIDR != cidr {
					out = append(out, p)
				}
			}
			st.Shaper.Profiles = out
		})
		_ = srv.store.Save()
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (srv *Server) handleShaperHost(w http.ResponseWriter, r *http.Request) {
	ip := strings.TrimPrefix(r.URL.Path, "/api/v1/shaper/hosts/")
	if ip == "" || strings.Contains(ip, "/") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ip required in path"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		st := srv.store.Get()
		h, ok := st.Shaper.Hosts[ip]
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		writeJSON(w, http.StatusOK, h)
	case http.MethodPut:
		var body store.HostRate
		if err := readJSON(r, &body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
			return
		}
		rv, err := srv.rateVal(body.Down, body.Up)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if err := srv.bpf.UpdateHost(ip, rv); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if err := srv.hosts.EnsureHost(ip, rv.DownBPS, rv.UpBPS, rv.ClassMinor); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		_ = srv.store.Update(func(st *store.State) {
			st.Shaper.Hosts[ip] = body
		})
		_ = srv.store.Save()
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case http.MethodDelete:
		if err := srv.bpf.DeleteHost(ip); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		_ = srv.hosts.DeleteHost(ip)
		_ = srv.store.Update(func(st *store.State) {
			delete(st.Shaper.Hosts, ip)
		})
		_ = srv.store.Save()
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (srv *Server) handleShaperWizard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		CIDR string `json:"cidr"`
		Down string `json:"down"`
		Up   string `json:"up"`
		Mask int    `json:"mask"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
		return
	}
	if body.CIDR == "" {
		body.CIDR = "10.0.0.0/8"
	}
	if body.Down == "" {
		body.Down = "8mbit"
	}
	if body.Up == "" {
		body.Up = "8mbit"
	}
	if body.Mask == 0 {
		body.Mask = 32
	}
	rv, err := srv.rateVal(body.Down, body.Up)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := srv.bpf.UpdateProfile(body.CIDR, rv); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	added := false
	_ = srv.store.Update(func(st *store.State) {
		found := false
		for i, p := range st.Shaper.Profiles {
			if p.CIDR == body.CIDR {
				st.Shaper.Profiles[i].Down = body.Down
				st.Shaper.Profiles[i].Up = body.Up
				st.Shaper.Profiles[i].Mask = body.Mask
				found = true
				break
			}
		}
		if !found {
			st.Shaper.Profiles = append(st.Shaper.Profiles, store.ProfileEntry{
				CIDR: body.CIDR, Down: body.Down, Up: body.Up, Mask: body.Mask,
			})
			added = true
		}
		// 仅首次（无策略网段时）写入默认 policy，避免后续向导覆盖
		if st.Shaper.PolicyCIDR == "" {
			st.Shaper.PolicyCIDR = body.CIDR
			st.Shaper.DefaultProfile = store.RateProfile{Down: body.Down, Up: body.Up, HostMask: body.Mask}
		}
		hasRoute := false
		for _, c := range st.PolicyRoutes {
			if c == body.CIDR {
				hasRoute = true
				break
			}
		}
		if !hasRoute {
			st.PolicyRoutes = append(st.PolicyRoutes, body.CIDR)
		}
	})
	_ = srv.store.Save()
	if err := srv.reloadNft(); err != nil {
		log.Printf("wizard nft: %v", err)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "added": added, "cidr": body.CIDR})
}

func (srv *Server) rateVal(down, up string) (ebpf.RateVal, error) {
	d, err := store.MbitToBPS(down)
	if err != nil {
		return ebpf.RateVal{}, err
	}
	u, err := store.MbitToBPS(up)
	if err != nil {
		return ebpf.RateVal{}, err
	}
	return ebpf.RateVal{DownBPS: d, UpBPS: u}, nil
}

func (srv *Server) handleShaperActive(w http.ResponseWriter, r *http.Request) {
	list, err := srv.bpf.ListActive()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}
	if list == nil {
		list = []ebpf.ActiveEntry{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (srv *Server) handleEbpfMaps(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, srv.bpf.Status())
}

func (srv *Server) serveOpenAPI(w http.ResponseWriter, r *http.Request) {
	path := srv.env.OpenAPIPath
	if path == "" {
		path = "api/openapi.yaml"
	}
	b, err := os.ReadFile(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/yaml")
	w.Write(b)
}

func (srv *Server) webRoot() string {
	root := srv.env.WebRoot
	if root == "" {
		root = "web"
	}
	if dist := filepath.Join(root, "dist"); fileExists(filepath.Join(dist, "index.html")) {
		return dist
	}
	return root
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func (srv *Server) serveStatic(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		http.NotFound(w, r)
		return
	}
	root := filepath.Clean(srv.webRoot())
	path := r.URL.Path
	if path == "/" {
		path = "/index.html"
	}
	full := filepath.Join(root, filepath.Clean(path))
	if !strings.HasPrefix(full, root+string(os.PathSeparator)) && full != root {
		http.NotFound(w, r)
		return
	}
	if fileExists(full) && !isDir(full) {
		http.ServeFile(w, r, full)
		return
	}
	// Vue SPA fallback (hash router 亦兼容直接访问 /)
	index := filepath.Join(root, "index.html")
	if fileExists(index) {
		http.ServeFile(w, r, index)
		return
	}
	http.NotFound(w, r)
}

func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// Listen 启动 HTTP
func (srv *Server) Listen() error {
	_ = srv.sessions.load()
	addr := ":" + srv.env.AdminPort
	log.Printf("qosnatd listening on %s (LAN=%s WAN=%s)", addr, srv.env.DevLAN, srv.env.DevWAN)
	return http.ListenAndServe(addr, srv.Handler())
}

// PersistState 保存 state
func (srv *Server) PersistState() error {
	return srv.store.Save()
}

// InitFromEnvFile 加载 /etc/qosnat2/env
func InitFromEnvFile(path string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return
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
		k := strings.TrimSpace(line[:i])
		v := strings.TrimSpace(line[i+1:])
		if os.Getenv(k) == "" {
			os.Setenv(k, v)
		}
	}
}

func EnvOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func LoadEnv() Env {
	InitFromEnvFile("/etc/qosnat2/env")
	return Env{
		AdminUser:   EnvOr("ADMIN_USER", "admin"),
		AdminPass:   EnvOr("ADMIN_PASS", "QosNat@2026"),
		AdminPort:   EnvOr("ADMIN_PORT", "8080"),
		DevLAN:      EnvOr("DEV_LAN", ""),
		DevWAN:      EnvOr("DEV_WAN", ""),
		StateFile:   EnvOr("STATE_FILE", "/var/lib/qosnat2/state.json"),
		SessionFile: EnvOr("SESSION_FILE", "/var/lib/qosnat2/sessions.json"),
		OpenAPIPath: EnvOr("OPENAPI_PATH", "/opt/qosnat2/api/openapi.yaml"),
		WebRoot:     EnvOr("WEB_ROOT", "/opt/qosnat2/web"),
	}
}

func ValidateEnv(e Env) error {
	if e.DevLAN == "" || e.DevWAN == "" {
		return fmt.Errorf("DEV_LAN and DEV_WAN must be set (no default ens18/ens19)")
	}
	return nil
}
