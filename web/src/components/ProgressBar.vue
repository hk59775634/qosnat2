<script setup>
defineProps({
  label: String,
  value: { type: Number, default: 0 },
  max: { type: Number, default: 100 },
  unit: { type: String, default: '%' },
  color: { type: String, default: 'blue' },
})

const colors = {
  blue: 'bg-blue-500',
  green: 'bg-green-500',
  amber: 'bg-amber-500',
  red: 'bg-red-500',
}
</script>

<template>
  <div class="mb-3 last:mb-0">
    <div class="flex justify-between text-xs text-slate-600 mb-1">
      <span>{{ label }}</span>
      <span class="font-mono">{{ value?.toFixed?.(1) ?? value }}{{ unit }}</span>
    </div>
    <div class="h-2 bg-slate-100 rounded-full overflow-hidden">
      <div
        class="h-full rounded-full transition-all duration-500"
        :class="colors[color] || colors.blue"
        :style="{ width: `${Math.min(100, (value / max) * 100)}%` }"
      />
    </div>
  </div>
</template>
