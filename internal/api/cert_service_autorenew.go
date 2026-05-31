package api

import (
	"log"
	"os"
	"strings"

	"github.com/hk59775634/qosnat2/internal/certs"
	"github.com/hk59775634/qosnat2/internal/ocserv"
	"github.com/hk59775634/qosnat2/internal/store"
)

// rebindServiceCertID 将引用 oldID 的 HTTPS/ocserv 绑定切换到 newID
func (srv *Server) rebindServiceCertID(oldID, newID string) {
	oldID = strings.TrimSpace(oldID)
	newID = strings.TrimSpace(newID)
	if oldID == "" || newID == "" || oldID == newID {
		return
	}
	_ = srv.store.Update(func(s *store.State) {
		if strings.TrimSpace(s.System.TLSManagedCertID) == oldID {
			s.System.TLSManagedCertID = newID
		}
		o := &s.VPN.OCServ
		if strings.TrimSpace(o.ManagedCertID) == oldID {
			o.ManagedCertID = newID
		}
		for i := range o.Vhosts {
			if strings.TrimSpace(o.Vhosts[i].ManagedCertID) == oldID {
				o.Vhosts[i].ManagedCertID = newID
			}
		}
	})
	if err := srv.store.Save(); err != nil {
		log.Printf("save state: %v", err)
	}
}

func (srv *Server) applyOCServCertAfterChange(certID string, switchedFrom string) {
	st := srv.store.Get()
	usages := store.ManagedCertUsages(certID, st)
	ocservUsed := false
	for _, u := range usages {
		if strings.HasPrefix(u.Place, "ocserv") {
			ocservUsed = true
			break
		}
	}
	if !ocservUsed {
		return
	}
	o := st.VPN.OCServ
	if err := ocserv.WriteConf(o, st.Certificates); err != nil {
		srv.pushUINotification("error", "OpenConnect 证书未能写入配置",
			"续期后的证书无法写入 ocserv.conf："+err.Error(), "#/vpn/ocserv")
		return
	}
	if !ocserv.InstallInfo().Active {
		srv.pushUINotification("info", "OpenConnect 证书已更新",
			"配置已保存。启用 ocserv 后将使用新证书。", "#/vpn/ocserv")
		return
	}
	if err := ocserv.Reload(o); err != nil {
		srv.pushUINotification("warn", "OpenConnect 需手动应用证书",
			"证书文件已更新，但 occtl reload 未成功（未重启服务）。请在 VPN → OpenConnect 点击「保存并应用」："+err.Error(),
			"#/vpn/ocserv")
		return
	}
	msg := "新证书已通过 reload 生效，未重启 ocserv 服务。"
	if switchedFrom != "" {
		msg = "已优先使用证书库中同域名、有效期更长的证书，" + msg
	}
	srv.pushUINotification("success", "OpenConnect 证书已更新", msg, "#/vpn/ocserv")
}

func (srv *Server) applySystemTLSAfterCertRenewNotify(certID string) {
	if strings.TrimSpace(srv.store.Get().System.TLSManagedCertID) != certID {
		return
	}
	if _, err := srv.applyTLSFromManagedCertID(certID); err != nil {
		srv.pushUINotification("error", "HTTPS 证书未能写入",
			err.Error(), "#/system/general")
		return
	}
	srv.recordAcmeResult(nil)
	srv.pushUINotification("success", "HTTPS 证书已更新",
		"证书已写入 /etc/qosnat2/tls.crt 与 tls.key。新证书将在后续 TLS 连接时自动加载（无需重启 qosnatd）；请刷新浏览器或重新打开页面以建立新连接。",
		"#/system/general")
}

func (srv *Server) tryUseLibraryCertFirst(boundID string) (usedID string, switched bool) {
	st := srv.store.Get()
	cur, ok := store.FindManagedCert(st.Certificates, boundID)
	if !ok {
		return boundID, false
	}
	for _, domain := range cur.Domains {
		best, found := certs.FindBestCertForDomain(st.Certificates, domain, boundID)
		if !found || !certs.IsFresherThan(best, cur) {
			continue
		}
		srv.rebindServiceCertID(boundID, best.ID)
		srv.applyOCServCertAfterChange(best.ID, boundID)
		srv.applySystemTLSAfterCertRenewNotify(best.ID)
		return best.ID, true
	}
	return boundID, false
}

func (srv *Server) tryServiceCertAutoRenew(boundID string) {
	st := srv.store.Get()
	mc, ok := store.FindManagedCert(st.Certificates, boundID)
	if !ok || mc.Type != store.CertTypeACME || mc.CertPath == "" {
		return
	}
	if !store.CertShouldAutoRenew(mc) {
		return
	}
	days := certs.DaysUntilExpiry(mc.CertPath)
	renewBefore := store.ServiceBoundCertRenewBeforeDays(mc)
	if days < 0 || days > renewBefore {
		return
	}
	log.Printf("acme: service-bound cert %s (%s) expires in %d days (renew when <=%d)", boundID, mc.Name, days, renewBefore)

	if newID, switched := srv.tryUseLibraryCertFirst(boundID); switched {
		log.Printf("acme: switched service binding %s -> %s (library cert)", boundID, newID)
		return
	}

	if err := srv.executeManagedCertRenew(boundID, renewApplyService); err != nil {
		info := certs.ClassifyACMEError(err)
		title := "证书自动续期失败"
		msg := err.Error()
		if info.Summary != "" {
			msg = info.Summary
		}
		srv.pushUINotification("error", title, msg, "#/system/certificates")
		return
	}
	srv.applyOCServCertAfterChange(boundID, "")
	srv.applySystemTLSAfterCertRenewNotify(boundID)
	srv.pushUINotification("success", "证书已自动续期",
		mc.Name+" 续期成功。OpenConnect 已尝试 reload；HTTPS 新证书将在下次连接时生效。",
		"#/system/certificates")
}

func (srv *Server) maybeRenewServiceBoundCertificates() {
	if os.Getuid() != 0 {
		return
	}
	st := srv.store.Get()
	for _, id := range store.ServiceBoundCertIDs(st) {
		srv.tryServiceCertAutoRenew(id)
	}
}
