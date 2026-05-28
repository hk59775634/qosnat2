/** pfSense 风格：按网卡筛选防火墙规则 */

import { isRuleMutable } from './firewallRuleForm'

export const IFACE_ALL = ''
export const IFACE_FLOATING = '__floating__'

/** @param {string} iface 选中的网卡名；空 = 全部 */
export function ruleTouchesIface(rule, iface, chain) {
  if (!iface || iface === IFACE_ALL) return true
  const iif = String(rule?.iif || '').trim()
  const oif = String(rule?.oif || '').trim()
  const ch = String(chain || rule?.chain || '').toLowerCase()

  if (iface === IFACE_FLOATING) {
    if (ch === 'input') return !iif
    return !iif && !oif
  }

  if (ch === 'input') return iif === iface
  return iif === iface || oif === iface
}

/** 内置展示行是否属于当前网卡视图 */
export function builtinTouchesIface(br, iface, devLan, devWan, wanDevices = []) {
  if (!iface || iface === IFACE_ALL) return true
  const id = String(br?.id || '')
  const wans = new Set([devWan, ...wanDevices].filter(Boolean))

  if (iface === IFACE_FLOATING) return false

  switch (id) {
    case 'sys-est':
    case 'sys-est-in':
      return false
    case 'sys-lo':
      return iface === 'lo'
    case 'sys-lan-wan':
    case 'sys-wan-lan':
    case 'sys-asym':
      return iface === devLan || wans.has(iface)
    case 'sys-lan-in':
      return iface === devLan
    case 'sys-acme-80':
      return wans.has(iface)
    case 'sys-ifb0':
      return iface === 'ifb0'
    case 'sys-vpn-wg':
    case 'sys-vpn-ocserv':
    case 'sys-forward-vpn-wg':
    case 'sys-forward-vpn-ocserv':
    case 'sys-forward-default-deny':
    case 'sys-default-deny':
      return false
    default: {
      const id = String(br?.id || '')
      if (id.startsWith('sys-asym-')) {
        const wan = id.slice('sys-asym-'.length)
        return iface === wan || iface === devLan
      }
      if (id.startsWith('sys-lan-wan-')) {
        const wan = id.slice('sys-lan-wan-'.length)
        return iface === devLan || iface === wan
      }
      if (id.startsWith('sys-wan-lan-')) {
        const wan = id.slice('sys-wan-lan-'.length)
        return iface === wan || iface === devLan
      }
      return true
    }
  }
}

export function filterRulesByIface(rules, iface, chain) {
  return (rules || []).filter((r) => ruleTouchesIface(r, iface, chain))
}

export function filterBuiltinByIface(rows, iface, devLan, devWan, wanDevices) {
  return (rows || []).filter((br) => builtinTouchesIface(br, iface, devLan, devWan, wanDevices))
}

/** 同链、同网卡视图内重排（隐藏规则保持原位） */
export function mergeChainReorderForIface(allRules, chain, iface, reorderedVisibleSubset) {
  const result = []
  let vi = 0
  for (const r of allRules || []) {
    if (String(r.chain).toLowerCase() !== chain) {
      result.push(r)
      continue
    }
    if (!isRuleMutable(r)) {
      result.push(r)
      continue
    }
    if (!ruleTouchesIface(r, iface, chain)) {
      result.push(r)
      continue
    }
    result.push(reorderedVisibleSubset[vi++])
  }
  return result
}

/** 从 API interfaces 列表提取额外 WAN 设备名 */
export function wanDeviceNames(ifaceList, devWan) {
  const wans = new Set()
  for (const x of ifaceList || []) {
    if (x.role === 'WAN' && x.name && x.name !== devWan) wans.add(x.name)
  }
  return [...wans]
}

/** 全部 WAN 设备名（主 WAN + 多 WAN 链路），稳定排序 */
export function allWanDeviceNames(ifaceList, devWan) {
  const wans = new Set()
  if (devWan) wans.add(devWan)
  for (const x of ifaceList || []) {
    if (x.role === 'WAN' && x.name) wans.add(x.name)
  }
  return [...wans].sort()
}

/** 网卡 Tab 显示标签 */
export function ifaceTabLabel(item, t) {
  if (!item?.name) return ''
  const role = item.role || ''
  const roleLabel =
    role === 'LAN'
      ? t('security.firewall.roleLan')
      : role === 'WAN'
        ? t('security.firewall.roleWan')
        : role === 'VLAN'
          ? t('security.firewall.roleVlan')
          : role === 'OPT'
            ? t('security.firewall.roleOpt')
            : ''
  const base = item.label ? `${item.label} (${item.name})` : item.name
  return roleLabel ? `${base} · ${roleLabel}` : base
}

/** 根据选中 Tab 预填表单接口模式 */
export function ifaceFormDefaults(activeIface, devLan, devWan, ifaceList) {
  if (!activeIface || activeIface === IFACE_ALL || activeIface === IFACE_FLOATING) {
    return { iif_mode: 'any', oif_mode: 'any' }
  }
  if (activeIface === devLan) return { iif_mode: 'lan', oif_mode: 'any' }
  if (activeIface === devWan) return { iif_mode: 'wan', oif_mode: 'any' }
  const hit = (ifaceList || []).find((x) => x.name === activeIface)
  if (hit?.role === 'WAN' && activeIface === devWan) return { iif_mode: 'wan', oif_mode: 'any' }
  return { iif_mode: `dev:${activeIface}`, oif_mode: 'any' }
}
