<script setup>
defineProps({
  label: String,
  value: [String, Number],
  sub: String,
})
</script>

<template>
  <div class="card p-4">
    <div class="text-xs text-slate-500 uppercase tracking-wide">{{ label }}</div>
    <div class="text-2xl font-semibold text-slate-800 mt-1">{{ value }}</div>
    <div v-if="sub" class="text-xs text-slate-400 mt-1">{{ sub }}</div>
  </div>
</template>
