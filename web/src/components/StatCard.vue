<script setup>
defineProps({
  label: String,
  value: [String, Number],
  sub: String,
})
</script>

<template>
  <div class="card p-3">
    <div class="text-[10px] text-slate-500 uppercase tracking-wide">{{ label }}</div>
    <div class="text-xl font-semibold text-slate-800 mt-0.5 leading-tight">{{ value }}</div>
    <div v-if="sub" class="text-[10px] text-slate-400 mt-0.5">{{ sub }}</div>
  </div>
</template>
