package api

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
)

const (
	terminalMaxSessions = 2
	terminalReadLimit   = 64 * 1024
)

var terminalSlots = make(chan struct{}, terminalMaxSessions)

type terminalResizeMsg struct {
	Type string `json:"type"`
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
}

var terminalUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     terminalCheckOrigin,
}

func terminalCheckOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	host := r.Host
	if host == "" {
		return false
	}
	// Accept same host over http/https/ws/wss.
	for _, prefix := range []string{
		"https://" + host,
		"http://" + host,
		"wss://" + host,
		"ws://" + host,
	} {
		if origin == prefix {
			return true
		}
	}
	return false
}

func (srv *Server) handleTerminalWS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !srv.requestAuthorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	if !terminalClientAllowed(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "client IP not allowed for web terminal (QOSNAT_TERMINAL_ALLOW_CIDRS)"})
		return
	}
	if !srv.store.Get().System.DiagnosticsTerminalEnabled {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "web terminal disabled; enable in System → General"})
		return
	}
	tok := sessionTokenFromRequest(r)
	if tok == "" || !srv.terminalGrants.consume(tok) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "password verification required; confirm in terminal dialog"})
		return
	}
	select {
	case terminalSlots <- struct{}{}:
	default:
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "terminal sessions full"})
		return
	}
	defer func() { <-terminalSlots }()

	conn, err := terminalUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("terminal ws upgrade: %v", err)
		return
	}
	defer conn.Close()
	conn.SetReadLimit(terminalReadLimit)

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}
	cmd := exec.Command(shell)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color", "COLORTERM=truecolor")
	ptmx, err := pty.Start(cmd)
	if err != nil {
		log.Printf("terminal pty: %v", err)
		_ = conn.WriteMessage(websocket.TextMessage, []byte("\r\n[terminal] failed to start shell\r\n"))
		return
	}
	defer func() {
		_ = ptmx.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_, _ = cmd.Process.Wait()
	}()

	srv.auditLog(r, "diagnostics.terminal.open", r.RemoteAddr)

	var wg sync.WaitGroup
	done := make(chan struct{})
	var closeOnce sync.Once
	closeAll := func() { closeOnce.Do(func() { close(done) }) }

	wg.Add(1)
	go func() {
		defer wg.Done()
		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				if werr := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); werr != nil {
					closeAll()
					return
				}
			}
			if err != nil {
				closeAll()
				return
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer closeAll()
		for {
			mt, payload, err := conn.ReadMessage()
			if err != nil {
				return
			}
			switch mt {
			case websocket.TextMessage:
				if len(payload) > 0 && payload[0] == '{' {
					var msg terminalResizeMsg
					if json.Unmarshal(payload, &msg) == nil && msg.Type == "resize" && msg.Cols > 0 && msg.Rows > 0 {
						_ = pty.Setsize(ptmx, &pty.Winsize{Cols: msg.Cols, Rows: msg.Rows})
						continue
					}
				}
				if _, err := ptmx.Write(payload); err != nil {
					return
				}
			case websocket.BinaryMessage:
				if _, err := ptmx.Write(payload); err != nil {
					return
				}
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = cmd.Wait()
		closeAll()
	}()

	<-done
	conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	wg.Wait()
	srv.auditLog(r, "diagnostics.terminal.close", r.RemoteAddr)
}
