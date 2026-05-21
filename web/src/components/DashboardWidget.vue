<script setup>
import { onMounted, ref, watch } from 'vue'

const props = defineProps({
  id: { type: String, required: true },
  title: { type: String, required: true },
  defaultOpen: { type: Boolean, default: true },
  reorderable: { type: Boolean, default: false },
  canMoveUp: { type: Boolean, default: false },
  canMoveDown: { type: Boolean, default: false },
})
const emit = defineEmits(['move-up', 'move-down'])

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
      class="w-full flex items-center justify-between px-3 py-2 bg-slate-50 border-b border-slate-200 hover:bg-slate-100 text-left"
      @click="toggle"
    >
      <h3 class="font-medium text-sm text-slate-800">{{ title }}</h3>
      <span class="flex items-center gap-2 shrink-0">
        <span v-if="reorderable" class="flex gap-1" @click.stop>
          <button
            type="button"
            class="text-xs px-1.5 py-0.5 rounded border border-slate-200 text-slate-500 hover:bg-white disabled:opacity-30"
            :disabled="!canMoveUp"
            title="上移"
            @click="emit('move-up')"
          >
            ↑
          </button>
          <button
            type="button"
            class="text-xs px-1.5 py-0.5 rounded border border-slate-200 text-slate-500 hover:bg-white disabled:opacity-30"
            :disabled="!canMoveDown"
            title="下移"
            @click="emit('move-down')"
          >
            ↓
          </button>
        </span>
        <span class="text-slate-400 text-xs">{{ open ? '收起' : '展开' }}</span>
      </span>
    </button>
    <div v-show="open" class="p-3 text-sm">
      <slot />
    </div>
  </section>
</template>
