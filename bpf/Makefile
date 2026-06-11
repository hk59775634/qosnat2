CLANG ?= clang
LLC ?= llc
ARCH := $(shell uname -m | sed 's/x86_64/x86/' | sed 's/aarch64/arm64/')
ARCH_INC := /usr/include/$(shell uname -m)-linux-gnu
BPF_CFLAGS := -O2 -g -target bpf -D__TARGET_ARCH_$(ARCH) -I$(ARCH_INC)

OBJS := classify.bpf.o rate_edt.bpf.o
INSTALL_DIR ?= /usr/lib/qosnat2

all: $(OBJS)

%.bpf.o: %.bpf.c headers/qosnat2.h
	$(CLANG) $(BPF_CFLAGS) -I. -c $< -o $@

rate_edt.bpf.o: rate_edt.bpf.c headers/qosnat2.h headers/edt.h
	$(CLANG) $(BPF_CFLAGS) -I. -c rate_edt.bpf.c -o rate_edt.bpf.o

install: $(OBJS)
	install -d $(INSTALL_DIR)
	install -m 0644 classify.bpf.o rate_edt.bpf.o $(INSTALL_DIR)/

clean:
	rm -f *.bpf.o
