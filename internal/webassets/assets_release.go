//go:build release

package webassets

import "embed"

// Static 由 scripts/build-release.sh 从 web/dist 同步到 static/ 后嵌入。
//
//go:embed static/*
var Static embed.FS

// BPF classify.bpf.o（HTB 旧模式）与 rate_edt.bpf.o（EDT 默认）
//
//go:embed classify.bpf.o
var BPF []byte

//go:embed rate_edt.bpf.o
var BPFEDT []byte

func Enabled() bool { return true }
