package api

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

var errReloadListener = errors.New("reload http listener")

// httpListener 管理 HTTP/HTTPS 监听；切换模式时优雅 Shutdown，不重启 qosnatd 进程。
type httpListener struct {
	mu         sync.Mutex
	srv        *Server
	httpSrv    *http.Server
	reloadReq  chan struct{}
	reloadDone chan error
	fatalErr   chan error
	started    bool
}

func (srv *Server) initHTTPListener() {
	if srv.httpListen != nil {
		return
	}
	srv.httpListen = &httpListener{
		srv:        srv,
		reloadReq:  make(chan struct{}, 1),
		reloadDone: make(chan error, 1),
		fatalErr:   make(chan error, 1),
	}
}

func (srv *Server) listenerSupervisor() {
	for {
		if err := srv.httpListen.runOnce(); err != nil {
			if errors.Is(err, errReloadListener) {
				continue
			}
			srv.httpListen.fatalErr <- err
			return
		}
		return
	}
}

func (hl *httpListener) runOnce() error {
	addr := hl.srv.listenAddr()
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	tlsOn := hl.srv.tlsActive()
	h := hl.srv.Handler()
	httpSrv := &http.Server{
		Handler:           h,
		ReadHeaderTimeout: 30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	if tlsOn {
		certPath := hl.srv.env.TLSCert
		keyPath := hl.srv.env.TLSKey
		if certPath == "" {
			certPath = defaultTLSCertPath
		}
		if keyPath == "" {
			keyPath = defaultTLSKeyPath
		}
		hl.srv.tlsReloader = newTLSCertReloader(certPath, keyPath)
		httpSrv.TLSConfig = hl.srv.tlsReloader.tlsConfig()
		log.Printf("qosnatd listening HTTPS on %s (LAN=%s WAN=%s, graceful TLS switch)", addr, hl.srv.env.DevLAN, hl.srv.env.DevWAN)
	} else {
		hl.srv.tlsReloader = nil
		log.Printf("qosnatd listening HTTP on %s (LAN=%s WAN=%s)", addr, hl.srv.env.DevLAN, hl.srv.env.DevWAN)
	}

	hl.mu.Lock()
	hl.httpSrv = httpSrv
	hl.mu.Unlock()

	serveDone := make(chan error, 1)
	go func() {
		if tlsOn {
			serveDone <- httpSrv.ServeTLS(ln, "", "")
		} else {
			serveDone <- httpSrv.Serve(ln)
		}
	}()

	select {
	case err := <-serveDone:
		hl.mu.Lock()
		hl.httpSrv = nil
		hl.mu.Unlock()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		select {
		case <-hl.reloadReq:
			return errReloadListener
		default:
			return err
		}
	case <-hl.reloadReq:
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		shutdownErr := httpSrv.Shutdown(ctx)
		cancel()
		serveErr := <-serveDone
		hl.mu.Lock()
		hl.httpSrv = nil
		hl.mu.Unlock()
		if shutdownErr != nil {
			hl.reloadDone <- shutdownErr
			return shutdownErr
		}
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			hl.reloadDone <- serveErr
			return serveErr
		}
		hl.reloadDone <- nil
		return errReloadListener
	}
}

// reloadHTTPListener 请求切换 HTTP/HTTPS 监听（等待当前连接优雅结束）
func (srv *Server) reloadHTTPListener() error {
	srv.initHTTPListener()
	if !srv.httpListen.started {
		return nil
	}
	select {
	case <-srv.httpListen.reloadDone:
	default:
	}
	srv.httpListen.reloadReq <- struct{}{}
	select {
	case err := <-srv.httpListen.reloadDone:
		return err
	case <-time.After(50 * time.Second):
		return errors.New("listener reload timeout")
	}
}
