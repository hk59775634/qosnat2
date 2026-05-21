package geoip

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DataDir 国家 CIDR 列表目录（每国一个 ISO2.cidr，每行一条）
const DataDir = "/var/lib/qosnat2/geoip"

// LoadCIDRs 优先 custom，否则读 DataDir/{CC}.cidr
func LoadCIDRs(country string, custom []string) ([]string, error) {
	if len(custom) > 0 {
		return normalizeLines(custom), nil
	}
	cc := strings.ToUpper(strings.TrimSpace(country))
	if len(cc) != 2 {
		return nil, fmt.Errorf("invalid country code")
	}
	path := filepath.Join(DataDir, cc+".cidr")
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("geoip data missing: %s (place %s.cidr in %s)", cc, cc, DataDir)
		}
		return nil, err
	}
	var out []string
	sc := bufio.NewScanner(strings.NewReader(string(b)))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no CIDRs in %s", path)
	}
	return out, nil
}

func normalizeLines(in []string) []string {
	var out []string
	for _, line := range in {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out
}

// EnsureDataDir 创建 GeoIP 数据目录
func EnsureDataDir() error {
	return os.MkdirAll(DataDir, 0755)
}
