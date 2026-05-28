//go:build !release

package ocserv

// AllowSourceInstall 开发构建允许 UI 触发源码编译安装。
func AllowSourceInstall() bool { return true }
