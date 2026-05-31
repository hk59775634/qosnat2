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

/**
 * Poll health on the new admin port (cross-origin no-cors) until reachable or timeout.
 * @param {{ host: string, port: string|number, scheme?: string, timeoutMs?: number, intervalMs?: number, minWaitMs?: number }} opts
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
 * Wait for new port then replace location (preserves hash route).
 */
export async function redirectAfterAdminPortChange({
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
