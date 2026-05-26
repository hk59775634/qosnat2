import { ref } from 'vue'

/** 默认 UI 产品名（与后端 store.DefaultDisplayName 一致） */
export const DEFAULT_DISPLAY_NAME = 'qosnat2'

export const displayName = ref(DEFAULT_DISPLAY_NAME)

export function applyDocumentTitle(name = displayName.value) {
  document.title = (name || DEFAULT_DISPLAY_NAME).trim() || DEFAULT_DISPLAY_NAME
}

export function setDisplayName(name) {
  const n = String(name || '').trim()
  displayName.value = n || DEFAULT_DISPLAY_NAME
  applyDocumentTitle()
}

/** @param {import('@/api/client').api} apiClient */
export async function refreshBrandingFromHealth(apiClient) {
  try {
    const h = await apiClient.health()
    setDisplayName(h.display_name)
  } catch {
    /* keep current */
  }
}

export function useBranding() {
  return {
    displayName,
    DEFAULT_DISPLAY_NAME,
    setDisplayName,
    applyDocumentTitle,
    refreshBrandingFromHealth,
  }
}
