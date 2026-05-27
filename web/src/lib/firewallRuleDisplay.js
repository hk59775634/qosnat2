/** 将 FilterRule 格式化为 pfSense 风格展示字段 */

export function formatAny(text, fallback = '*') {
  const s = String(text || '').trim()
  return s || fallback
}

export function formatSource(r) {
  if (r.src_alias) return { kind: 'alias', label: r.src_alias }
  if (r.src_addr) return { kind: 'cidr', label: r.src_addr }
  return { kind: 'any', label: '*' }
}

export function formatDestination(r) {
  if (r.dst_alias) return { kind: 'alias', label: r.dst_alias }
  if (r.dst_addr) return { kind: 'cidr', label: r.dst_addr }
  return { kind: 'any', label: '*' }
}

export function formatIface(name, devLan, devWan) {
  const n = String(name || '').trim()
  if (!n) return { name: '—', roleKey: '' }
  if (n === devLan) return { name: n, roleKey: 'lan' }
  if (n === devWan) return { name: n, roleKey: 'wan' }
  return { name: n, roleKey: '' }
}

export function actionMeta(action) {
  switch (String(action || '').toLowerCase()) {
    case 'accept':
      return { key: 'accept', badge: 'pass', class: 'fw-action-pass' }
    case 'reject':
      return { key: 'reject', badge: 'reject', class: 'fw-action-reject' }
    case 'drop':
    default:
      return { key: 'drop', badge: 'block', class: 'fw-action-block' }
  }
}

export function formatProto(proto) {
  const p = String(proto || '').trim().toLowerCase()
  if (!p || p === 'all') return '*'
  return p.toUpperCase()
}

export function formatPort(port) {
  const n = Number(port)
  if (!n || n <= 0) return '*'
  return String(n)
}

/** 系统内置规则（与 internal/nft/nft.go 生成顺序一致，只读展示） */
export function builtinRulesForChain(chain, devLan, devWan, ctx, t) {
  const sys = (id, action, summary, detail) => ({
    id,
    system: true,
    action,
    summary,
    detail,
    enabled: true,
  })

  const adminPort = ctx?.adminPort || ''
  const vpn = ctx?.vpn || {}

  if (chain === 'forward') {
    const rows = [sys('sys-est', 'accept', t('security.firewall.sysEstablished'), '')]
    if (devLan && devWan) {
      rows.push(
        sys(
          'sys-lan-wan',
          'accept',
          t('security.firewall.sysLanToWan', { lan: devLan, wan: devWan }),
          '',
        ),
        sys(
          'sys-wan-lan',
          'accept',
          t('security.firewall.sysWanToLan', { lan: devLan, wan: devWan }),
          '',
        ),
        sys(
          'sys-asym',
          'drop',
          t('security.firewall.sysAsymmetric'),
          t('security.firewall.sysAsymmetricDetail', { lan: devLan, wan: devWan }),
        ),
      )
    }
    return rows
  }

  if (chain === 'input') {
    const rows = [
      sys('sys-lo', 'accept', t('security.firewall.sysLoopback'), 'lo'),
      sys('sys-est-in', 'accept', t('security.firewall.sysEstablished'), ''),
    ]
    if (devLan) {
      rows.push(sys('sys-lan-in', 'accept', t('security.firewall.sysLanInput', { lan: devLan }), ''))
    }
    if (devWan && adminPort) {
      rows.push(
        sys(
          'sys-wan-admin',
          'accept',
          t('security.firewall.sysWanAdmin', { wan: devWan, port: adminPort }),
          '',
        ),
      )
    }
    if (devWan && vpn.ocserv_enabled) {
      rows.push(
        sys(
          'sys-wan-ocserv-tcp',
          'accept',
          t('security.firewall.sysWanOcservTcp', { wan: devWan, port: vpn.ocserv_tcp_port }),
          '',
        ),
      )
      if (vpn.ocserv_udp_port) {
        rows.push(
          sys(
            'sys-wan-ocserv-udp',
            'accept',
            t('security.firewall.sysWanOcservUdp', { wan: devWan, port: vpn.ocserv_udp_port }),
            '',
          ),
        )
      }
    }
    const wgPorts =
      Array.isArray(vpn.wireguard_ports) && vpn.wireguard_ports.length > 0
        ? vpn.wireguard_ports
        : vpn.wireguard_port
          ? [vpn.wireguard_port]
          : []
    if (devWan && wgPorts.length > 0) {
      rows.push(
        sys(
          'sys-wan-wg',
          'accept',
          t('security.firewall.sysWanWireGuard', { wan: devWan, port: wgPorts.join(', ') }),
          '',
        ),
      )
    }
    if (devWan) {
      rows.push(
        sys(
          'sys-wan-drop',
          'drop',
          t('security.firewall.sysWanDeny', { wan: devWan }),
          t('security.firewall.sysWanDenyDetail'),
        ),
      )
    }
    return rows
  }

  return []
}

export function rulesForChain(rules, chain) {
  return (rules || []).filter((r) => String(r.chain).toLowerCase() === chain)
}

/** 在同链内拖动排序后，写回完整 rules 数组（保持其他链规则原位） */
export function mergeChainReorder(allRules, chain, reorderedSubset) {
  const result = []
  let i = 0
  for (const r of allRules || []) {
    if (String(r.chain).toLowerCase() === chain) {
      result.push(reorderedSubset[i++])
    } else {
      result.push(r)
    }
  }
  return result
}
