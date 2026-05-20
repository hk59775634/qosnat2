# NAT-QoS eBPF 编译
CLANG ?= clang
LLC   ?= llc
ARCH  := $(shell uname -m | sed 's/x86_64/x86/' | sed 's/aarch64/arm64/')
BPF_DIR := bpf
OUT_DIR := build

CFLAGS := -O2 -g -Wall -Werror \
	-target bpf -D__TARGET_ARCH_$(ARCH) \
	-I/usr/include/$(shell uname -m)-linux-gnu \
	-I/usr/include

.PHONY: all clean install

all: $(OUT_DIR)/ratelimit.bpf.o

$(OUT_DIR):
	mkdir -p $(OUT_DIR)

$(OUT_DIR)/ratelimit.bpf.o: $(BPF_DIR)/ratelimit.bpf.c | $(OUT_DIR)
	$(CLANG) $(CFLAGS) -c $< -o $@

install: all
	install -d /usr/lib/nat-qos
	install -m 644 $(OUT_DIR)/ratelimit.bpf.o /usr/lib/nat-qos/ratelimit.bpf.o

clean:
	rm -rf $(OUT_DIR)
