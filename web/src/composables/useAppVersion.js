import { ref } from 'vue'

/** @param {Record<string, unknown> | null | undefined} h */
export function versionLabelFromHealth(h) {
  const tag = String(h?.release_tag || h?.current_tag || '')
    .trim()
    .replace(/^v/i, '')
  if (tag) return `v${tag}`
  const build = String(h?.build_version || h?.current_version || '').trim()
  if (build && build !== 'unknown') return build
  return ''
}

export const appVersionLabel = ref('')

/** @param {Record<string, unknown> | null | undefined} h */
export function setAppVersionFromHealth(h) {
  appVersionLabel.value = versionLabelFromHealth(h)
}

/** @param {import('@/api/client').api} apiClient */
export async function refreshAppVersionFromHealth(apiClient) {
  try {
    const h = await apiClient.health()
    setAppVersionFromHealth(h)
  } catch {
    /* keep current */
  }
}

export function useAppVersion() {
  return { appVersionLabel, setAppVersionFromHealth, refreshAppVersionFromHealth }
}
