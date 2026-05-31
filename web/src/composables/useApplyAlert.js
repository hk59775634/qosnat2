import { ref } from 'vue'

/** 全局配置应用失败提示（nft/save/dataplane） */
export const applyAlert = ref(null)

export function setApplyAlert(message) {
  applyAlert.value = message
}

export function clearApplyAlert() {
  applyAlert.value = null
}

export function isApplyFailure(status, data) {
  const msg = String(data?.error || data?.message || '')
  if (status === 422) return true
  if (status >= 500) return true
  if (/nft ruleset invalid|save failed|apply failed/i.test(msg)) return true
  return false
}
