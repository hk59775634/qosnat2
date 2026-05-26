package store

import (
	"fmt"
	"regexp"
	"strings"
)

const maxIfaceNameLen = 15

var ifaceNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

// ValidateIfaceName 校验 Linux 网卡名（防火墙 iif/oif 等嵌入 nft 时使用）。
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
