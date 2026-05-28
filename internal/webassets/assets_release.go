//go:build release

package webassets

import "embed"

// Static 由 scripts/build-release.sh 从 web/dist 同步到 static/ 后嵌入。
//
//go:embed static/*
var Static embed.FS

// BPF classify.bpf.o（由 build-release.sh 复制到本目录）。
//
//go:embed classify.bpf.o
var BPF []byte

func Enabled() bool { return true }
