<script setup>
import { onMounted, ref, watch } from 'vue'

const props = defineProps({
  id: { type: String, required: true },
  title: { type: String, required: true },
  defaultOpen: { type: Boolean, default: true },
})

const open = ref(props.defaultOpen)
const storageKey = 'qosnat2-widget-collapsed'

function loadState() {
  try {
    const raw = localStorage.getItem(storageKey)
    if (!raw) return
    const map = JSON.parse(raw)
    if (props.id in map) open.value = !map[props.id]
  } catch {
    /* ignore */
  }
}

function saveState() {
  try {
    const raw = localStorage.getItem(storageKey)
    const map = raw ? JSON.parse(raw) : {}
    map[props.id] = !open.value
    localStorage.setItem(storageKey, JSON.stringify(map))
  } catch {
    /* ignore */
  }
}

function toggle() {
  open.value = !open.value
  saveState()
}

onMounted(loadState)
watch(() => props.id, loadState)
</script>

<template>
  <section class="card overflow-hidden">
    <button
      type="button"
      class="w-full flex items-center justify-between px-4 py-3 bg-slate-50 border-b border-slate-200 hover:bg-slate-100 text-left"
      @click="toggle"
    >
      <h3 class="font-medium text-slate-800">{{ title }}</h3>
      <span class="text-slate-400 text-xs">{{ open ? '收起' : '展开' }}</span>
    </button>
    <div v-show="open" class="p-4">
      <slot />
    </div>
  </section>
</template>
