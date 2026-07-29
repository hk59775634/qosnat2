/** Egress policy form helpers (shared create/edit + API body). */

export const WAN_TAB_ALL = ''

export function emptyEgressForm(overrides = {}) {
  return {
    name: '',
    src_iface: '',
    src_mode: 'cidr',
    src_cidr: '',
    src_alias: '',
    dst_mode: 'none',
    dst_cidr: '',
    dst_alias: '',
    wan_link_id: '',
    snat_ip: '',
    no_snat: false,
    priority: 100,
    enabled: true,
    ...overrides,
  }
}

/** Prefer primary DEV_WAN link, else first enabled, else first link. */
export function pickOptimalWanLinkId(links, devWan) {
  const list = links || []
  const enabled = list.filter((w) => w.enabled)
  const pool = enabled.length ? enabled : list
  if (!pool.length) return ''
  const wan = String(devWan || '').trim()
  if (wan) {
    const hit = pool.find((w) => w.device === wan)
    if (hit) return hit.id
  }
  // Prefer non-policy-only physical uplinks for default SNAT egress.
  const normal = pool.find((w) => !w.policy_only && !w.iface_managed && !w.warp_managed)
  if (normal) return normal.id
  return pool[0].id
}

/** When WAN is policy-only / managed tunnel, default to NoSNAT. */
export function optimalNoSnatForWan(link) {
  if (!link) return false
  return !!(link.policy_only || link.iface_managed || link.warp_managed)
}

export function buildEgressBody(f) {
  const body = {
    name: f.name,
    wan_link_id: f.wan_link_id,
    priority: f.priority,
    enabled: f.enabled,
    no_snat: !!f.no_snat,
  }
  if (!f.no_snat && f.snat_ip) body.snat_ip = f.snat_ip
  if (f.src_iface) body.src_iface = f.src_iface.trim()
  if (f.src_mode === 'cidr' && f.src_cidr) body.src_cidr = f.src_cidr.trim()
  if (f.src_mode === 'alias' && f.src_alias) body.src_alias = f.src_alias
  if (f.dst_mode === 'cidr' && f.dst_cidr) body.dst_cidr = f.dst_cidr.trim()
  if (f.dst_mode === 'alias' && f.dst_alias) body.dst_alias = f.dst_alias
  return body
}

export function policyToEditForm(p) {
  let src_mode = 'none'
  let dst_mode = 'none'
  let src_cidr = ''
  let src_alias = ''
  let dst_cidr = ''
  let dst_alias = ''
  if (p.src_alias) {
    src_mode = 'alias'
    src_alias = p.src_alias
  } else if (p.src_cidr) {
    src_mode = 'cidr'
    src_cidr = p.src_cidr
  } else if (p.cidr && p.match !== 'destination') {
    src_mode = 'cidr'
    src_cidr = p.cidr
  }
  if (p.dst_alias) {
    dst_mode = 'alias'
    dst_alias = p.dst_alias
  } else if (p.dst_cidr) {
    dst_mode = 'cidr'
    dst_cidr = p.dst_cidr
  } else if (p.cidr && p.match === 'destination') {
    dst_mode = 'cidr'
    dst_cidr = p.cidr
  }
  return emptyEgressForm({
    name: p.name || '',
    src_iface: p.src_iface || '',
    src_mode,
    src_cidr,
    src_alias,
    dst_mode,
    dst_cidr,
    dst_alias,
    wan_link_id: p.wan_link_id,
    snat_ip: p.snat_ip || '',
    no_snat: !!p.no_snat,
    priority: p.priority,
    enabled: p.enabled,
  })
}

export function egressEndpointsLabel(p, labels = {}) {
  const iif = labels.iif || 'iif'
  const src = labels.src || 'src'
  const dst = labels.dst || 'dst'
  const parts = []
  if (p.src_iface) parts.push(`${iif}:${p.src_iface}`)
  if (p.src_alias) parts.push(`${src}:@${p.src_alias}`)
  else if (p.src_cidr) parts.push(`${src}:${p.src_cidr}`)
  else if (p.cidr && p.match !== 'destination') parts.push(`${src}:${p.cidr}`)
  if (p.dst_alias) parts.push(`${dst}:@${p.dst_alias}`)
  else if (p.dst_cidr) parts.push(`${dst}:${p.dst_cidr}`)
  else if (p.cidr && (p.match === 'destination' || (!p.src_cidr && !p.src_alias))) {
    parts.push(`${dst}:${p.cidr}`)
  }
  return parts.join(' · ') || '—'
}

export function isAutoManagedEgress(p) {
  const id = String(p?.id || '')
  return id.startsWith('auto-iface-pr-') || id.startsWith('auto-fw-gw-')
}

export function wanTabLabel(w) {
  if (!w) return ''
  const name = w.name || w.id
  const dev = w.device ? ` (${w.device})` : ''
  return `${name}${dev}`
}
