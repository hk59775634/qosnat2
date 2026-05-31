/** 防火墙规则表单：UI 模式 ↔ API FilterRule */

export function isRuleAutoManaged(r) {
  if (!r) return false
  if (r.system) return true
  const id = String(r.id || '').trim()
  return id.startsWith('auto-')
}

export function isRuleMutable(r) {
  if (!r) return false
  const id = String(r.id || '').trim()
  if (!id) return false
  if (id.startsWith('sys-')) return false
  return !isRuleAutoManaged(r)
}

export const PROTO_OPTIONS = [
  { value: '', labelKey: 'optAnyProto' },
  { value: 'tcp', labelKey: 'protoTcp' },
  { value: 'udp', labelKey: 'protoUdp' },
  { value: 'icmp', labelKey: 'protoIcmp' },
  { value: 'icmpv6', labelKey: 'protoIcmpv6' },
  { value: 'sctp', labelKey: 'protoSctp' },
  { value: 'udplite', labelKey: 'protoUdplite' },
]

export function validateRuleForm(form, t, aliasNames = []) {
  const errors = []
  const knownAliases = new Set((aliasNames || []).map((n) => String(n).trim()).filter(Boolean))
  if (form.iif_mode === 'custom' && !String(form.iif_custom || '').trim()) {
    errors.push(t('security.firewall.errInIface'))
  }
  if (form.chain === 'forward' && form.oif_mode === 'custom' && !String(form.oif_custom || '').trim()) {
    errors.push(t('security.firewall.errOutIface'))
  }
  if (form.src_mode === 'cidr') {
    const s = String(form.src_cidr || '').trim()
    if (!s) errors.push(t('security.firewall.errSrcCidr'))
    else if (!looksLikeIPv4OrCIDR(s)) errors.push(t('security.firewall.errBadSrcCidr'))
  }
  if (form.src_mode === 'alias' && !String(form.src_alias || '').trim()) {
    errors.push(t('security.firewall.errSrcAlias'))
  } else if (form.src_mode === 'alias' && knownAliases.size && !knownAliases.has(String(form.src_alias).trim())) {
    errors.push(t('security.firewall.errSrcAliasMissing'))
  }
  if (form.dst_mode === 'cidr') {
    const s = String(form.dst_cidr || '').trim()
    if (!s) errors.push(t('security.firewall.errDstCidr'))
    else if (!looksLikeIPv4OrCIDR(s)) errors.push(t('security.firewall.errBadDstCidr'))
  }
  if (form.dst_mode === 'alias' && !String(form.dst_alias || '').trim()) {
    errors.push(t('security.firewall.errDstAlias'))
  } else if (form.dst_mode === 'alias' && knownAliases.size && !knownAliases.has(String(form.dst_alias).trim())) {
    errors.push(t('security.firewall.errDstAliasMissing'))
  }
  if (form.src_port_mode === 'custom' && !validPort(form.src_port_custom)) {
    errors.push(t('security.firewall.errBadSrcPort'))
  }
  if (form.dst_port_mode === 'custom' && !validPort(form.dst_port_custom)) {
    errors.push(t('security.firewall.errBadDstPort'))
  }
  return errors
}

function validPort(raw) {
  const s = String(raw || '').trim()
  if (!s) return false
  const n = parseInt(s, 10)
  return Number.isFinite(n) && n >= 1 && n <= 65535
}

function looksLikeIPv4OrCIDR(s) {
  const m = String(s || '').trim().match(/^(\d{1,3}\.){3}\d{1,3}(\/\d{1,2})?$/)
  if (!m) return false
  const parts = s.split('/')[0].split('.')
  return parts.every((p) => {
    const n = Number(p)
    return n >= 0 && n <= 255
  })
}

export function emptyRuleForm(chain = 'forward') {
  return {
    chain,
    action: 'drop',
    iif_mode: 'any',
    iif_custom: '',
    oif_mode: 'any',
    oif_custom: '',
    proto: '',
    src_mode: 'any',
    src_cidr: '',
    src_alias: '',
    dst_mode: 'any',
    dst_cidr: '',
    dst_alias: '',
    src_port_mode: 'any',
    src_port_custom: '',
    dst_port_mode: 'any',
    dst_port_custom: '',
    comment: '',
    enabled: true,
  }
}

function ifaceFromMode(mode, custom, devLan, devWan) {
  if (String(mode || '').startsWith('dev:')) {
    return String(mode).slice(4).trim()
  }
  switch (mode) {
    case 'lan':
      return devLan || ''
    case 'wan':
      return devWan || ''
    case 'custom':
      return String(custom || '').trim()
    default:
      return ''
  }
}

function modeFromIface(name, devLan, devWan) {
  const n = String(name || '').trim()
  if (!n) return { mode: 'any', custom: '' }
  if (n === devLan) return { mode: 'lan', custom: '' }
  if (n === devWan) return { mode: 'wan', custom: '' }
  return { mode: `dev:${n}`, custom: '' }
}

function portFromRule(port) {
  const n = Number(port)
  if (!n || n <= 0) return { mode: 'any', custom: '' }
  return { mode: 'custom', custom: String(n) }
}

function addrFromRule(alias, cidr) {
  if (alias) return { mode: 'alias', cidr: '', alias }
  if (cidr) return { mode: 'cidr', cidr, alias: '' }
  return { mode: 'any', cidr: '', alias: '' }
}

export function ruleToForm(r, devLan, devWan) {
  const iif = modeFromIface(r.iif, devLan, devWan)
  const oif = modeFromIface(r.oif, devLan, devWan)
  const src = addrFromRule(r.src_alias, r.src_addr)
  const dst = addrFromRule(r.dst_alias, r.dst_addr)
  const sp = portFromRule(r.src_port)
  const dp = portFromRule(r.dst_port)
  return {
    chain: r.chain || 'forward',
    action: r.action || 'drop',
    iif_mode: iif.mode,
    iif_custom: iif.custom,
    oif_mode: oif.mode,
    oif_custom: oif.custom,
    proto: r.proto || '',
    src_mode: src.mode,
    src_cidr: src.cidr,
    src_alias: src.alias,
    dst_mode: dst.mode,
    dst_cidr: dst.cidr,
    dst_alias: dst.alias,
    src_port_mode: sp.mode,
    src_port_custom: sp.custom,
    dst_port_mode: dp.mode,
    dst_port_custom: dp.custom,
    comment: r.comment || '',
    enabled: r.enabled !== false,
  }
}

export function formToPayload(form, devLan, devWan) {
  const chain = form.chain || 'forward'
  const payload = {
    chain,
    action: form.action,
    proto: String(form.proto || '').trim().toLowerCase(),
    comment: String(form.comment || '').trim(),
    enabled: form.enabled !== false,
    iif: ifaceFromMode(form.iif_mode, form.iif_custom, devLan, devWan),
    src_addr: '',
    dst_addr: '',
    src_alias: '',
    dst_alias: '',
    src_port: 0,
    dst_port: 0,
  }
  if (chain === 'forward') {
    payload.oif = ifaceFromMode(form.oif_mode, form.oif_custom, devLan, devWan)
  }
  if (form.src_mode === 'cidr') payload.src_addr = String(form.src_cidr || '').trim()
  if (form.src_mode === 'alias') payload.src_alias = String(form.src_alias || '').trim()
  if (form.dst_mode === 'cidr') payload.dst_addr = String(form.dst_cidr || '').trim()
  if (form.dst_mode === 'alias') payload.dst_alias = String(form.dst_alias || '').trim()
  if (form.src_port_mode === 'custom') {
    const n = parseInt(form.src_port_custom, 10)
    if (n > 0) payload.src_port = n
  }
  if (form.dst_port_mode === 'custom') {
    const n = parseInt(form.dst_port_custom, 10)
    if (n > 0) payload.dst_port = n
  }
  if (!payload.proto) delete payload.proto
  return payload
}
