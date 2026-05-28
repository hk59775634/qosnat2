#!/usr/bin/env bash
set -euo pipefail

# Standalone test script:
# - create an isolated netns
# - connect host <-> netns with a veth pair
# - bootstrap netns internet via host NAT
# - run Cloudflare WARP only inside netns

NS_NAME="${NS_NAME:-warp-test}"
VETH_HOST="${VETH_HOST:-qwt0}"
VETH_NS="${VETH_NS:-qwt1}"
HOST_CIDR="${HOST_CIDR:-10.199.0.1/30}"
NS_CIDR="${NS_CIDR:-10.199.0.2/30}"
NS_SUBNET="${NS_SUBNET:-10.199.0.0/30}"
WARP_LOG="${WARP_LOG:-/tmp/${NS_NAME}-warp-svc.log}"

require_root() {
  if [[ "${EUID}" -ne 0 ]]; then
    echo "Run as root." >&2
    exit 1
  fi
}

detect_uplink() {
  ip route get 1.1.1.1 2>/dev/null | awk '/dev/ {for (i=1;i<=NF;i++) if ($i=="dev") {print $(i+1); exit}}'
}

iptables_rule_exists() {
  local table="$1"
  shift
  iptables -t "${table}" -C "$@" >/dev/null 2>&1
}

add_host_nat_forward() {
  local uplink="$1"
  sysctl -w net.ipv4.ip_forward=1 >/dev/null

  if ! iptables_rule_exists nat POSTROUTING -s "${NS_SUBNET}" -o "${uplink}" -j MASQUERADE; then
    iptables -t nat -A POSTROUTING -s "${NS_SUBNET}" -o "${uplink}" -j MASQUERADE
  fi
  if ! iptables_rule_exists filter FORWARD -i "${VETH_HOST}" -o "${uplink}" -j ACCEPT; then
    iptables -A FORWARD -i "${VETH_HOST}" -o "${uplink}" -j ACCEPT
  fi
  if ! iptables_rule_exists filter FORWARD -i "${uplink}" -o "${VETH_HOST}" -m state --state RELATED,ESTABLISHED -j ACCEPT; then
    iptables -A FORWARD -i "${uplink}" -o "${VETH_HOST}" -m state --state RELATED,ESTABLISHED -j ACCEPT
  fi
}

del_host_nat_forward() {
  local uplink="$1"
  iptables -t nat -D POSTROUTING -s "${NS_SUBNET}" -o "${uplink}" -j MASQUERADE >/dev/null 2>&1 || true
  iptables -D FORWARD -i "${VETH_HOST}" -o "${uplink}" -j ACCEPT >/dev/null 2>&1 || true
  iptables -D FORWARD -i "${uplink}" -o "${VETH_HOST}" -m state --state RELATED,ESTABLISHED -j ACCEPT >/dev/null 2>&1 || true
}

create_netns_topology() {
  local uplink="$1"

  ip netns del "${NS_NAME}" >/dev/null 2>&1 || true
  ip link del "${VETH_HOST}" >/dev/null 2>&1 || true

  ip netns add "${NS_NAME}"
  ip link add "${VETH_HOST}" type veth peer name "${VETH_NS}"
  ip link set "${VETH_NS}" netns "${NS_NAME}"

  ip addr add "${HOST_CIDR}" dev "${VETH_HOST}"
  ip link set "${VETH_HOST}" up

  ip netns exec "${NS_NAME}" ip addr add "${NS_CIDR}" dev "${VETH_NS}"
  ip netns exec "${NS_NAME}" ip link set lo up
  ip netns exec "${NS_NAME}" ip link set "${VETH_NS}" up
  ip netns exec "${NS_NAME}" ip route replace default via "${HOST_CIDR%/*}" dev "${VETH_NS}"

  add_host_nat_forward "${uplink}"
}

start_warp_in_netns() {
  ip netns exec "${NS_NAME}" bash -lc "pkill -x warp-svc >/dev/null 2>&1 || true; nohup warp-svc >\"${WARP_LOG}\" 2>&1 &"

  local tries=0
  until ip netns exec "${NS_NAME}" warp-cli --accept-tos status >/dev/null 2>&1; do
    tries=$((tries + 1))
    if [[ ${tries} -gt 25 ]]; then
      echo "warp-cli is not responding in netns. Check ${WARP_LOG}" >&2
      return 1
    fi
    sleep 1
  done

  ip netns exec "${NS_NAME}" warp-cli --accept-tos registration new >/dev/null 2>&1 || true
  ip netns exec "${NS_NAME}" warp-cli --accept-tos mode warp >/dev/null
  ip netns exec "${NS_NAME}" warp-cli --accept-tos connect >/dev/null
}

show_status() {
  echo "=== host side ==="
  ip -br addr show "${VETH_HOST}" || true
  echo
  echo "=== netns links ==="
  ip netns exec "${NS_NAME}" ip -br link || true
  echo
  echo "=== netns route ==="
  ip netns exec "${NS_NAME}" ip -4 route || true
  echo
  echo "=== WARP status in netns ==="
  ip netns exec "${NS_NAME}" warp-cli --accept-tos status || true
}

test_egress() {
  echo "=== netns direct test ==="
  ip netns exec "${NS_NAME}" curl -sS --max-time 15 https://1.1.1.1/cdn-cgi/trace || true
}

do_up() {
  local uplink
  uplink="$(detect_uplink)"
  if [[ -z "${uplink}" ]]; then
    echo "Cannot detect host uplink interface." >&2
    exit 1
  fi

  create_netns_topology "${uplink}"
  start_warp_in_netns
  show_status
  test_egress
}

do_down() {
  local uplink
  uplink="$(detect_uplink || true)"
  if [[ -n "${uplink}" ]]; then
    del_host_nat_forward "${uplink}"
  fi
  ip netns exec "${NS_NAME}" bash -lc "pkill -x warp-svc >/dev/null 2>&1 || true" >/dev/null 2>&1 || true
  ip netns del "${NS_NAME}" >/dev/null 2>&1 || true
  ip link del "${VETH_HOST}" >/dev/null 2>&1 || true
}

usage() {
  cat <<EOF
Usage: $0 {up|down|status|test}

  up      Create netns+veth, start WARP in netns, and test egress
  down    Remove netns+veth and host NAT/FORWARD test rules
  status  Print link/route/WARP status for this test namespace
  test    Run trace curl from inside netns

Environment overrides:
  NS_NAME, VETH_HOST, VETH_NS, HOST_CIDR, NS_CIDR, NS_SUBNET, WARP_LOG
EOF
}

main() {
  require_root
  case "${1:-}" in
    up) do_up ;;
    down) do_down ;;
    status) show_status ;;
    test) test_egress ;;
    *) usage; exit 1 ;;
  esac
}

main "$@"
