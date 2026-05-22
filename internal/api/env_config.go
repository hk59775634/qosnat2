package api

import (
	"fmt"
)

const defaultEnvPath = "/etc/qosnat2/env"

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
