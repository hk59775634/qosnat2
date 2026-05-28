//go:build release

package ocserv

// AllowSourceInstall release 构建仅使用预编译可执行文件安装/切换。
func AllowSourceInstall() bool { return false }
