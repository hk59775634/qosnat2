# qosnat2 build
BPF_CLANG ?= clang
BPF_CFLAGS := -O2 -g -target bpf -D__TARGET_ARCH_x86

.PHONY: all bpf go install clean

all: go bpf

go:
	go build -o bin/qosnatd ./cmd/qosnatd

bpf:
	$(MAKE) -C bpf

install: go
	install -m 0755 bin/qosnatd /usr/local/bin/qosnatd

clean:
	rm -f bin/qosnatd
	$(MAKE) -C bpf clean
