package netif

import "fmt"

// SetConfig 已弃用：网卡地址由 netplan 管理，请通过 state + ApplyNetplan
func SetConfig(dev string, ipv4 []string, up *bool) error {
	_ = dev
	_ = ipv4
	_ = up
	return fmt.Errorf("interface %q: direct ip configuration disabled; use netplan (99-qosnat2.yaml)", dev)
}
