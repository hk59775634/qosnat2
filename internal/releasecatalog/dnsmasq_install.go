package releasecatalog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hk59775634/qosnat2/internal/dnsmasq"
)

const releaseTagFile = "/etc/qosnat2/release-tag"

// InstallDnsmasqChnroutesFromRelease 从当前已安装版本的 release 包安装预编译 dnsmasq-chnroutes。
func InstallDnsmasqChnroutesFromRelease(route string) error {
	if dnsmasq.SupportsChnroutes() {
		return nil
	}
	versionID, err := readInstalledReleaseID()
	if err != nil {
		return err
	}
	gz, _, err := FetchReleaseArchive(versionID, route)
	if err != nil {
		return err
	}
	tmp, err := os.MkdirTemp("", "qosnat2-dnsmasq-prebuilt-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	if err := ExtractReleaseTarGz(gz, tmp); err != nil {
		return fmt.Errorf("extract release: %w", err)
	}
	prebuilt := filepath.Join(tmp, dnsmasq.ReleaseTarDnsmasqRel)
	if _, err := os.Stat(prebuilt); err != nil {
		return fmt.Errorf("release %s missing %s (upgrade qosnat2 release)", versionID, dnsmasq.ReleaseTarDnsmasqRel)
	}
	return dnsmasq.InstallChnroutesBinary(prebuilt)
}

func readInstalledReleaseID() (string, error) {
	b, err := os.ReadFile(releaseTagFile)
	if err != nil {
		return "", fmt.Errorf("read %s: %w (install qosnat2 release first)", releaseTagFile, err)
	}
	id := strings.TrimSpace(string(b))
	id = strings.TrimPrefix(id, "v")
	if len(id) != 10 {
		return "", fmt.Errorf("invalid release tag in %s", releaseTagFile)
	}
	return id, nil
}
