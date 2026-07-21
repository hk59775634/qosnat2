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

export function validateRuleForm(form, t, aliasNames = [], portAliasNames = []) {
  const errors = []
  const knownAliases = new Set((aliasNames || []).map((n) => String(n).trim()).filter(Boolean))
  const knownPorts = new Set((portAliasNames || []).map((n) => String(n).trim()).filter(Boolean))
  if (form.iif_mode === 'custom' && !String(form.iif_custom || '').trim()) {
    errors.push(t('security.firewall.errInIface'))
  }
  if (
    (form.chain === 'forward' || form.chain === 'output') &&
    form.oif_mode === 'custom' &&
    !String(form.oif_custom || '').trim()
  ) {
    errors.push(t('security.firewall.errOutIface'))
  }
  if (form.src_mode === 'cidr') {
    const s = String(form.src_cidr || '').trim()
    if (!s) errors.push(t('security.firewall.errSrcCidr'))
    else if (!looksLikeIPv4OrCIDR(s) && !looksLikeIPv6OrCIDR(s)) errors.push(t('security.firewall.errBadSrcCidr'))
  }
  if (form.src_mode === 'alias' && !String(form.src_alias || '').trim()) {
    errors.push(t('security.firewall.errSrcAlias'))
  } else if (form.src_mode === 'alias' && knownAliases.size && !knownAliases.has(String(form.src_alias).trim())) {
    errors.push(t('security.firewall.errSrcAliasMissing'))
  }
  if (form.dst_mode === 'cidr') {
    const s = String(form.dst_cidr || '').trim()
    if (!s) errors.push(t('security.firewall.errDstCidr'))
    else if (!looksLikeIPv4OrCIDR(s) && !looksLikeIPv6OrCIDR(s)) errors.push(t('security.firewall.errBadDstCidr'))
  }
  if (form.dst_mode === 'alias' && !String(form.dst_alias || '').trim()) {
    errors.push(t('security.firewall.errDstAlias'))
  } else if (form.dst_mode === 'alias' && knownAliases.size && !knownAliases.has(String(form.dst_alias).trim())) {
    errors.push(t('security.firewall.errDstAliasMissing'))
  }
  if (form.src_port_mode === 'custom' && !validPortSpec(form.src_port_custom)) {
    errors.push(t('security.firewall.errBadSrcPort'))
  }
  if (form.dst_port_mode === 'custom' && !validPortSpec(form.dst_port_custom)) {
    errors.push(t('security.firewall.errBadDstPort'))
  }
  if (form.src_port_mode === 'alias' && !String(form.src_port_alias || '').trim()) {
    errors.push(t('security.firewall.errSrcPortAlias'))
  } else if (
    form.src_port_mode === 'alias' &&
    knownPorts.size &&
    !knownPorts.has(String(form.src_port_alias).trim())
  ) {
    errors.push(t('security.firewall.errSrcPortAliasMissing'))
  }
  if (form.dst_port_mode === 'alias' && !String(form.dst_port_alias || '').trim()) {
    errors.push(t('security.firewall.errDstPortAlias'))
  } else if (
    form.dst_port_mode === 'alias' &&
    knownPorts.size &&
    !knownPorts.has(String(form.dst_port_alias).trim())
  ) {
    errors.push(t('security.firewall.errDstPortAliasMissing'))
  }
  return errors
}

function validPort(raw) {
  const s = String(raw || '').trim()
  if (!s) return false
  const n = parseInt(s, 10)
  return Number.isFinite(n) && n >= 1 && n <= 65535
}

function validPortSpec(raw) {
  const s = String(raw || '').trim()
  if (!s) return false
  if (!s.includes(',') && !s.includes('-')) return validPort(s)
  return s.split(',').every((part) => {
    const p = part.trim()
    if (!p) return false
    if (p.includes('-')) {
      const [a, b] = p.split('-')
      return validPort(a) && validPort(b) && parseInt(a, 10) <= parseInt(b, 10)
    }
    return validPort(p)
  })
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

function looksLikeIPv6OrCIDR(s) {
  return String(s || '').includes(':')
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
    src_port_alias: '',
    dst_port_mode: 'any',
    dst_port_custom: '',
    dst_port_alias: '',
    comment: '',
    enabled: true,
    log: false,
    counter: true,
    schedule_id: '',
    wan_link_id: '',
    shaper_down: '',
    shaper_up: '',
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

function portFromRule(port, ports, alias) {
  if (alias) return { mode: 'alias', custom: '', alias }
  const multi = String(ports || '').trim()
  if (multi) return { mode: 'custom', custom: multi, alias: '' }
  const n = Number(port)
  if (!n || n <= 0) return { mode: 'any', custom: '', alias: '' }
  return { mode: 'custom', custom: String(n), alias: '' }
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
  const sp = portFromRule(r.src_port, r.src_ports, r.src_port_alias)
  const dp = portFromRule(r.dst_port, r.dst_ports, r.dst_port_alias)
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
    src_port_alias: sp.alias,
    dst_port_mode: dp.mode,
    dst_port_custom: dp.custom,
    dst_port_alias: dp.alias,
    comment: r.comment || '',
    enabled: r.enabled !== false,
    log: !!r.log,
    counter: !!r.counter,
    schedule_id: r.schedule_id || '',
    wan_link_id: r.wan_link_id || '',
    shaper_down: r.shaper_down || '',
    shaper_up: r.shaper_up || '',
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
    log: !!form.log,
    counter: !!form.counter,
    schedule_id: String(form.schedule_id || '').trim(),
    wan_link_id: String(form.wan_link_id || '').trim(),
    shaper_down: String(form.shaper_down || '').trim(),
    shaper_up: String(form.shaper_up || '').trim(),
    iif: '',
    oif: '',
    src_addr: '',
    dst_addr: '',
    src_alias: '',
    dst_alias: '',
    src_port: 0,
    dst_port: 0,
    src_ports: '',
    dst_ports: '',
    src_port_alias: '',
    dst_port_alias: '',
  }
  if (chain === 'input') {
    payload.iif = ifaceFromMode(form.iif_mode, form.iif_custom, devLan, devWan)
  } else if (chain === 'output') {
    payload.oif = ifaceFromMode(form.oif_mode, form.oif_custom, devLan, devWan)
  } else {
    payload.iif = ifaceFromMode(form.iif_mode, form.iif_custom, devLan, devWan)
    payload.oif = ifaceFromMode(form.oif_mode, form.oif_custom, devLan, devWan)
  }
  if (form.src_mode === 'cidr') payload.src_addr = String(form.src_cidr || '').trim()
  if (form.src_mode === 'alias') payload.src_alias = String(form.src_alias || '').trim()
  if (form.dst_mode === 'cidr') payload.dst_addr = String(form.dst_cidr || '').trim()
  if (form.dst_mode === 'alias') payload.dst_alias = String(form.dst_alias || '').trim()
  if (form.src_port_mode === 'alias') {
    payload.src_port_alias = String(form.src_port_alias || '').trim()
  } else if (form.src_port_mode === 'custom') {
    const raw = String(form.src_port_custom || '').trim()
    if (raw.includes(',') || raw.includes('-')) payload.src_ports = raw
    else {
      const n = parseInt(raw, 10)
      if (n > 0) payload.src_port = n
    }
  }
  if (form.dst_port_mode === 'alias') {
    payload.dst_port_alias = String(form.dst_port_alias || '').trim()
  } else if (form.dst_port_mode === 'custom') {
    const raw = String(form.dst_port_custom || '').trim()
    if (raw.includes(',') || raw.includes('-')) payload.dst_ports = raw
    else {
      const n = parseInt(raw, 10)
      if (n > 0) payload.dst_port = n
    }
  }
  if (!payload.proto) delete payload.proto
  if (!payload.schedule_id) delete payload.schedule_id
  if (!payload.wan_link_id) delete payload.wan_link_id
  if (!payload.shaper_down) delete payload.shaper_down
  if (!payload.shaper_up) delete payload.shaper_up
  return payload
}
