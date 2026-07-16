package api

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	gatewayAptPeriodicConf  = "/etc/apt/apt.conf.d/20qosnat2-gateway.conf"
	gatewayAptBlacklistConf = "/etc/apt/apt.conf.d/51qosnat2-gateway-unattended.conf"

	gatewayAptPeriodicInline = `// qosnat2 生产网关：禁止 unattended-upgrades 自动安装/下载升级包。
APT::Periodic::Update-Package-Lists "0";
APT::Periodic::Download-Upgradeable-Packages "0";
APT::Periodic::Unattended-Upgrade "0";
APT::Periodic::AutocleanInterval "0";
`

	gatewayAptBlacklistInline = `// qosnat2 生产网关：黑名单屏蔽网络/数据面关键包
Unattended-Upgrade::Package-Blacklist {
    "systemd";
    "systemd-sysv";
    "systemd-timesyncd";
    "udev";
    "netplan.io";
    "netplan-generator";
    "frr";
    "frr-pythontools";
    "linux-image-.*";
    "linux-headers-.*";
    "linux-modules-.*";
    "linux-generic";
    "linux-tools-.*";
    "iproute2";
    "nftables";
    "dnsmasq";
    "openssh-server";
    "openssh-client";
};
`
)

// ensureGatewayAptLockdown 生产网关：若尚未配置 apt 限制，在 qosnatd 启动时自动 lockdown。
// 设 QOSNAT_GATEWAY_APT=off 可跳过。
func (srv *Server) ensureGatewayAptLockdown() {
	if os.Getuid() != 0 {
		return
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("QOSNAT_GATEWAY_APT")), "off") {
		return
	}
	if _, err := os.Stat(gatewayAptPeriodicConf); err == nil {
		return
	}
	if script := findConfigureGatewayAptScript(); script != "" {
		out, err := exec.Command("bash", script, "lockdown").CombinedOutput()
		if err == nil {
			logGatewayAptOutput(string(out))
			return
		}
		log.Printf("gateway apt lockdown script: %v: %s", err, strings.TrimSpace(string(out)))
	}
	if err := applyGatewayAptLockdownInline(); err != nil {
		log.Printf("gateway apt lockdown: %v", err)
		return
	}
	log.Printf("gateway apt: lockdown applied")
}

func applyGatewayAptLockdownInline() error {
	if err := os.WriteFile(gatewayAptPeriodicConf, []byte(gatewayAptPeriodicInline), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(gatewayAptBlacklistConf, []byte(gatewayAptBlacklistInline), 0644); err != nil {
		return err
	}
	_ = exec.Command("systemctl", "disable", "--now", "apt-daily-upgrade.timer").Run()
	_ = exec.Command("systemctl", "disable", "--now", "apt-daily.timer").Run()
	return nil
}

func logGatewayAptOutput(out string) {
	for _, ln := range strings.Split(out, "\n") {
		if t := strings.TrimSpace(ln); t != "" {
			log.Printf("gateway apt: %s", t)
		}
	}
}

func findConfigureGatewayAptScript() string {
	for _, root := range []string{os.Getenv("QOSNAT_ROOT"), "/opt/qosnat2"} {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		p := filepath.Join(root, "scripts", "configure-gateway-apt.sh")
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}
