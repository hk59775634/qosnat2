package api

import (
	"fmt"
	"sync"

	"github.com/hk59775634/qosnat2/internal/store"
)

var acmeHTTP01PortMu sync.Mutex

// withAcmeHTTP01Port80Open 串行化 ACME HTTP-01（防火墙开放在 acme.HTTP01PortHook 中按 target 解析 IP 执行）。
func (srv *Server) withAcmeHTTP01Port80Open(_ string, fn func() error) error {
	if fn == nil {
		return fmt.Errorf("nil fn")
	}
	acmeHTTP01PortMu.Lock()
	defer acmeHTTP01PortMu.Unlock()
	return fn()
}

func (srv *Server) setAcmeTempAllowHTTP01(v bool, ips []string) error {
	_ = srv.store.Update(func(s *store.State) {
		s.System.AcmeTempAllowHTTP01 = v
		if v {
			s.System.AcmeTempAllowHTTP01IPs = append([]string(nil), ips...)
		} else {
			s.System.AcmeTempAllowHTTP01IPs = nil
		}
	})
	if err := srv.store.Save(); err != nil {
		return err
	}
	return srv.reloadNft()
}
