//go:build release

package ocserv

// AllowSourceInstall release 构建通过源码编译安装 ocserv（不提供预编译包切换）。
func AllowSourceInstall() bool { return true }
