import { onMounted, onUnmounted, ref } from 'vue'
import { api } from '@/api/client'

const items = ref([])
const unread = ref(0)
const open = ref(false)
let pollTimer = null

export function useNotifications() {
  async function refresh() {
    try {
      const data = await api.system.notifications.list()
      items.value = data.notifications || []
      unread.value = data.unread ?? 0
    } catch {
      /* ignore poll errors */
    }
  }

  async function dismiss(id) {
    await api.system.notifications.dismiss([id])
    await refresh()
  }

  async function dismissAll() {
    await api.system.notifications.dismissAll()
    open.value = false
    await refresh()
  }

  function startPolling(ms = 30000) {
    stopPolling()
    pollTimer = setInterval(refresh, ms)
  }

  function stopPolling() {
    if (pollTimer) {
      clearInterval(pollTimer)
      pollTimer = null
    }
  }

  return {
    items,
    unread,
    open,
    refresh,
    dismiss,
    dismissAll,
    startPolling,
    stopPolling,
  }
}

export function useNotificationsPollOnMount() {
  const n = useNotifications()
  onMounted(() => {
    n.refresh()
    n.startPolling()
  })
  onUnmounted(() => n.stopPolling())
  return n
}
