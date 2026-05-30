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
  ensure_test_nft_rules "${uplink}"
}

ensure_test_nft_rules() {
  local uplink="$1"
  # qosnat 默认 forward 链会 drop 非 qwp0 流量；测试 veth 需临时放行。
  if nft list chain inet qosnat forward >/dev/null 2>&1; then
    nft list chain inet qosnat forward 2>/dev/null | grep -q "iifname \"${VETH_HOST}\"" || \
      nft insert rule inet qosnat forward iifname "${VETH_HOST}" accept comment "warp-netns-test" || true
    nft list chain inet qosnat forward 2>/dev/null | grep -q "oifname \"${VETH_HOST}\"" || \
      nft insert rule inet qosnat forward oifname "${VETH_HOST}" accept comment "warp-netns-test" || true
  fi
  nft add table ip qosnat2_warp_test >/dev/null 2>&1 || true
  nft 'add chain ip qosnat2_warp_test postrouting { type nat hook postrouting priority srcnat; policy accept; }' >/dev/null 2>&1 || true
  local masq_rule="ip saddr ${NS_SUBNET} oifname \"${uplink}\" masquerade"
  nft list chain ip qosnat2_warp_test postrouting 2>/dev/null | grep -qF "${masq_rule}" || \
    nft add rule ip qosnat2_warp_test postrouting ip saddr "${NS_SUBNET}" oifname "${uplink}" masquerade || true
}

remove_test_nft_rules() {
  local uplink="$1"
  nft -a list chain inet qosnat forward 2>/dev/null | awk -v dev="${VETH_HOST}" '
    /warp-netns-test/ && $0 ~ dev { if (match($0,/handle ([0-9]+)/,a)) print a[1] }
  ' | sort -rn | while read -r h; do
    [[ -n "${h}" ]] && nft delete rule inet qosnat forward handle "${h}" 2>/dev/null || true
  done
  nft delete table ip qosnat2_warp_test >/dev/null 2>&1 || true
}

del_host_nat_forward() {
  local uplink="$1"
  remove_test_nft_rules "${uplink}"
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

  tries=0
  until ip netns exec "${NS_NAME}" warp-cli --accept-tos status 2>/dev/null | grep -qi '^Status update: Connected'; do
    tries=$((tries + 1))
    if [[ ${tries} -gt 45 ]]; then
      echo "WARP did not reach Connected in netns. Check ${WARP_LOG}" >&2
      ip netns exec "${NS_NAME}" warp-cli --accept-tos status || true
      return 1
    fi
    sleep 2
  done
}

warp_connected() {
  local st
  st="$(ip netns exec "${NS_NAME}" warp-cli --accept-tos status 2>/dev/null || true)"
  local low
  low="$(printf '%s' "${st}" | tr '[:upper:]' '[:lower:]')"
  [[ "${low}" == *"status update: connected"* ]] && return 0
  [[ "${low}" == *"connected"* && "${low}" != *"disconnected"* && "${low}" != *"unable to connect"* && "${low}" != *"no network"* ]]
}

netns_healthy() {
  ip netns exec "${NS_NAME}" true >/dev/null 2>&1 && \
    ip -d -o link show "${VETH_HOST}" 2>/dev/null | grep -qv 'link-netnsid 0'
}

apply_policy_phase() {
  local uplink="$1"
  echo "=== phase 2: apply host policies (no netns reset, no nft flush) ==="
  ensure_test_nft_rules "${uplink}"
  nft add table ip qosnat2_warp >/dev/null 2>&1 || true
  nft 'add chain ip qosnat2_warp postrouting { type nat hook postrouting priority srcnat; policy accept; }' >/dev/null 2>&1 || true
  nft add rule ip qosnat2_warp postrouting ip saddr "${NS_SUBNET}" oifname "${uplink}" masquerade >/dev/null 2>&1 || true
  ip route flush table 202 >/dev/null 2>&1 || true
  ip route replace default dev "${VETH_HOST}" table 202
  ip rule del lookup 202 priority 32000 >/dev/null 2>&1 || true
  ip rule add lookup 202 priority 32000 >/dev/null 2>&1 || true
}

verify_after_policy() {
  echo "=== verify WARP after policy apply ==="
  if ! netns_healthy; then
    echo "FAIL: netns unhealthy (ip netns exec or veth peer broken)" >&2
    ip -d link show "${VETH_HOST}" 2>&1 | head -3 || true
    return 1
  fi
  if ! warp_connected; then
    echo "FAIL: WARP not connected after policy apply" >&2
    ip netns exec "${NS_NAME}" warp-cli --accept-tos status || true
    return 1
  fi
  echo "OK: WARP Connected, netns healthy"
  ip netns exec "${NS_NAME}" warp-cli --accept-tos status | head -2
  return 0
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

  echo "=== phase 1: netns + WARP tunnel ==="
  create_netns_topology "${uplink}"
  start_warp_in_netns
  show_status
  echo "=== phase 1 verify ==="
  verify_after_policy
  apply_policy_phase "${uplink}"
  verify_after_policy
  test_egress
}

do_policy_test() {
  local uplink
  uplink="$(detect_uplink)"
  if [[ -z "${uplink}" ]]; then
    echo "Cannot detect host uplink interface." >&2
    exit 1
  fi
  if ! ip netns list | grep -qx "${NS_NAME}"; then
    echo "Netns ${NS_NAME} not found. Run: $0 up" >&2
    exit 1
  fi
  if ! warp_connected; then
    echo "WARP not connected. Run: $0 up" >&2
    exit 1
  fi
  apply_policy_phase "${uplink}"
  verify_after_policy
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
Usage: $0 {up|down|status|test|policy-test}

  up           Phase 1: netns+veth+WARP; Phase 2: apply policies; verify tunnel stays up
  down         Remove netns+veth and host NAT/FORWARD test rules
  status       Print link/route/WARP status for this test namespace
  test         Run trace curl from inside netns
  policy-test  Re-run phase 2 policy apply on an existing connected test netns

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
    policy-test) do_policy_test ;;
    *) usage; exit 1 ;;
  esac
}

main "$@"
