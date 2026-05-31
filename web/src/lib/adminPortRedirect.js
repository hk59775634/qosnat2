function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

/** @param {{ host: string, port: string|number, scheme?: string, pathname?: string, hash?: string }} opts */
export function buildAdminPortUrl({ host, port, scheme = location.protocol, pathname = location.pathname, hash = location.hash }) {
  const h = host || location.hostname || 'localhost'
  const p = String(port || '').trim()
  const path = pathname || '/'
  const frag = hash || ''
  return `${scheme}//${h}:${p}${path}${frag}`
}

/** Prefer API access_host, then cert SAN/CN, then ACME domain, then current hostname. */
export function resolveTlsAccessHost(tlsStatus, fallbackHost = location.hostname) {
  const access = String(tlsStatus?.access_host || '').trim()
  if (access) return access
  const names = tlsStatus?.cert_hostnames
  if (Array.isArray(names)) {
    for (const raw of names) {
      const h = String(raw || '').trim()
      if (!h) continue
      if (h.startsWith('*.')) return h.slice(2)
      return h
    }
  }
  const subj = String(tlsStatus?.cert_subject || '')
  const cn = subj.match(/CN=([^,/]+)/i)
  if (cn?.[1]) return cn[1].trim()
  const domain = String(tlsStatus?.domain || '').trim()
  if (domain) return domain
  return fallbackHost || 'localhost'
}

/**
 * Poll health on the target admin port (cross-origin no-cors) until reachable or timeout.
 */
export async function waitForAdminPort({
  host,
  port,
  scheme = location.protocol,
  timeoutMs = 45000,
  intervalMs = 500,
  minWaitMs = 800,
}) {
  await sleep(minWaitMs)
  const url = `${scheme}//${host}:${port}/api/v1/health`
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    try {
      const res = await fetch(url, { mode: 'no-cors', cache: 'no-store' })
      if (res.type === 'opaque' || res.ok) return true
    } catch {
      /* listener not ready yet */
    }
    await sleep(intervalMs)
  }
  return false
}

/**
 * Wait for listener then replace location (preserves hash route).
 */
export async function redirectAfterListenerChange({
  host,
  port,
  tlsActive = false,
  pathname = location.pathname,
  hash = location.hash,
}) {
  const scheme = tlsActive ? 'https:' : 'http:'
  await waitForAdminPort({ host, port, scheme })
  window.location.replace(buildAdminPortUrl({ host, port, scheme, pathname, hash }))
}

/** @deprecated use redirectAfterListenerChange */
export async function redirectAfterAdminPortChange(opts) {
  return redirectAfterListenerChange(opts)
}

/**
 * @returns {Promise<boolean>} true if redirect started
 */
export async function maybeRedirectAfterSystemSave({
  res,
  previousPort,
  wasTlsActive,
  onSwitching,
}) {
  const newPort = String(res?.admin_port || previousPort || location.port || '').trim()
  if (!newPort) return false

  const nowTlsActive = !!res?.tls?.tls_active
  const portChanged = newPort !== String(previousPort || location.port || '').trim()
  const httpsEnabled = !wasTlsActive && nowTlsActive
  const httpsDisabled = wasTlsActive && !nowTlsActive

  if (!portChanged && !httpsEnabled && !httpsDisabled) return false

  if (onSwitching) {
    onSwitching({ portChanged, httpsEnabled, httpsDisabled, newPort, nowTlsActive })
  }

  const host = nowTlsActive
    ? resolveTlsAccessHost(res?.tls, location.hostname)
    : (location.hostname || 'localhost')

  await redirectAfterListenerChange({
    host,
    port: newPort,
    tlsActive: nowTlsActive,
  })
  return true
}
