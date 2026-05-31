import { isApplyFailure, setApplyAlert } from '@/composables/useApplyAlert'

const base = ''

export async function request(path, opts = {}) {
  let res
  try {
    res = await fetch(`${base}${path}`, {
      credentials: 'include',
      headers: { 'Content-Type': 'application/json', ...(opts.headers || {}) },
      ...opts,
    })
  } catch (e) {
    const err = new Error(
      e?.name === 'AbortError'
        ? 'request timeout'
        : 'network error (connection closed or blocked before server responded; WARP operations may still be running in background)',
    )
    err.cause = e
    throw err
  }
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
    if (isApplyFailure(res.status, data)) {
      const code = data?.code ? `[${data.code}] ` : ''
      setApplyAlert(`${code}${err.message}`)
    }
    throw err
  }
  return data
}

export const api = {
  health: () => request('/api/v1/health'),
  setup: {
    status: () => request('/api/v1/setup/status'),
    interfaces: () => request('/api/v1/setup/interfaces'),
    complete: (body) =>
      request('/api/v1/setup/complete', { method: 'POST', body: JSON.stringify(body) }),
  },
  login: (user, pass) =>
    request('/api/v1/login', { method: 'POST', body: JSON.stringify({ user, pass }) }),
  session: () => request('/api/v1/session'),
  logout: () => request('/api/v1/logout', { method: 'POST' }),

  dashboard: () => request('/api/v1/stats/dashboard'),
  metrics: {
    ops: () => request('/api/v1/metrics/ops'),
  },

  policyRoutes: {
    list: () => request('/api/v1/nat/policy-routes'),
    add: (cidr) => request('/api/v1/nat/policy-routes', { method: 'POST', body: JSON.stringify({ cidr }) }),
    del: (cidr) => request(`/api/v1/nat/policy-routes?cidr=${encodeURIComponent(cidr)}`, { method: 'DELETE' }),
  },
  sharedIPs: {
    list: () => request('/api/v1/nat/shared-ips'),
    add: (ip) => request('/api/v1/nat/shared-ips', { method: 'POST', body: JSON.stringify({ ip }) }),
    del: (ip) => request(`/api/v1/nat/shared-ips?ip=${encodeURIComponent(ip)}`, { method: 'DELETE' }),
  },
  wanForwards: {
    list: () => request('/api/v1/nat/wan-forwards'),
    add: (body) => request('/api/v1/nat/wan-forwards', { method: 'POST', body: JSON.stringify(body) }),
    del: (id) => request(`/api/v1/nat/wan-forwards?id=${encodeURIComponent(id)}`, { method: 'DELETE' }),
  },
  staticMappings: {
    list: () => request('/api/v1/nat/static-mappings'),
    add: (inner, outer) =>
      request('/api/v1/nat/static-mappings', { method: 'POST', body: JSON.stringify({ inner, outer }) }),
    del: (inner) =>
      request(`/api/v1/nat/static-mappings?inner=${encodeURIComponent(inner)}`, { method: 'DELETE' }),
  },
  prefixMappings: {
    list: () => request('/api/v1/nat/prefix-mappings'),
    add: (inner, outer) =>
      request('/api/v1/nat/prefix-mappings', { method: 'POST', body: JSON.stringify({ inner, outer }) }),
    del: (inner) =>
      request(`/api/v1/nat/prefix-mappings?inner=${encodeURIComponent(inner)}`, { method: 'DELETE' }),
  },
  nat: {
    summary: () => request('/api/v1/nat'),
    nptv6: {
      get: () => request('/api/v1/nat/nptv6'),
      put: (body) => request('/api/v1/nat/nptv6', { method: 'PUT', body: JSON.stringify(body) }),
    },
    nat64: {
      get: () => request('/api/v1/nat/nat64'),
      put: (body) => request('/api/v1/nat/nat64', { method: 'PUT', body: JSON.stringify(body) }),
    },
    dns64: {
      get: () => request('/api/v1/nat/dns64'),
      put: (body) => request('/api/v1/nat/dns64', { method: 'PUT', body: JSON.stringify(body) }),
    },
  },

  ebpfMaps: () => request('/api/v1/ebpf/maps'),
  ebpfPrograms: () => request('/api/v1/ebpf/programs'),
  markPolicy: () => request('/api/v1/system/mark-policy'),
    system: {
    general: {
      get: () => request('/api/v1/system/general'),
      put: (body) => request('/api/v1/system/general', { method: 'PUT', body: JSON.stringify(body) }),
    },
    state: {
      exportUrl: () => '/api/v1/system/state/export',
      import: (body) =>
        request('/api/v1/system/state/import', { method: 'POST', body: JSON.stringify(body) }),
    },
    version: {
      get: () => request('/api/v1/system/version'),
      switchVerify: (body) =>
        request('/api/v1/system/version/switch/verify', {
          method: 'POST',
          body: JSON.stringify(body),
        }),
      switch: (body) =>
        request('/api/v1/system/version/switch', { method: 'POST', body: JSON.stringify(body) }),
      switchStatus: () => request('/api/v1/system/version/switch/status'),
    },
    tuning: {
      get: () => request('/api/v1/system/tuning'),
      put: (body) => request('/api/v1/system/tuning', { method: 'PUT', body: JSON.stringify(body) }),
    },
    audit: {
      list: () => request('/api/v1/system/audit'),
    },
    notifications: {
      list: () => request('/api/v1/system/notifications'),
      dismiss: (ids) =>
        request('/api/v1/system/notifications', {
          method: 'POST',
          body: JSON.stringify({ ids }),
        }),
      dismissAll: () =>
        request('/api/v1/system/notifications', {
          method: 'POST',
          body: JSON.stringify({ dismiss_all: true }),
        }),
    },
    certificates: {
      list: () => request('/api/v1/system/certificates'),
      create: (body) =>
        request('/api/v1/system/certificates', { method: 'POST', body: JSON.stringify(body) }),
      renew: (id) =>
        request('/api/v1/system/certificates/renew', { method: 'POST', body: JSON.stringify({ id }) }),
      setAutoRenew: (id, enabled) =>
        request('/api/v1/system/certificates/auto-renew', {
          method: 'POST',
          body: JSON.stringify({ id, enabled, resume: enabled }),
        }),
      del: (id) => request(`/api/v1/system/certificates?id=${encodeURIComponent(id)}`, { method: 'DELETE' }),
    },
    apiKeys: {
      list: () => request('/api/v1/api-keys'),
      create: (name, role = 'admin') =>
        request('/api/v1/api-keys', { method: 'POST', body: JSON.stringify({ name, role }) }),
      del: (id) => request(`/api/v1/api-keys?id=${encodeURIComponent(id)}`, { method: 'DELETE' }),
    },
  },
  firewall: {
    rules: {
      list: () => request('/api/v1/firewall/rules'),
      add: (body, opts = {}) => {
        const q = opts.dryRun ? '?dry_run=1' : ''
        return request(`/api/v1/firewall/rules${q}`, { method: 'POST', body: JSON.stringify(body) })
      },
      put: (id, body, opts = {}) => {
        const dry = opts.dryRun ? '&dry_run=1' : ''
        return request(`/api/v1/firewall/rules?id=${encodeURIComponent(id)}${dry}`, {
          method: 'PUT',
          body: JSON.stringify(body),
        })
      },
      reorder: (order) =>
        request('/api/v1/firewall/rules/order', { method: 'PUT', body: JSON.stringify({ order }) }),
      del: (id) => request(`/api/v1/firewall/rules?id=${encodeURIComponent(id)}`, { method: 'DELETE' }),
    },
    aliases: {
      list: () => request('/api/v1/firewall/aliases'),
      add: (body) => request('/api/v1/firewall/aliases', { method: 'POST', body: JSON.stringify(body) }),
      del: (name) => request(`/api/v1/firewall/aliases?name=${encodeURIComponent(name)}`, { method: 'DELETE' }),
    },
  },
  network: {
    vlans: {
      list: () => request('/api/v1/network/vlans'),
      add: (body) => request('/api/v1/network/vlans', { method: 'POST', body: JSON.stringify(body) }),
      put: (id, body) =>
        request(`/api/v1/network/vlans?id=${encodeURIComponent(id)}`, {
          method: 'PUT',
          body: JSON.stringify(body),
        }),
      del: (id) => request(`/api/v1/network/vlans?id=${encodeURIComponent(id)}`, { method: 'DELETE' }),
    },
    netplan: {
      preview: () => request('/api/v1/network/netplan'),
      apply: () => request('/api/v1/network/netplan/apply', { method: 'POST', body: '{}' }),
    },
    vxlan: {
      list: () => request('/api/v1/network/vxlan'),
      add: (body) => request('/api/v1/network/vxlan', { method: 'POST', body: JSON.stringify(body) }),
      put: (id, body) =>
        request(`/api/v1/network/vxlan?id=${encodeURIComponent(id)}`, {
          method: 'PUT',
          body: JSON.stringify(body),
        }),
      del: (id) => request(`/api/v1/network/vxlan?id=${encodeURIComponent(id)}`, { method: 'DELETE' }),
    },
    wanLinks: {
      list: () => request('/api/v1/network/wan-links'),
      add: (body) => request('/api/v1/network/wan-links', { method: 'POST', body: JSON.stringify(body) }),
      put: (id, body) =>
        request(`/api/v1/network/wan-links?id=${encodeURIComponent(id)}`, {
          method: 'PUT',
          body: JSON.stringify(body),
        }),
      del: (id) => request(`/api/v1/network/wan-links?id=${encodeURIComponent(id)}`, { method: 'DELETE' }),
    },
    egressPolicies: {
      list: () => request('/api/v1/network/egress-policies'),
      bulkAdd: (policies, skipExisting = true) =>
        request('/api/v1/network/egress-policies/bulk', {
          method: 'POST',
          body: JSON.stringify({ policies, skip_existing: skipExisting }),
        }),
      add: (body) =>
        request('/api/v1/network/egress-policies', { method: 'POST', body: JSON.stringify(body) }),
      put: (id, body) =>
        request(`/api/v1/network/egress-policies?id=${encodeURIComponent(id)}`, {
          method: 'PUT',
          body: JSON.stringify(body),
        }),
      del: (id) =>
        request(`/api/v1/network/egress-policies?id=${encodeURIComponent(id)}`, { method: 'DELETE' }),
    },
    warp: {
      status: () => request('/api/v1/network/warp/status'),
      install: () => request('/api/v1/network/warp/install', { method: 'POST', body: '{}' }),
      installStatus: () => request('/api/v1/network/warp/install/status'),
      connect: () => request('/api/v1/network/warp/connect', { method: 'POST', body: '{}' }),
      disconnect: () => request('/api/v1/network/warp/disconnect', { method: 'POST', body: '{}' }),
      taskStatus: () => request('/api/v1/network/warp/task/status'),
    },
  },
  interfacesEthtool: (device) => request(`/api/v1/interfaces/ethtool?device=${encodeURIComponent(device)}`),
  setEthtool: (device, body) =>
    request(`/api/v1/interfaces/ethtool?device=${encodeURIComponent(device)}`, {
      method: 'PUT',
      body: JSON.stringify(body),
    }),
  shaper: {
    wizard: (body) => request('/api/v1/shaper/wizard', { method: 'POST', body: JSON.stringify(body) }),
    tenants: {
      list: () => request('/api/v1/shaper/tenants'),
      add: (body) => request('/api/v1/shaper/tenants', { method: 'POST', body: JSON.stringify(body) }),
      put: (id, body) =>
        request(`/api/v1/shaper/tenants?id=${encodeURIComponent(id)}`, {
          method: 'PUT',
          body: JSON.stringify(body),
        }),
      del: (id) => request(`/api/v1/shaper/tenants?id=${encodeURIComponent(id)}`, { method: 'DELETE' }),
    },
    putProfile: (body) =>
      request('/api/v1/shaper/profiles', { method: 'PUT', body: JSON.stringify(body) }),
    profiles: () => request('/api/v1/shaper/profiles'),
    delProfile: (cidr) => request(`/api/v1/shaper/profiles?cidr=${encodeURIComponent(cidr)}`, { method: 'DELETE' }),
    reorderProfiles: (order) =>
      request('/api/v1/shaper/profiles/order', { method: 'PUT', body: JSON.stringify({ order }) }),
    active: () => request('/api/v1/shaper/active'),
    tc: {
      put: (body) => request('/api/v1/shaper/tc', { method: 'PUT', body: JSON.stringify(body) }),
    },
  },
  interfaces: {
    list: () => request('/api/v1/interfaces'),
    update: (body) => request('/api/v1/interfaces', { method: 'PUT', body: JSON.stringify(body) }),
    setRoles: (body) =>
      request('/api/v1/interfaces/roles', { method: 'PUT', body: JSON.stringify(body) }),
  },
  ifaceQueues: () => request('/api/v1/interfaces/queues'),
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
