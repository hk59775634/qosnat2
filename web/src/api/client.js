const base = ''

export async function request(path, opts = {}) {
  const res = await fetch(`${base}${path}`, {
    credentials: 'include',
    headers: { 'Content-Type': 'application/json', ...(opts.headers || {}) },
    ...opts,
  })
  const text = await res.text()
  let data = null
  if (text) {
    try {
      data = JSON.parse(text)
    } catch {
      data = text
    }
  }
  if (!res.ok) {
    const err = new Error(data?.error || res.statusText || 'request failed')
    err.status = res.status
    err.data = data
    throw err
  }
  return data
}

export const api = {
  health: () => request('/api/v1/health'),
  login: (user, pass) =>
    request('/api/v1/login', { method: 'POST', body: JSON.stringify({ user, pass }) }),
  session: () => request('/api/v1/session'),
  logout: () => request('/api/v1/logout', { method: 'POST' }),

  dashboard: () => request('/api/v1/stats/dashboard'),
  stats: () => request('/api/v1/stats'),

  policyRoutes: {
    list: () => request('/api/v1/nat/policy-routes'),
    add: (cidr) => request('/api/v1/nat/policy-routes', { method: 'POST', body: JSON.stringify({ cidr }) }),
    del: (cidr) => request(`/api/v1/nat/policy-routes?cidr=${encodeURIComponent(cidr)}`, { method: 'DELETE' }),
  },
  sharedIPs: {
    list: () => request('/api/v1/nat/shared-ips'),
    add: (ip) => request('/api/v1/nat/shared-ips', { method: 'POST', body: JSON.stringify({ ip }) }),
  },
  wanForwards: {
    list: () => request('/api/v1/nat/wan-forwards'),
    add: (body) => request('/api/v1/nat/wan-forwards', { method: 'POST', body: JSON.stringify(body) }),
  },
  staticMappings: {
    list: () => request('/api/v1/nat/static-mappings'),
    add: (inner, outer) =>
      request('/api/v1/nat/static-mappings', { method: 'POST', body: JSON.stringify({ inner, outer }) }),
  },

  shaper: {
    wizard: (body) => request('/api/v1/shaper/wizard', { method: 'POST', body: JSON.stringify(body) }),
    profiles: () => request('/api/v1/shaper/profiles'),
    putProfile: (body) => request('/api/v1/shaper/profiles', { method: 'PUT', body: JSON.stringify(body) }),
    delProfile: (cidr) => request(`/api/v1/shaper/profiles?cidr=${encodeURIComponent(cidr)}`, { method: 'DELETE' }),
    hosts: () => request('/api/v1/shaper/hosts'),
    getHost: (ip) => request(`/api/v1/shaper/hosts/${ip}`),
    putHost: (ip, body) =>
      request(`/api/v1/shaper/hosts/${ip}`, { method: 'PUT', body: JSON.stringify(body) }),
    delHost: (ip) => request(`/api/v1/shaper/hosts/${ip}`, { method: 'DELETE' }),
    active: () => request('/api/v1/shaper/active'),
  },
  ebpfMaps: () => request('/api/v1/ebpf/maps'),
  ebpfPrograms: () => request('/api/v1/ebpf/programs'),
  markPolicy: () => request('/api/v1/system/mark-policy'),
  ifaceQueues: () => request('/api/v1/interfaces/queues'),
  request,
  get: (path) => request(path),
  post: (path, body) => request(path, { method: 'POST', body: JSON.stringify(body ?? {}) }),
  put: (path, body) => request(path, { method: 'PUT', body: JSON.stringify(body) }),
  del: (path) => request(path, { method: 'DELETE' }),
}

export function bpsLabel(bps) {
  if (!bps) return '—'
  const mbit = bps / 125000
  if (mbit >= 1000) return `${(mbit / 1000).toFixed(1)} Gbit/s`
  if (mbit >= 1) return `${mbit.toFixed(1)} Mbit/s`
  return `${(bps / 125).toFixed(0)} Kbit/s`
}
