package netif

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// IfIndex returns the kernel ifIndex for dev (/sys/class/net/<dev>/ifindex).
func IfIndex(dev string) (int, error) {
	dev = strings.TrimSpace(dev)
	if err := ValidateIfaceName(dev); err != nil {
		return 0, err
	}
	b, err := os.ReadFile(filepath.Join("/sys/class/net", dev, "ifindex"))
	if err != nil {
		return 0, fmt.Errorf("ifindex %s: %w", dev, err)
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("ifindex %s: invalid value", dev)
	}
	return n, nil
}
