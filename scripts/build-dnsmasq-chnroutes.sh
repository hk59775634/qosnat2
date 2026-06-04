#!/usr/bin/env bash
# 从源码编译带 chnroutes 补丁的 dnsmasq，并替换系统 /usr/sbin/dnsmasq（保留 .dist 备份）。
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DNSMASQ_VERSION="${DNSMASQ_VERSION:-2.90}"
PATCH_REPO="${PATCH_REPO:-https://raw.githubusercontent.com/hk59775634/dnsmasq-chnroute-path/master}"
INSTALL_BIN="${INSTALL_BIN:-/usr/sbin/dnsmasq}"
QOSNAT_LIB="${QOSNAT_LIB:-/usr/local/lib/qosnat2}"
WORK="${WORK:-/tmp/qosnat2-dnsmasq-build}"

log() { echo "[$(date '+%F %T')] $*"; }
warn() { echo "[$(date '+%F %T')] WARN: $*" >&2; }
die() { echo "[$(date '+%F %T')] ERROR: $*" >&2; exit 1; }

need() { command -v "$1" &>/dev/null || die "missing command: $1 (install build deps via apt)"; }

# 与 Ubuntu dnsmasq 包对齐的编译选项（systemd-helper 需要 DBus/DNSSEC 等）
DNSMASQ_COPTS="${DNSMASQ_COPTS:--DHAVE_DBUS -DHAVE_DNSSEC -DHAVE_CONNTRACK -DHAVE_IDN2 -DHAVE_IPSET -DHAVE_NFTSET -DHAVE_AUTH -DHAVE_LOOP -DHAVE_DUMPFILE}"
# 编译工具 + 头文件库（UI/API 触发安装时主机可能仅有 dnsmasq 运行时包）
DNSMASQ_APT_PACKAGES=(
  build-essential
  curl
  patch
  pkg-config
  python3
  libdbus-1-dev
  libidn2-dev
  nettle-dev
  libnftables-dev
)

if [[ "${EUID:-$(id -u)}" -ne 0 ]]; then
  die "run as root (sudo $0)"
fi

if command -v apt-get &>/dev/null; then
  export DEBIAN_FRONTEND=noninteractive
  log "install build dependencies (apt)…"
  apt-get update -qq || warn "apt-get update failed, continuing"
  apt-get install -y -qq "${DNSMASQ_APT_PACKAGES[@]}" || die "install dnsmasq build deps failed"
else
  warn "no apt-get; assuming build tools are already installed"
fi

need curl
need patch
need make
need cc
need pkg-config
need python3

mkdir -p "${WORK}" "${QOSNAT_LIB}"
cd "${WORK}"

TARBALL="dnsmasq-${DNSMASQ_VERSION}.tar.gz"
if [[ ! -f "${TARBALL}" ]]; then
  log "download dnsmasq ${DNSMASQ_VERSION}…"
  curl -fsSL -o "${TARBALL}" "https://thekelleys.org.uk/dnsmasq/${TARBALL}"
fi

SRC="dnsmasq-${DNSMASQ_VERSION}"
rm -rf "${SRC}"
tar xzf "${TARBALL}"

PATCH="${WORK}/chnroutes.patch"
curl -fsSL -o "${PATCH}" \
  "${PATCH_REPO}/0001-Add-feature-to-support-chnroutes-${DNSMASQ_VERSION}-openwrt24.10.patch"

cd "${SRC}"
patch -p1 < "${PATCH}"
python3 <<'PY'
from pathlib import Path
p = Path("src/dnsmasq.h")
text = p.read_text()
old = """int add_update_server(int flags,
\t\t      union mysockaddr *addr,
\t\t      union mysockaddr *source_addr,
\t\t      const char *interface,
\t\t      const char *domain,
\t\t      union all_addr *local_addr); """
new = """int add_update_server(int flags,
\t\t      union mysockaddr *addr,
\t\t      union mysockaddr *source_addr,
\t\t      const char *interface,
\t\t      const char *domain,
\t\t      union all_addr *local_addr,
\t\t      int trust); """
if old not in text:
    raise SystemExit("dnsmasq.h add_update_server signature not found")
p.write_text(text.replace(old, new, 1))
PY

log "compile dnsmasq ${DNSMASQ_VERSION} (chnroutes)…"
make clean 2>/dev/null || true
make COPTS="${DNSMASQ_COPTS}" -j"$(nproc 2>/dev/null || echo 2)"

BUILT="${WORK}/${SRC}/src/dnsmasq"
[[ -x "${BUILT}" ]] || die "build failed: ${BUILT}"

# 仅产出预编译二进制（release 打包），不写入系统路径
if [[ -n "${OUTPUT:-}" ]]; then
  mkdir -p "$(dirname "${OUTPUT}")"
  install -m 0755 "${BUILT}" "${OUTPUT}"
  log "prebuilt written: ${OUTPUT}"
  "${OUTPUT}" --help 2>&1 | grep -q chnroutes-file || die "prebuilt lacks chnroutes-file"
  exit 0
fi

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
bash "${ROOT_DIR}/scripts/install-dnsmasq-chnroutes-binary.sh" "${BUILT}"

if ! "${INSTALL_BIN}" --test -7 /etc/dnsmasq.d,.dpkg-dist,.dpkg-old,.dpkg-new --local-service >/dev/null 2>&1; then
  warn "dnsmasq --test with --local-service failed; check compile options"
fi
