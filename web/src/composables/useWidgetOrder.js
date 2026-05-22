import { ref, watch } from 'vue'

const DEFAULT_KEY = 'qosnat2-dashboard-widget-order'

export function useWidgetOrder(defaultIds, storageKey = DEFAULT_KEY) {
  const order = ref(loadOrder(defaultIds, storageKey))

  function loadOrder(def, key) {
    try {
      const raw = localStorage.getItem(key)
      if (!raw) return [...def]
      const saved = JSON.parse(raw)
      if (!Array.isArray(saved)) return [...def]
      const set = new Set(def)
      const merged = saved.filter((id) => set.has(id))
      for (const id of def) {
        if (!merged.includes(id)) merged.push(id)
      }
      return merged
    } catch {
      return [...def]
    }
  }

  function saveOrder() {
    try {
      localStorage.setItem(storageKey, JSON.stringify(order.value))
    } catch {
      /* ignore */
    }
  }

  function moveUp(id) {
    const i = order.value.indexOf(id)
    if (i <= 0) return
    const next = [...order.value]
    ;[next[i - 1], next[i]] = [next[i], next[i - 1]]
    order.value = next
    saveOrder()
  }

  function moveDown(id) {
    const i = order.value.indexOf(id)
    if (i < 0 || i >= order.value.length - 1) return
    const next = [...order.value]
    ;[next[i], next[i + 1]] = [next[i + 1], next[i]]
    order.value = next
    saveOrder()
  }

  watch(order, saveOrder, { deep: true })

  return { order, moveUp, moveDown }
}
