/** 防火墙规则表单：UI 模式 ↔ API FilterRule */

export function isRuleMutable(r) {
  if (!r || r.system) return false
  const id = String(r.id || '').trim()
  if (!id) return false
  if (id.startsWith('sys-') || id.startsWith('auto-')) return false
  return true
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
  return { mode: 'custom', custom: n }
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
