package api

import (
	"fmt"
	"log"
	"sync"

	"github.com/hk59775634/qosnat2/internal/store"
)

var acmeHTTP01PortMu sync.Mutex

// withAcmeHTTP01Port80Open 在执行 HTTP-01 验证期间临时放开 tcp/80 入站。
// 完成后会自动恢复（reload nftables），并尽量保证关闭动作不因 fn 返回错误而遗漏。
func (srv *Server) withAcmeHTTP01Port80Open(fn func() error) error {
	if fn == nil {
		return fmt.Errorf("nil fn")
	}

	acmeHTTP01PortMu.Lock()
	defer acmeHTTP01PortMu.Unlock()

	st := srv.store.Get()
	if st.System.AcmeTempAllowHTTP01 {
		// 已经打开，避免重复 reload。
		return fn()
	}

	if err := srv.setAcmeTempAllowHTTP01(true); err != nil {
		return err
	}
	var fnErr error
	defer func() {
		if err := srv.setAcmeTempAllowHTTP01(false); err != nil {
			log.Printf("acme-http01: restore allow tcp/80 failed: %v", err)
		}
	}()

	fnErr = fn()
	return fnErr
}

func (srv *Server) setAcmeTempAllowHTTP01(v bool) error {
	_ = srv.store.Update(func(s *store.State) {
		s.System.AcmeTempAllowHTTP01 = v
	})
	if err := srv.store.Save(); err != nil {
		log.Printf("save state: %v", err)
	}
	// nft 重建会 delete table inet qosnat，因此用 reload 保证规则变更立即生效。
	return srv.reloadNft()
}

