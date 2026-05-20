<script setup>
defineProps({
  label: String,
  ok: Boolean,
  detail: String,
})
</script>

<template>
  <div class="flex items-center justify-between py-2 border-b border-slate-100 last:border-0">
    <span class="text-sm text-slate-700">{{ label }}</span>
    <div class="text-right">
      <span
        class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium"
        :class="ok ? 'bg-green-100 text-green-800' : 'bg-slate-100 text-slate-600'"
      >
        {{ ok ? '运行中' : '未启用' }}
      </span>
      <p v-if="detail" class="text-xs text-slate-400 mt-0.5 font-mono">{{ detail }}</p>
    </div>
  </div>
</template>
