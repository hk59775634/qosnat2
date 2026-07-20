import { isApplyFailure, setApplyAlert } from '@/composables/useApplyAlert'

const base = ''
const sessionStorageKey = 'qosnat_sess'

function readStoredSessionToken() {
  try {
    return sessionStorage.getItem(sessionStorageKey) || ''
  } catch {
    return ''
  }
}

function storeSessionToken(tok) {
  if (!tok) return
  try {
    sessionStorage.setItem(sessionStorageKey, tok)
  } catch {
    /* ignore */
  }
}

function clearStoredSessionToken() {
  try {
    sessionStorage.removeItem(sessionStorageKey)
  } catch {
    /* ignore */
  }
}

export async function request(path, opts = {}) {
  const headers = { 'Content-Type': 'application/json', ...(opts.headers || {}) }
  const tok = readStoredSessionToken()
  if (tok) {
    headers['X-Qosnat-Session'] = tok
  }
  let res
  try {
    res = await fetch(`${base}${path}`, {
      credentials: 'include',
      headers,
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
  if (data?.session_token) {
    storeSessionToken(data.session_token)
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
  logout: () => {
    clearStoredSessionToken()
    return request('/api/v1/logout', { method: 'POST' })
  },

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
    ipv4: {
      get: () => request('/api/v1/nat/ipv4'),
      put: (body) => request('/api/v1/nat/ipv4', { method: 'PUT', body: JSON.stringify(body) }),
    },
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
      export: (body) =>
        request('/api/v1/system/state/export', { method: 'POST', body: JSON.stringify(body) }),
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
      switchReset: () =>
        request('/api/v1/system/version/switch/reset', { method: 'POST', body: '{}' }),
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
    apply: () => request('/api/v1/firewall/apply', { method: 'POST', body: '{}' }),
    discard: () => request('/api/v1/firewall/discard', { method: 'POST', body: '{}' }),
    sessionLimit: {
      put: (body) =>
        request('/api/v1/firewall/session-limit', { method: 'PUT', body: JSON.stringify(body) }),
    },
  aliases: {
    list: () => request('/api/v1/firewall/aliases'),
    add: (body) => request('/api/v1/firewall/aliases', { method: 'POST', body: JSON.stringify(body) }),
    refresh: (name) =>
      request(
        `/api/v1/firewall/aliases/refresh${name ? `?name=${encodeURIComponent(name)}` : ''}`,
        { method: 'POST', body: '{}' },
      ),
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
      saveLicense: (body) =>
        request('/api/v1/network/warp/license', { method: 'PUT', body: JSON.stringify(body) }),
      applyLicense: () => request('/api/v1/network/warp/license/apply', { method: 'POST', body: '{}' }),
      deleteLicense: () => request('/api/v1/network/warp/license', { method: 'DELETE' }),
      install: () => request('/api/v1/network/warp/install', { method: 'POST', body: '{}' }),
      installStatus: () => request('/api/v1/network/warp/install/status'),
      connect: () => request('/api/v1/network/warp/connect', { method: 'POST', body: '{}' }),
      disconnect: () => request('/api/v1/network/warp/disconnect', { method: 'POST', body: '{}' }),
      taskStatus: () => request('/api/v1/network/warp/task/status'),
    },
    proxyEgress: {
      list: () => request('/api/v1/network/proxy-egress'),
      add: (body) => request('/api/v1/network/proxy-egress', { method: 'POST', body: JSON.stringify(body) }),
      put: (id, body) =>
        request(`/api/v1/network/proxy-egress?id=${encodeURIComponent(id)}`, {
          method: 'PUT',
          body: JSON.stringify(body),
        }),
      del: (id) =>
        request(`/api/v1/network/proxy-egress?id=${encodeURIComponent(id)}`, { method: 'DELETE' }),
      status: () => request('/api/v1/network/proxy-egress/status'),
      install: () => request('/api/v1/network/proxy-egress/install', { method: 'POST', body: '{}' }),
      installStatus: () => request('/api/v1/network/proxy-egress/install/status'),
      uninstall: () => request('/api/v1/network/proxy-egress/uninstall', { method: 'POST', body: '{}' }),
      connect: (id) =>
        request(`/api/v1/network/proxy-egress/connect?id=${encodeURIComponent(id)}`, {
          method: 'POST',
          body: '{}',
        }),
      disconnect: (id) =>
        request(`/api/v1/network/proxy-egress/disconnect?id=${encodeURIComponent(id)}`, {
          method: 'POST',
          body: '{}',
        }),
      taskStatus: () => request('/api/v1/network/proxy-egress/task/status'),
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
    enabled: {
      get: () => request('/api/v1/shaper/enabled'),
      put: (body) => request('/api/v1/shaper/enabled', { method: 'PUT', body: JSON.stringify(body) }),
    },
    profiles: () => request('/api/v1/shaper/profiles'),
    delProfile: (cidr) => request(`/api/v1/shaper/profiles?cidr=${encodeURIComponent(cidr)}`, { method: 'DELETE' }),
    reorderProfiles: (order) =>
      request('/api/v1/shaper/profiles/order', { method: 'PUT', body: JSON.stringify({ order }) }),
    active: (ip) =>
      request(
        ip
          ? `/api/v1/shaper/active?ip=${encodeURIComponent(ip)}`
          : '/api/v1/shaper/active',
      ),
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
  dhcp: {
    installChnroutesDnsmasq: () =>
      request('/api/v1/dhcp/dnsmasq/install-chnroutes', { method: 'POST', body: '{}' }),
    installChnroutesDnsmasqStatus: () => request('/api/v1/dhcp/dnsmasq/install-chnroutes/status'),
  },
  snmp: {
    get: () => request('/api/v1/snmp'),
    put: (body) => request('/api/v1/snmp', { method: 'PUT', body: JSON.stringify(body) }),
    apply: () => request('/api/v1/snmp/apply', { method: 'POST', body: '{}' }),
    install: () => request('/api/v1/snmp/install', { method: 'POST', body: '{}' }),
    service: (action) =>
      request('/api/v1/snmp/service', { method: 'POST', body: JSON.stringify({ action }) }),
  },
  lvs: {
    get: () => request('/api/v1/lvs'),
    put: (body) => request('/api/v1/lvs', { method: 'PUT', body: JSON.stringify(body) }),
    apply: () => request('/api/v1/lvs/apply', { method: 'POST', body: '{}' }),
    install: () => request('/api/v1/lvs/install', { method: 'POST', body: '{}' }),
    addVirtualServer: (body) =>
      request('/api/v1/lvs/virtual-servers', { method: 'POST', body: JSON.stringify(body) }),
    addRealServer: (body) =>
      request('/api/v1/lvs/virtual-servers/real-servers', { method: 'POST', body: JSON.stringify(body) }),
    delRealServer: (vsId, ip, port) => {
      let q = `vs_id=${encodeURIComponent(vsId)}&ip=${encodeURIComponent(ip)}`
      if (port) q += `&port=${encodeURIComponent(port)}`
      return request(`/api/v1/lvs/virtual-servers/real-servers?${q}`, { method: 'DELETE' })
    },
    addOcservCluster: (body) =>
      request('/api/v1/lvs/ocserv-cluster', { method: 'POST', body: JSON.stringify(body) }),
    delVirtualServer: (id) =>
      request(`/api/v1/lvs/virtual-servers?id=${encodeURIComponent(id)}`, { method: 'DELETE' }),
  },
  frr: {
    get: () => request('/api/v1/frr'),
    put: (body) => request('/api/v1/frr', { method: 'PUT', body: JSON.stringify(body) }),
    apply: () => request('/api/v1/frr/apply', { method: 'POST', body: '{}' }),
    install: () => request('/api/v1/frr/install', { method: 'POST', body: '{}' }),
    installStatus: () => request('/api/v1/frr/install/status'),
    service: (action) =>
      request('/api/v1/frr/service', { method: 'POST', body: JSON.stringify({ action }) }),
    getConfig: (which) => request(`/api/v1/frr/config?which=${encodeURIComponent(which)}`),
    putConfig: (which, content) =>
      request('/api/v1/frr/config', {
        method: 'PUT',
        body: JSON.stringify({ which, content }),
      }),
    dynamicRouting: {
      get: () => request('/api/v1/frr/dynamic-routing'),
      put: (dynamicRouting) =>
        request('/api/v1/frr/dynamic-routing', {
          method: 'PUT',
          body: JSON.stringify({ dynamic_routing: dynamicRouting }),
        }),
      apply: () =>
        request('/api/v1/frr/dynamic-routing/apply', { method: 'POST', body: '{}' }),
      status: () => request('/api/v1/frr/dynamic-routing/status'),
    },
  },
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
