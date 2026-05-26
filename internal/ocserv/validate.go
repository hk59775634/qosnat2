package ocserv

import (
	"fmt"

	"github.com/hk59775634/qosnat2/internal/store"
)

const ciscoSvcRequiredUDPPort = 443

// UsesCiscoSvcCompat 全局或任一 vhost 启用了 cisco-svc-client-compat
func UsesCiscoSvcCompat(o store.OCServState) bool {
	if o.Advanced.CiscoSvcCompat {
		return true
	}
	for _, v := range o.Vhosts {
		if v.CiscoSvcCompat {
			return true
		}
	}
	return false
}

// ValidateState 在写入/应用 ocserv 配置前校验（与 ocserv 二进制约束一致）
func ValidateState(o store.OCServState) error {
	if !UsesCiscoSvcCompat(o) {
		return nil
	}
	if !o.Advanced.Udp {
		return fmt.Errorf("cisco-svc-client-compat requires UDP enabled (udp-port = %d)", ciscoSvcRequiredUDPPort)
	}
	if o.UDPPort != ciscoSvcRequiredUDPPort {
		return fmt.Errorf("cisco-svc-client-compat requires udp-port = %d (current: %d)", ciscoSvcRequiredUDPPort, o.UDPPort)
	}
	return nil
}
