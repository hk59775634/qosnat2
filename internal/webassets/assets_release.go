//go:build release

package webassets

import "embed"

// Static 由 scripts/build-release.sh 从 web/dist 同步到 static/ 后嵌入。
//
//go:embed static/*
var Static embed.FS

// BPFEDT rate_edt.bpf.o（EDT 数据面）
//
//go:embed rate_edt.bpf.o
var BPFEDT []byte

func Enabled() bool { return true }
