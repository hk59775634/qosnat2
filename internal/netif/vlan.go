package netif

import "fmt"

// VLANName 由 parent + vid 生成接口名（与 netplan 一致）
func VLANName(parent string, vid int) string {
	return fmt.Sprintf("%s.%d", parent, vid)
}
