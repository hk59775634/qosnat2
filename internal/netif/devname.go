package netif

import (
	"fmt"
	"regexp"
	"strings"
)

// Linux IF_NAMESIZE 为 16（含 '\0'），网卡名最长 15 字符
const maxIfaceNameLen = 15

var ifaceNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

// ValidateIfaceName 校验网卡名，防止 /sys/class/net 路径穿越与异常 tc/ethtool 参数
func ValidateIfaceName(dev string) error {
	dev = strings.TrimSpace(dev)
	if dev == "" {
		return fmt.Errorf("empty device name")
	}
	if len(dev) > maxIfaceNameLen {
		return fmt.Errorf("device name too long (max %d)", maxIfaceNameLen)
	}
	if strings.Contains(dev, "/") || strings.Contains(dev, "\\") || strings.Contains(dev, "..") {
		return fmt.Errorf("invalid device name")
	}
	if !ifaceNameRe.MatchString(dev) {
		return fmt.Errorf("invalid device name %q", dev)
	}
	return nil
}
