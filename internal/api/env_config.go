package api

import (
	"fmt"
	"os"
)

var defaultEnvPath = "/etc/qosnat2/env"

// ValidateEnv 启动校验：未完成引导时允许空网卡
func ValidateEnv(e Env) error {
	if e.AdminPort == "" {
		return fmt.Errorf("ADMIN_PORT required")
	}
	return nil
}

// WriteRuntimeEnv 写入 /etc/qosnat2/env（引导完成或更新网卡；合并保留 TLS 等已有项）
func WriteRuntimeEnv(e Env) error {
	return writeRuntimeEnvMerged(e)
}

// WriteDevRoles 写入 WAN/LAN 网卡映射；空字符串表示清除对应项
func WriteDevRoles(lan, wan string) error {
	m := readEnvFileMap()
	if lan == "" {
		delete(m, "DEV_LAN")
	} else {
		m["DEV_LAN"] = lan
	}
	if wan == "" {
		delete(m, "DEV_WAN")
	} else {
		m["DEV_WAN"] = wan
	}
	if err := writeEnvMap(m); err != nil {
		return err
	}
	syncDevRolesFromFile()
	return nil
}

// ClearAdminPassFromEnv 引导完成后从 env 文件移除明文 ADMIN_PASS。
func ClearAdminPassFromEnv() error {
	m := readEnvFileMap()
	delete(m, "ADMIN_PASS")
	_ = os.Unsetenv("ADMIN_PASS")
	return writeEnvMap(m)
}

// syncDevRolesFromFile 从 /etc/qosnat2/env 同步 DEV_LAN/DEV_WAN 到进程环境。
// systemd 启动时已注入的旧值不会被 InitFromEnvFile 覆盖，保存网卡角色后必须显式刷新。
func syncDevRolesFromFile() (lan, wan string) {
	m := readEnvFileMap()
	lan = m["DEV_LAN"]
	wan = m["DEV_WAN"]
	if lan != "" {
		_ = os.Setenv("DEV_LAN", lan)
	} else {
		_ = os.Unsetenv("DEV_LAN")
	}
	if wan != "" {
		_ = os.Setenv("DEV_WAN", wan)
	} else {
		_ = os.Unsetenv("DEV_WAN")
	}
	return lan, wan
}
