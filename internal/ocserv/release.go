package ocserv

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/hk59775634/qosnat2/internal/releasecatalog"
)

const (
	ocservReleaseTagFile = "/var/lib/qosnat2/ocserv-release-tag"
	ocservReleaseAsset   = "ocserv-linux-amd64.tar.gz"
)

// VersionStatus 可安装版本与当前运行信息。
type VersionStatus struct {
	BinaryPath      string              `json:"binary_path"`
	CurrentTag      string              `json:"current_tag"`
	CurrentVersion  string              `json:"current_version"`
	Installed       bool                `json:"installed"`
	Active          bool                `json:"active"`
	AllowSource     bool                `json:"allow_source_install"`
	RootRequired    bool                `json:"root_required"`
	ListError       string              `json:"list_error,omitempty"`
	Releases        []ReleaseVersionItem `json:"releases"`
}

type ReleaseVersionItem struct {
	Tag          string `json:"tag"`
	Name         string `json:"name,omitempty"`
	Prerelease   bool   `json:"prerelease"`
	PublishedAt  string `json:"published_at,omitempty"`
}

// VersionInfo 返回 OCServ 版本管理状态。
func VersionInfo() VersionStatus {
	st := InstallInfo()
	tag := releasecatalog.NormalizeOcservVersion(readOcservTagFile())
	ver := strings.TrimSpace(st.Version)
	if tag == "" && ver != "" {
		tag = releasecatalog.NormalizeOcservVersion(parseVersionFromOcservOutput(ver))
	}
	entries, listErr := releasecatalog.ListEntries("ocserv")
	releases := manifestToOcservItems(entries)
	out := VersionStatus{
		BinaryPath:     st.Binary,
		CurrentTag:     tag,
		CurrentVersion: ver,
		Installed:      st.Installed,
		Active:         st.Active,
		AllowSource:    AllowSourceInstall(),
		RootRequired:   os.Getuid() == 0,
		Releases:       releases,
	}
	if listErr != nil {
		out.ListError = listErr.Error()
	}
	return out
}

func manifestToOcservItems(entries []releasecatalog.VersionEntry) []ReleaseVersionItem {
	out := make([]ReleaseVersionItem, 0, len(entries))
	for _, e := range entries {
		id := releasecatalog.NormalizeOcservVersion(e.ID)
		if id == "" {
			id = releasecatalog.NormalizeOcservVersion(e.Tag)
		}
		if id == "" {
			continue
		}
		out = append(out, ReleaseVersionItem{
			Tag:         id,
			Name:        id,
			PublishedAt: strings.TrimSpace(e.PublishedAt),
		})
	}
	return out
}

func readOcservTagFile() string {
	b, err := os.ReadFile(ocservReleaseTagFile)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func saveOcservReleaseTag(tag string) {
	_ = os.MkdirAll(filepath.Dir(ocservReleaseTagFile), 0755)
	_ = os.WriteFile(ocservReleaseTagFile, []byte(strings.TrimSpace(tag)+"\n"), 0644)
}

func parseVersionFromOcservOutput(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	fields := strings.Fields(s)
	for i, f := range fields {
		if f == "version" && i+1 < len(fields) {
			return strings.TrimPrefix(fields[i+1], "v")
		}
	}
	if len(fields) > 0 {
		return strings.TrimPrefix(fields[len(fields)-1], "v")
	}
	return ""
}

func ocservReleaseDownloadURL(version string) string {
	return releasecatalog.OcservDownloadURL(version)
}

// InstallReleaseVersion 下载并安装指定 ocserv 版本（二进制包），不启动服务。
func InstallReleaseVersion(version string) error {
	version = releasecatalog.NormalizeOcservVersion(version)
	if version == "" {
		return fmt.Errorf("version required")
	}
	if !releasecatalog.ValidOcservVersion(version) {
		return fmt.Errorf("invalid ocserv version (expected official tag e.g. 1.4.2)")
	}
	script := InstallScriptPath()
	if _, err := os.Stat(script); err != nil {
		return fmt.Errorf("install script not found: %s", script)
	}
	url := ocservReleaseDownloadURL(version)
	cmd := exec.Command("bash", script, "--method", "release", "--version", version, "--url", url)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("%s", msg)
	}
	saveOcservReleaseTag(releasecatalog.NormalizeOcservVersion(version))
	return nil
}

// SwitchReleaseVersion 安装指定版本并重启 ocserv 服务。
func SwitchReleaseVersion(version string) error {
	if err := InstallReleaseVersion(version); err != nil {
		return err
	}
	out, err := exec.Command("systemctl", "restart", "ocserv").CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl restart: %s %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}
