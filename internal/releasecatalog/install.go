package releasecatalog

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/hk59775634/qosnat2/internal/dnsmasq"
)

// FetchReleaseArchive 下载 release 压缩包。
func FetchReleaseArchive(versionID, route string) ([]byte, string, error) {
	urls := QosnatDownloadURLsForRoute(versionID, route)
	if len(urls) == 0 {
		return nil, "", fmt.Errorf("invalid version id")
	}
	return FetchBytesRelease(urls)
}

// InstallReleaseBinary 下载并安装指定版本的 qosnatd 与 BPF 对象。
func InstallReleaseBinary(versionID, binPath, route string) error {
	gz, _, err := FetchReleaseArchive(versionID, route)
	if err != nil {
		return err
	}
	tmp, err := os.MkdirTemp("", "qosnat2-switch-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	if err := ExtractReleaseTarGz(gz, tmp); err != nil {
		return fmt.Errorf("extract release: %w", err)
	}
	bin := filepath.Join(tmp, "qosnatd-linux-amd64")
	if _, err := os.Stat(bin); err != nil {
		return fmt.Errorf("release missing qosnatd-linux-amd64")
	}
	// 不能覆盖正在运行的可执行文件（会 ETXTBSY/text file busy）；
	// 先写到 .new，再 rename 换入目标路径（进程仍持有旧 inode，重启后加载新文件）。
	if err := replaceFileAtomic(bin, binPath, 0755); err != nil {
		return fmt.Errorf("install binary: %w", err)
	}
	edtBPF := filepath.Join(tmp, "lib", "rate_edt.bpf.o")
	if _, err := os.Stat(edtBPF); err == nil {
		destDir := "/usr/lib/qosnat2"
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return err
		}
		if err := replaceFileAtomic(edtBPF, filepath.Join(destDir, "rate_edt.bpf.o"), 0644); err != nil {
			return fmt.Errorf("install rate_edt bpf: %w", err)
		}
	}
	if !dnsmasq.SupportsChnroutes() {
		prebuilt := filepath.Join(tmp, dnsmasq.ReleaseTarDnsmasqRel)
		if _, err := os.Stat(prebuilt); err == nil {
			if err := dnsmasq.InstallChnroutesBinary(prebuilt); err != nil {
				return fmt.Errorf("install dnsmasq-chnroutes: %w", err)
			}
		}
	}
	return nil
}

// ExtractReleaseTarGz 解压 qosnat2-linux-amd64.tar.gz 到目录。
func ExtractReleaseTarGz(gz []byte, destDir string) error {
	gr, err := gzip.NewReader(bytes.NewReader(gz))
	if err != nil {
		return err
	}
	defer gr.Close()
	tr := tar.NewReader(gr)
	destClean := filepath.Clean(destDir)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		name := strings.TrimPrefix(hdr.Name, "./")
		name = filepath.Clean(name)
		if name == "." || strings.HasPrefix(name, "..") || filepath.IsAbs(name) {
			continue
		}
		target := filepath.Join(destClean, name)
		if !strings.HasPrefix(target, destClean+string(os.PathSeparator)) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		mode := os.FileMode(hdr.Mode) & 0777
		f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
		if err != nil {
			return err
		}
		if _, err := io.Copy(f, tr); err != nil {
			f.Close()
			return err
		}
		f.Close()
	}
}

// replaceFileAtomic 用 staging+rename 替换目标文件，避免覆盖正在使用中的路径（ETXTBSY）。
func replaceFileAtomic(src, dst string, mode os.FileMode) error {
	staging := dst + ".new"
	if err := copyFile(src, staging, mode); err != nil {
		return err
	}
	backup := dst + ".old"
	_ = os.Remove(backup)
	if _, err := os.Stat(dst); err == nil {
		if err := os.Rename(dst, backup); err != nil {
			_ = os.Remove(staging)
			return fmt.Errorf("rename %s aside: %w", dst, err)
		}
	}
	if err := os.Rename(staging, dst); err != nil {
		if _, err2 := os.Stat(backup); err2 == nil {
			_ = os.Rename(backup, dst)
		}
		return fmt.Errorf("activate %s: %w", dst, err)
	}
	_ = os.Remove(backup)
	return nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Chmod(mode)
}
