/** ocserv 限速：RX=服务器收=客户端上行，TX=服务器发=客户端下行（字节/秒） */

export const BPS_PER_MBPS = 125000

export function bpsToMbps(bps) {
  if (!bps || bps <= 0) return 0
  const m = bps / BPS_PER_MBPS
  return Number(m.toFixed(4).replace(/\.?0+$/, ''))
}

export function mbpsToBps(m) {
  const n = Number(m)
  if (!n || n <= 0) return 0
  return Math.round(n * BPS_PER_MBPS)
}

/** 从 ocserv 字段转为 UI：下行 Mbps、上行 Mbps（客户端视角） */
export function clientMbpsFromOcserv(rxBps, txBps) {
  return {
    downMbps: bpsToMbps(txBps),
    upMbps: bpsToMbps(rxBps),
  }
}

/** UI 下行/上行 Mbps → 写入 ocserv 的 rx/tx（字节/秒） */
export function ocservBpsFromClientMbps(downMbps, upMbps) {
  return {
    rx_data_per_sec: mbpsToBps(upMbps),
    tx_data_per_sec: mbpsToBps(downMbps),
  }
}
