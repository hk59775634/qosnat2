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
		hosts:    shaper.NewHostShaper(shaperDevLAN(env.DevLAN), st.Get().Shaper.Leaf),
	}
	s.mux = http.NewServeMux()
	s.routes()
	return s
}

func (srv *Server) routes() {
	m := srv.mux
	m.HandleFunc("/api/v1/health", srv.handleHealth)
	m.HandleFunc("/api/v1/setup/status", srv.handleSetupStatus)
	m.HandleFunc("/api/v1/setup/interfaces", srv.handleSetupInterfaces)
	m.HandleFunc("/api/v1/setup/complete", srv.handleSetupComplete)
	m.HandleFunc("/api/v1/login", srv.handleLogin)
	m.HandleFunc("/api/v1/session", srv.requireAuth(srv.handleSession))
	m.HandleFunc("/api/v1/logout", srv.requireAuth(srv.handleLogout))
	m.HandleFunc("/api/v1/api-keys", srv.requireAuth(srv.handleAPIKeys))

	m.HandleFunc("/api/v1/dhcp/apply", srv.requireAuth(srv.handleDHCPApply))
	m.HandleFunc("/api/v1/dhcp", srv.requireAuth(srv.handleDHCP))
	m.HandleFunc("/api/v1/routes/apply", srv.requireAuth(srv.handleRoutesApply))
	m.HandleFunc("/api/v1/routes/", srv.requireAuth(srv.handleRoutesItem))
	m.HandleFunc("/api/v1/routes", srv.requireAuth(srv.handleRoutes))
	m.HandleFunc("/api/v1/nat/policy-routes", srv.requireAuth(srv.handlePolicyRoutes))
	m.HandleFunc("/api/v1/nat/shared-ips", srv.requireAuth(srv.handleSharedIPs))
	m.HandleFunc("/api/v1/nat/static-mappings", srv.requireAuth(srv.handleStaticMappings))
	m.HandleFunc("/api/v1/nat/prefix-mappings", srv.requireAuth(srv.handlePrefixMappings))
	m.HandleFunc("/api/v1/nat/wan-forwards", srv.requireAuth(srv.handleWanForwards))

	m.HandleFunc("/api/v1/shaper/profiles/order", srv.requireAuth(srv.handleShaperProfilesOrder))
	m.HandleFunc("/api/v1/shaper/profiles", srv.requireAuth(srv.handleShaperProfiles))
	m.HandleFunc("/api/v1/shaper/wizard", srv.requireAuth(srv.handleShaperWizard))
	m.HandleFunc("/api/v1/shaper/active", srv.requireAuth(srv.handleShaperActive))

	m.HandleFunc("/api/v1/stats/dashboard", srv.requireAuth(srv.handleStatsDashboard))
	m.HandleFunc("/api/v1/ebpf/maps", srv.requireAuth(srv.handleEbpfMaps))
	m.HandleFunc("/api/v1/ebpf/programs", srv.requireAuth(srv.handleEbpfPrograms))
	m.HandleFunc("/api/v1/ebpf/reload", srv.requireAuth(srv.handleEbpfReload))
	m.HandleFunc("/api/v1/system/mark-policy", srv.requireAuth(srv.handleMarkPolicy))
	m.HandleFunc("/api/v1/system/tuning", srv.requireAuth(srv.handleSystemTuning))
	m.HandleFunc("/api/v1/interfaces/queues", srv.requireAuth(srv.handleIfaceQueues))
	m.HandleFunc("/api/v1/interfaces", srv.requireAuth(srv.handleInterfaces))

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

// ApplyAll 启动/回放：sysctl → tc → nft（未完成引导时跳过）
func (srv *Server) ApplyAll() error {
	if !srv.setupComplete() {
		log.Printf("apply skipped: initial setup not complete")
		return nil
	}
	if srv.env.DevLAN == "" || srv.env.DevWAN == "" {
		return fmt.Errorf("DEV_LAN and DEV_WAN must be set")
	}
	st := srv.store.Get()
	if err := srv.applySystemTuning(st); err != nil {
		log.Printf("system tuning: %v", err)
	}
	if err := shaper.SetupP0(shaper.Config{DevLAN: srv.env.DevLAN, Leaf: st.Shaper.Leaf}); err != nil {
		return fmt.Errorf("shaper: %w", err)
	}
	cfg := nft.Config{DevLAN: srv.env.DevLAN, DevWAN: srv.env.DevWAN}
	if ips, auto := nft.ResolveSharedIPs(cfg, st); len(ips) == 0 {
		log.Printf("warn: shared_ips empty and no IPv4 on WAN %s, nft SNAT uses masquerade only", srv.env.DevWAN)
	} else if auto {
		log.Printf("shared_ips: using WAN %s address %s", srv.env.DevWAN, ips[0])
	}
	if err := nft.Apply(cfg, st); err != nil {
		return fmt.Errorf("nft: %w", err)
	}
	srv.applyManagedRoutes()
	srv.applyManagedDHCP()
	srv.applyEBPF(st)
	return nil
}

// applyEBPF 在 TC 拓扑就绪后加载/附加 eBPF（引导完成或 apply-state 时调用）
func (srv *Server) applyEBPF(st store.State) {
	if srv.bpf == nil {
		return
	}
	if !srv.bpf.Ready() {
		if err := srv.bpf.Load(); err != nil {
			log.Printf("ebpf load: %v", err)
			return
		}
		log.Printf("ebpf loaded after TC/ifb0 ready")
	}
	if err := srv.bpf.ReplayState(st); err != nil {
		log.Printf("ebpf replay: %v", err)
	}
	srv.purgeLegacyHostExact(st)
	if err := srv.bpf.AttachTC(srv.env.DevLAN); err != nil {
		log.Printf("ebpf attach %s: %v", srv.env.DevLAN, err)
	} else {
		srv.syncShaperDevices()
		srv.replayProfileHosts()
		srv.StartBackground()
	}
	srv.setupWGShaper()
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
		KeepVIP: srv.gcKeepProfiles,
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
	complete := srv.setupComplete()
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":              true,
		"service":         "qosnatd",
		"phase":           phase,
		"setup_complete":  complete,
		"setup_required":  !complete,
		"dev_lan":         srv.env.DevLAN,
		"dev_wan":         srv.env.DevWAN,
		"bpf":             srv.bpf != nil && srv.bpf.Ready(),
		"tc_attach":       srv.bpf != nil && srv.bpf.AttachedDev() != "",
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
	if !srv.setupComplete() {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "complete initial setup first"})
		return
	}
	if !srv.verifyAdmin(body.User, body.Pass) {
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
	user := srv.env.AdminUser
	if st := srv.store.Get(); st.AdminUser != "" {
		user = st.AdminUser
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":             true,
		"user":           user,
		"setup_complete": srv.setupComplete(),
	})
}

func (srv *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		srv.sessions.delete(c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
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

func shaperDevLAN(dev string) string {
	if dev == "" {
		return "lo"
	}
	return dev
}
