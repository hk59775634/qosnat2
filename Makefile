# qosnat2 build
BPF_CLANG ?= clang
BPF_CFLAGS := -O2 -g -target bpf -D__TARGET_ARCH_x86

.PHONY: all bpf go install clean release

all: go bpf

# 单文件 release（内嵌 web/dist + classify.bpf.o），见 scripts/build-release.sh
release:
	./scripts/build-release.sh

go:
	go build -o bin/qosnatd ./cmd/qosnatd

bpf:
	$(MAKE) -C bpf

install: go
	install -m 0755 bin/qosnatd /usr/local/bin/qosnatd

clean:
	rm -f bin/qosnatd
	$(MAKE) -C bpf clean
