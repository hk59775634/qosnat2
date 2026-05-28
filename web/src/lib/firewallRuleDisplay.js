/** 将 FilterRule 格式化为 pfSense 风格展示字段 */

import { allWanDeviceNames } from './firewallIface'
import { isRuleAutoManaged, isRuleMutable } from './firewallRuleForm'

export { isRuleAutoManaged, isRuleMutable }

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

export function formatIface(name, devLan, devWan, wanDevices = []) {
  const n = String(name || '').trim()
  if (!n) return { name: '—', roleKey: '' }
  if (n === devLan) return { name: n, roleKey: 'lan' }
  if (n === devWan || (wanDevices.length && wanDevices.includes(n))) {
    return { name: n, roleKey: 'wan' }
  }
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

  const ifaceList = ctx?.ifaceList || []
  const wanDevs = allWanDeviceNames(ifaceList, devWan)

  if (chain === 'forward') {
    const rows = [sys('sys-est', 'accept', t('security.firewall.sysEstablished'), '')]
    if (devLan) {
      for (const wan of wanDevs) {
        if (!wan || wan === devLan) continue
        rows.push(
          sys(
            `sys-lan-wan-${wan}`,
            'accept',
            t('security.firewall.sysLanToWan', { lan: devLan, wan }),
            '',
          ),
          sys(
            `sys-wan-lan-${wan}`,
            'accept',
            t('security.firewall.sysWanToLan', { lan: devLan, wan }),
            '',
          ),
          sys(
            `sys-asym-${wan}`,
            'drop',
            t('security.firewall.sysAsymmetric'),
            t('security.firewall.sysAsymmetricDetail', { lan: devLan, wan }),
          ),
        )
      }
    }
    rows.push(
      sys('sys-forward-vpn-wg', 'accept', t('security.firewall.sysForwardVpnWg'), t('security.firewall.sysForwardVpnWgDetail')),
      sys(
        'sys-forward-vpn-ocserv',
        'accept',
        t('security.firewall.sysForwardVpnOcserv'),
        t('security.firewall.sysForwardVpnOcservDetail'),
      ),
      sys(
        'sys-forward-default-deny',
        'drop',
        t('security.firewall.sysForwardDefaultDeny'),
        t('security.firewall.sysForwardDefaultDenyDetail'),
      ),
    )
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
    rows.push(
      sys('sys-ifb0', 'accept', t('security.firewall.sysIfb0'), t('security.firewall.sysIfb0Detail')),
      sys('sys-vpn-wg', 'accept', t('security.firewall.sysVpnWg'), t('security.firewall.sysVpnWgDetail')),
      sys(
        'sys-vpn-ocserv',
        'accept',
        t('security.firewall.sysVpnOcserv'),
        t('security.firewall.sysVpnOcservDetail'),
      ),
    )
    if (ctx?.acmeTempAllow) {
      rows.push(
        sys(
          'sys-acme-80',
          'accept',
          t('security.firewall.sysAcmeHttp01'),
          t('security.firewall.sysAcmeHttp01Detail'),
        ),
      )
    }
    rows.push(
      sys(
        'sys-default-deny',
        'drop',
        t('security.firewall.sysDefaultDeny'),
        t('security.firewall.sysDefaultDenyDetail'),
      ),
    )
    return rows
  }

  return []
}

export function rulesForChain(rules, chain) {
  return (rules || []).filter((r) => String(r.chain).toLowerCase() === chain)
}

export function userRulesForChain(rules, chain) {
  return rulesForChain(rules, chain).filter((r) => isRuleMutable(r))
}

export function autoRulesForChain(rules, chain) {
  return rulesForChain(rules, chain).filter((r) => isRuleAutoManaged(r))
}

export function ruleMatchesSearch(r, q, devLan, devWan) {
  if (!q) return true
  const needle = q.toLowerCase()
  const parts = [
    r.id,
    r.comment,
    r.action,
    r.chain,
    r.proto,
    r.iif,
    r.oif,
    r.src_addr,
    r.dst_addr,
    r.src_alias,
    r.dst_alias,
    String(r.src_port || ''),
    String(r.dst_port || ''),
    formatSource(r).label,
    formatDestination(r).label,
    formatIface(r.iif, devLan, devWan).name,
    formatIface(r.oif, devLan, devWan).name,
  ]
  return parts.some((p) => String(p || '').toLowerCase().includes(needle))
}

/** 在同链内拖动排序后，写回完整 rules 数组（自动规则保持原位） */
export function mergeChainReorder(allRules, chain, reorderedUserSubset) {
  const result = []
  let ui = 0
  for (const r of allRules || []) {
    if (String(r.chain).toLowerCase() !== chain) {
      result.push(r)
      continue
    }
    if (!isRuleMutable(r)) {
      result.push(r)
    } else {
      result.push(reorderedUserSubset[ui++])
    }
  }
  return result
}

/** 规则摘要（详情面板） */
export function ruleDetailLines(r, devLan, devWan, t) {
  const inIf = formatIface(r.iif, devLan, devWan)
  const outIf = formatIface(r.oif, devLan, devWan)
  return [
    { k: t('security.firewall.detailId'), v: r.id || '—' },
    { k: t('security.firewall.chain'), v: r.chain },
    { k: t('security.firewall.action'), v: r.action },
    { k: t('security.firewall.colIn'), v: inIf.name },
    { k: t('security.firewall.colOut'), v: outIf.name },
    { k: t('security.firewall.colProto'), v: formatProto(r.proto) },
    { k: t('security.firewall.colSource'), v: formatSource(r).label },
    { k: t('security.firewall.colSPort'), v: formatPort(r.src_port) },
    { k: t('security.firewall.colDest'), v: formatDestination(r).label },
    { k: t('security.firewall.colDPort'), v: formatPort(r.dst_port) },
    { k: t('security.firewall.colDescription'), v: r.comment || '—' },
    {
      k: t('security.firewall.enabled'),
      v: r.enabled !== false ? t('security.firewall.statusOn') : t('security.firewall.statusOff'),
    },
  ]
}
