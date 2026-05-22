package netif

import "fmt"

// VXLANIfaceName 默认 VXLAN 接口名
func VXLANIfaceName(vni int) string {
	return fmt.Sprintf("vxlan%d", vni)
}
