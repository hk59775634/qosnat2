// Package releasecatalog 从 GitHub 仓库 manifest 读取可切换的 release 版本列表。
package releasecatalog

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	GitHubRepo     = "hk59775634/qosnat2"
	DefaultBranch  = "main"
	MaxVersions    = 5
	VersionIDLen   = 10
)

var qosnatVersionIDRe = regexp.MustCompile(`^\d{10}$`)

// Manifest 存于 releases/{product}-versions.json（main 分支 raw 内容）。
type Manifest struct {
	Schema   int            `json:"schema"`
	MaxKeep  int            `json:"max_keep"`
	Versions []VersionEntry `json:"versions"`
}

type VersionEntry struct {
	ID          string `json:"id"`
	Tag         string `json:"tag"`
	PublishedAt string `json:"published_at,omitempty"`
	Summary     string `json:"summary,omitempty"`
}

// ManifestURL 返回 raw.githubusercontent.com 上的 manifest 地址。
func ManifestURL(product string) string {
	product = strings.TrimSpace(product)
	return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/releases/%s-versions.json",
		GitHubRepo, DefaultBranch, product)
}

// FetchManifest 拉取版本清单（直连 GitHub 超时则依次尝试 gh-proxy 镜像）。
func FetchManifest(product string) (*Manifest, error) {
	urls := MirrorURLs(ManifestURL(product))
	body, _, err := FetchBytes(urls)
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// ListEntries 返回 manifest 中的版本条目；拉取失败时 err 非 nil。
func ListEntries(product string) ([]VersionEntry, error) {
	m, err := FetchManifest(product)
	if err != nil {
		return nil, err
	}
	if m == nil || len(m.Versions) == 0 {
		return nil, nil
	}
	out := make([]VersionEntry, len(m.Versions))
	copy(out, m.Versions)
	return out, nil
}

// NormalizeID 去掉 v 前缀，得到 qosnat2 的 10 位版本号。
func NormalizeID(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "v")
	return s
}

// ValidQosnatID 校验 qosnat2 的 YYYYMMDDNN 格式。
func ValidQosnatID(id string) bool {
	id = NormalizeID(id)
	if !qosnatVersionIDRe.MatchString(id) {
		return false
	}
	date := id[:8]
	seq := id[8:]
	if seq < "01" || seq > "99" {
		return false
	}
	_, err := time.Parse("20060102", date)
	return err == nil
}

// ValidID 为 ValidQosnatID 的别名（qosnat2 专用）。
func ValidID(id string) bool { return ValidQosnatID(id) }

// QosnatGitHubTag 将版本号转为 GitHub release tag（v2026052801）。
func QosnatGitHubTag(versionID string) string {
	id := NormalizeID(versionID)
	if id == "" {
		return ""
	}
	return "v" + id
}

// QosnatDownloadURL release 资产直连下载地址。
func QosnatDownloadURL(versionID string) string {
	tag := QosnatGitHubTag(versionID)
	if tag == "" {
		return ""
	}
	return fmt.Sprintf("https://github.com/%s/releases/download/%s/qosnat2-linux-amd64.tar.gz", GitHubRepo, tag)
}

// QosnatDownloadURLs 直连 + gh-proxy 备选下载地址。
func QosnatDownloadURLs(versionID string) []string {
	return MirrorURLs(QosnatDownloadURL(versionID))
}

// NotesURL 返回 raw.githubusercontent.com 上的版本更新说明地址。
func NotesURL(versionID string) string {
	id := NormalizeID(versionID)
	if id == "" {
		return ""
	}
	return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/releases/notes/%s.md",
		GitHubRepo, DefaultBranch, id)
}

// ToReleaseMaps 转为 API/Web 使用的 releases 列表（tag 字段为 10 位版本号）。
func ToReleaseMaps(entries []VersionEntry) []map[string]any {
	out := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		id := NormalizeID(e.ID)
		if id == "" {
			id = NormalizeID(e.Tag)
		}
		if id == "" {
			continue
		}
		m := map[string]any{
			"tag":          id,
			"id":           id,
			"github_tag":   strings.TrimSpace(e.Tag),
			"name":         id,
			"published_at": strings.TrimSpace(e.PublishedAt),
			"notes_url":    NotesURL(id),
		}
		if s := strings.TrimSpace(e.Summary); s != "" {
			m["summary"] = s
		}
		out = append(out, m)
	}
	return out
}
