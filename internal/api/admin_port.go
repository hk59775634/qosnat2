package api

import (
	"fmt"
	"net"
	"time"

	"github.com/hk59775634/qosnat2/internal/ebpf"
	"github.com/hk59775634/qosnat2/internal/netutil"
	"github.com/hk59775634/qosnat2/internal/store"
)

const initialAdminFile = "/etc/qosnat2/initial-admin.txt"

// ChangeAdminPort 更新 ADMIN_PORT、同步防火墙规则，并重启 qosnatd（需 root）。
func ChangeAdminPort(newPort string) (string, error) {
	validPort, err := netutil.ValidateListenPort(newPort)
	if err != nil {
		return "", err
	}
	env := LoadEnv()
	if env.AdminPort == validPort {
		return validPort, nil
	}
	if tcpPortInUse(validPort) {
		return "", fmt.Errorf("port %s already in use", validPort)
	}
	env.AdminPort = validPort
	if err := WriteRuntimeEnv(env); err != nil {
		return "", err
	}
	updateInitialAdminFileKey("ADMIN_PORT", validPort)

	st := store.New(env.StateFile)
	if err := st.Load(); err != nil {
		return "", fmt.Errorf("load state: %w", err)
	}
	if st.Get().SetupComplete {
		srv := New(env, st, ebpf.New())
		if err := srv.reloadNft(); err != nil {
			return "", fmt.Errorf("reload nft: %w", err)
		}
	}
	if err := restartQoSnatd(); err != nil {
		return "", err
	}
	return validPort, nil
}

func tcpPortInUse(port string) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", port), 500*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
