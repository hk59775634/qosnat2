//go:build !release

package webassets

import "embed"

// 开发构建不嵌入前端/BPF，仍从磁盘 WEB_ROOT 与 /usr/lib/qosnat2 加载。
var Static embed.FS
var BPF []byte
var BPFEDT []byte

func Enabled() bool { return false }
