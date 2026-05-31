package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hk59775634/qosnat2/internal/acme"
	"github.com/hk59775634/qosnat2/internal/capture"
	"github.com/hk59775634/qosnat2/internal/ebpf"
	"github.com/hk59775634/qosnat2/internal/ocserv/usertraffic"
	"github.com/hk59775634/qosnat2/internal/releasecatalog"
	"github.com/hk59775634/qosnat2/internal/shaper"
	"github.com/hk59775634/qosnat2/internal/stats"
	"github.com/hk59775634/qosnat2/internal/store"
	"github.com/hk59775634/qosnat2/internal/webassets"
	wgusertraffic "github.com/hk59775634/qosnat2/internal/wg/usertraffic"
)

// Env 运行配置
type Env struct {
	AdminUser   string
	AdminPass   string
	AdminPort   string
	DevLAN      string
	DevWAN      string
	StateFile   string
	SessionFile string
	OpenAPIPath string
	WebRoot     string
	TLSCert     string
	TLSKey      string
}

// Server HTTP API + 控制面编排
type Server struct {
	env                  Env
	store                *store.Store
	sessions             *sessionStore
	bpf                  *ebpf.Manager
	hosts                *shaper.HostShaper
	metrics              *stats.Collector
	pcap                 *capture.Manager
	ringCancel           context.CancelFunc
	metricsCancel        context.CancelFunc
	ocservTrafficCancel  context.CancelFunc
	wgTrafficCancel      context.CancelFunc
	mux                  *http.ServeMux
	loginLim             *loginLimiter
	versionSwitchGrants  *versionSwitchGrants
	tlsReloader          *tlsCertReloader
	httpListen           *httpListener
	ocservRestartHints   []string
	ocservRestartHintsMu sync.Mutex
	nftApplyMu           sync.Mutex
	natStackStatusMu     sync.RWMutex
	natStackStatus       map[string]any
	lastNatStackMu       sync.Mutex
	lastNatStackOK       bool
	lastNatStackNat      store.NatState
	lastNatStackDHCP     store.DHCPState
	dataplaneMetrics     dataplaneMetrics
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
		env:                 env,
		store:               st,
		bpf:                 bpfM,
		sessions:            newSessionStore(env.SessionFile),
		hosts:               shaper.NewHostShaper(shaperDevLAN(env.DevLAN), st.Get().Shaper.Leaf),
		loginLim:            newLoginLimiter(),
		versionSwitchGrants: newVersionSwitchGrants(),
	}
	s.mux = http.NewServeMux()
	s.routes()
	s.registerAcmeHooks()
	return s
}

func (srv *Server) registerAcmeHooks() {
	acme.SetHTTP01PortHook(func(open bool) error {
		if srv.env.DevWAN == "" {
			return nil
		}
		return srv.setAcmeTempAllowHTTP01(open)
	})
}

func (srv *Server) routes() {
	m := srv.mux
	m.HandleFunc("/api/v1/health", srv.handleHealth)
	m.HandleFunc("/api/v1/setup/status", srv.requireAuth(srv.handleSetupStatus))
	m.HandleFunc("/api/v1/setup/interfaces", srv.requireAuth(srv.handleSetupInterfaces))
	m.HandleFunc("/api/v1/setup/complete", srv.requireAuth(srv.handleSetupComplete))
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
	m.HandleFunc("/api/v1/nat", srv.requireAuth(srv.handleNatSummary))
	m.HandleFunc("/api/v1/nat/nptv6", srv.requireAuth(srv.handleNptv6))
	m.HandleFunc("/api/v1/nat/nat64", srv.requireAuth(srv.handleNat64))
	m.HandleFunc("/api/v1/nat/dns64", srv.requireAuth(srv.handleDNS64))

	m.HandleFunc("/api/v1/shaper/profiles/order", srv.requireAuth(srv.handleShaperProfilesOrder))
	m.HandleFunc("/api/v1/shaper/profiles", srv.requireAuth(srv.handleShaperProfiles))
	m.HandleFunc("/api/v1/shaper/tenants", srv.requireAuth(srv.handleShaperTenants))
	m.HandleFunc("/api/v1/shaper/wizard", srv.requireAuth(srv.handleShaperWizard))
	m.HandleFunc("/api/v1/shaper/active", srv.requireAuth(srv.handleShaperActive))

	m.HandleFunc("/api/v1/stats/dashboard", srv.requireAuth(srv.handleStatsDashboard))
	m.HandleFunc("/api/v1/metrics/ops", srv.requireAuth(srv.handleMetricsOps))
	m.HandleFunc("/api/v1/metrics/prometheus", srv.requireAuth(srv.handleMetricsPrometheus))
	m.HandleFunc("/api/v1/ebpf/maps", srv.requireAuth(srv.handleEbpfMaps))
	m.HandleFunc("/api/v1/ebpf/programs", srv.requireAuth(srv.handleEbpfPrograms))
	m.HandleFunc("/api/v1/ebpf/reload", srv.requireAuth(srv.handleEbpfReload))
	m.HandleFunc("/api/v1/system/mark-policy", srv.requireAuth(srv.handleMarkPolicy))
	m.HandleFunc("/api/v1/system/tuning", srv.requireAuth(srv.handleSystemTuning))
	m.HandleFunc("/api/v1/system/general", srv.requireAuth(srv.handleSystemGeneral))
	m.HandleFunc("/api/v1/system/state/export", srv.requireAuth(srv.handleSystemStateExport))
	m.HandleFunc("/api/v1/system/state/import", srv.requireAuth(srv.handleSystemStateImport))
	m.HandleFunc("/api/v1/system/state/import/raw", srv.requireAuth(srv.handleSystemStateImportRaw))
	m.HandleFunc("/api/v1/system/version/switch/verify", srv.requireAuth(srv.handleSystemVersionSwitchVerify))
	m.HandleFunc("/api/v1/system/version/switch/status", srv.requireAuth(srv.handleSystemVersionSwitchStatus))
	m.HandleFunc("/api/v1/system/version/switch/reset", srv.requireAuth(srv.handleSystemVersionSwitchReset))
	m.HandleFunc("/api/v1/system/version/switch", srv.requireAuth(srv.handleSystemVersionSwitch))
	m.HandleFunc("/api/v1/system/version", srv.requireAuth(srv.handleSystemVersion))
	m.HandleFunc("/api/v1/system/tls/acme", srv.requireAuth(srv.handleTLSAcme))
	m.HandleFunc("/api/v1/system/notifications", srv.requireAuth(srv.handleNotifications))
	m.HandleFunc("/api/v1/system/certificates/auto-renew", srv.requireAuth(srv.handleCertificateAutoRenew))
	m.HandleFunc("/api/v1/system/certificates/renew", srv.requireAuth(srv.handleCertificateRenew))
	m.HandleFunc("/api/v1/system/certificates", srv.requireAuth(srv.handleCertificates))
	m.HandleFunc("/api/v1/system/audit", srv.requireAuth(srv.handleSystemAudit))
	m.HandleFunc("/api/v1/firewall/apply", srv.requireAuth(srv.handleFirewallApply))
	m.HandleFunc("/api/v1/firewall/discard", srv.requireAuth(srv.handleFirewallDiscard))
	m.HandleFunc("/api/v1/firewall/rules/order", srv.requireAuth(srv.handleFirewallRulesOrder))
	m.HandleFunc("/api/v1/firewall/rules", srv.requireAuth(srv.handleFirewallRules))
	m.HandleFunc("/api/v1/interfaces/queues", srv.requireAuth(srv.handleIfaceQueues))
	m.HandleFunc("/api/v1/interfaces/ethtool", srv.requireAuth(srv.handleInterfacesEthtool))
	m.HandleFunc("/api/v1/interfaces/roles", srv.requireAuth(srv.handleInterfacesRoles))
	m.HandleFunc("/api/v1/interfaces", srv.requireAuth(srv.handleInterfaces))
	m.HandleFunc("/api/v1/firewall/aliases", srv.requireAuth(srv.handleFirewallAliases))
	m.HandleFunc("/api/v1/network/netplan/apply", srv.requireAuth(srv.handleNetworkNetplanApply))
	m.HandleFunc("/api/v1/network/netplan", srv.requireAuth(srv.handleNetworkNetplan))
	m.HandleFunc("/api/v1/network/vxlan", srv.requireAuth(srv.handleNetworkVXLAN))
	m.HandleFunc("/api/v1/network/vlans", srv.requireAuth(srv.handleNetworkVLANs))
	m.HandleFunc("/api/v1/network/wan-links", srv.requireAuth(srv.handleNetworkWanLinks))
	m.HandleFunc("/api/v1/network/egress-policies", srv.requireAuth(srv.handleNetworkEgressPolicies))
	m.HandleFunc("/api/v1/network/egress-policies/bulk", srv.requireAuth(srv.handleNetworkEgressPoliciesBulk))
	m.HandleFunc("/api/v1/network/warp/status", srv.requireAuth(srv.handleNetworkWarpStatus))
	m.HandleFunc("/api/v1/network/warp/install", srv.requireAuth(srv.handleNetworkWarpInstall))
	m.HandleFunc("/api/v1/network/warp/install/status", srv.requireAuth(srv.handleNetworkWarpInstallStatus))
	m.HandleFunc("/api/v1/network/warp/connect", srv.requireAuth(srv.handleNetworkWarpConnect))
	m.HandleFunc("/api/v1/network/warp/disconnect", srv.requireAuth(srv.handleNetworkWarpDisconnect))
	m.HandleFunc("/api/v1/network/warp/task/status", srv.requireAuth(srv.handleNetworkWarpTaskStatus))
	m.HandleFunc("/api/v1/shaper/tc", srv.requireAuth(srv.handleShaperTC))

	m.HandleFunc("/api/v1/vpn/wireguard/instances/", srv.requireAuth(srv.handleWireGuardInstancesSubtree))
	m.HandleFunc("/api/v1/vpn/wireguard/instances", srv.requireAuth(srv.handleWireGuardInstancesRoot))

	m.HandleFunc("/api/v1/vpn/ocserv/install/status", srv.requireAuth(srv.handleOCServInstallStatus))
	m.HandleFunc("/api/v1/vpn/ocserv/install", srv.requireAuth(srv.handleOCServInstall))
	m.HandleFunc("/api/v1/vpn/ocserv/uninstall", srv.requireAuth(srv.handleOCServUninstall))
	m.HandleFunc("/api/v1/vpn/ocserv/service", srv.requireAuth(srv.handleOCServService))
	m.HandleFunc("/api/v1/vpn/ocserv/apply", srv.requireAuth(srv.handleOCServApply))
	m.HandleFunc("/api/v1/vpn/ocserv/status/detail", srv.requireAuth(srv.handleOCServStatusDetail))
	m.HandleFunc("/api/v1/vpn/ocserv/sessions/disconnect", srv.requireAuth(srv.handleOCServSessionsDisconnect))
	m.HandleFunc("/api/v1/vpn/ocserv/sessions", srv.requireAuth(srv.handleOCServSessions))
	m.HandleFunc("/api/v1/vpn/ocserv/users/traffic", srv.requireAuth(srv.handleOCServUserTraffic))
	m.HandleFunc("/api/v1/vpn/ocserv/users", srv.requireAuth(srv.handleOCServUsers))
	m.HandleFunc("/api/v1/vpn/ocserv/groups", srv.requireAuth(srv.handleOCServGroups))
	m.HandleFunc("/api/v1/vpn/ocserv/vhosts/users", srv.requireAuth(srv.handleOCServVhostUsers))
	m.HandleFunc("/api/v1/vpn/ocserv/vhosts", srv.requireAuth(srv.handleOCServVhosts))
	m.HandleFunc("/api/v1/vpn/ocserv", srv.requireAuth(srv.handleOCServ))

	m.HandleFunc("/api/v1/diagnostics/captures/", srv.requireAuth(srv.handleCaptures))
	m.HandleFunc("/api/v1/diagnostics/captures", srv.requireAuth(srv.handleCaptures))
	m.HandleFunc("/api/v1/diagnostics/conntrack", srv.requireAuth(srv.handleConntrack))
	m.HandleFunc("/api/v1/diagnostics/terminal", srv.handleTerminalWS)

	m.HandleFunc("/openapi.yaml", srv.serveOpenAPI)
	m.HandleFunc("/", srv.serveStatic)
}

func (srv *Server) Handler() http.Handler {
	return srv.withSecurityHeaders(srv.mux)
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

// readJSONRelaxed 忽略未知 JSON 字段（用于 WireGuard 等 GET→PUT 回环：响应含只读展示字段）
func readJSONRelaxed(r *http.Request, dst any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	return dec.Decode(dst)
}

func (srv *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
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
	st := srv.store.Get()
	tlsActive := srv.env.TLSCert != "" && srv.env.TLSKey != ""
	resp := map[string]any{
		"ok":             true,
		"service":        "qosnatd",
		"display_name":   store.EffectiveDisplayName(st.System.DisplayName),
		"release_tag":    releasecatalog.NormalizeID(readTextFile(qosnatReleaseTag)),
		"build_version":  detectQosnatVersion(),
		"phase":          phase,
		"setup_complete": complete,
		"setup_required": !complete,
		"bpf":            srv.bpf != nil && srv.bpf.Ready(),
		"tc_attach":      srv.bpf != nil && srv.bpf.AttachedDev() != "",
		"tls_enabled":    st.System.TLSEnabled,
		"tls_active":     tlsActive,
		"suggest_https":  st.System.TLSEnabled && !tlsActive,
	}
	if complete {
		resp["dev_lan"] = srv.env.DevLAN
		resp["dev_wan"] = srv.env.DevWAN
		resp["admin_port"] = srv.env.AdminPort
	}
	resp["diagnostics_terminal_enabled"] = st.System.DiagnosticsTerminalEnabled
	writeJSON(w, http.StatusOK, resp)
}

func (srv *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	var body struct {
		User string `json:"user"`
		Pass string `json:"pass"`
	}
	if err := readJSON(r, &body); err != nil {
		writeBadJSON(w)
		return
	}
	ip := clientIP(r)
	if !srv.loginLim.allow(ip) {
		writeRateLimited(w, "too many login attempts")
		return
	}
	if !srv.setupComplete() {
		st := srv.store.Get()
		if st.AdminPassHash != "" {
			writeForbidden(w, "", "initial setup already in progress; use configured admin account")
			return
		}
		if srv.env.AdminPass == "" {
			writeForbidden(w, "", "no initial admin password; reinstall or read /etc/qosnat2/initial-admin.txt")
			return
		}
	}
	if !srv.verifyAdmin(body.User, body.Pass) {
		srv.loginLim.recordFail(ip)
		writeUnauthorized(w, "invalid credentials")
		return
	}
	srv.loginLim.clear(ip)
	tok, err := srv.sessions.create()
	if err != nil {
		writeInternalError(w, "session error")
		return
	}
	srv.setSessionCookie(w, r, tok)
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
	if srv.setupComplete() {
		if !srv.checkAPIKey(r) {
			if c, err := r.Cookie(sessionCookie); err != nil || !srv.sessions.valid(c.Value) {
				writeUnauthorized(w, "unauthorized")
				return
			}
		}
	}
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
	if webassets.Enabled() && webassets.HasIndex() {
		sub, err := webassets.SubStatic()
		if err == nil {
			webassets.SPAHandler(sub).ServeHTTP(w, r)
			return
		}
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

// Listen 启动 HTTP/HTTPS（支持运行中切换模式，无需重启进程）
func (srv *Server) Listen() error {
	_ = srv.sessions.load()
	srv.startMetricsSampler()
	srv.startOCServTrafficSampler()
	srv.startWireGuardTrafficSampler()
	srv.initHTTPListener()
	srv.httpListen.started = true
	go srv.listenerSupervisor()
	return <-srv.httpListen.fatalErr
}

func (srv *Server) startOCServTrafficSampler() {
	if srv.ocservTrafficCancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	srv.ocservTrafficCancel = cancel
	go usertraffic.StartSampler(ctx, func() store.OCServState {
		return srv.store.Get().VPN.OCServ
	})
}

func (srv *Server) startWireGuardTrafficSampler() {
	if srv.wgTrafficCancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	srv.wgTrafficCancel = cancel
	go wgusertraffic.StartSampler(ctx, func() []store.WireGuardInstance {
		return srv.store.Get().VPN.WireGuards
	})
}

func (srv *Server) startMetricsSampler() {
	if srv.metricsCancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	srv.metricsCancel = cancel
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		c := srv.collector()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.SampleAllInterfaceRates()
				if srv.env.DevLAN != "" || srv.env.DevWAN != "" {
					c.RecordTraffic(srv.env.DevLAN, srv.env.DevWAN)
				}
			}
		}
	}()
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
		AdminPass:   EnvOr("ADMIN_PASS", ""),
		AdminPort:   EnvOr("ADMIN_PORT", "8080"),
		DevLAN:      EnvOr("DEV_LAN", ""),
		DevWAN:      EnvOr("DEV_WAN", ""),
		StateFile:   EnvOr("STATE_FILE", "/var/lib/qosnat2/state.json"),
		SessionFile: EnvOr("SESSION_FILE", "/var/lib/qosnat2/sessions.json"),
		OpenAPIPath: EnvOr("OPENAPI_PATH", "/opt/qosnat2/api/openapi.yaml"),
		WebRoot:     EnvOr("WEB_ROOT", "/opt/qosnat2/web"),
		TLSCert:     EnvOr("TLS_CERT", ""),
		TLSKey:      EnvOr("TLS_KEY", ""),
	}
}

func shaperDevLAN(dev string) string {
	if dev == "" {
		return "lo"
	}
	return dev
}
