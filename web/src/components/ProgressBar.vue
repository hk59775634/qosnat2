<script setup>
import { computed } from 'vue'

const props = defineProps({
  label: String,
  value: { type: Number, default: 0 },
  max: { type: Number, default: 100 },
  unit: { type: String, default: '%' },
  color: { type: String, default: 'blue' },
  /** 与 capMbps 同用：按链路速率算百分比，trafficMbps 为当前流量 Mbps */
  trafficMbps: { type: Number, default: undefined },
  capMbps: { type: Number, default: 0 },
})

const barWidth = computed(() => {
  if (props.capMbps > 0 && props.trafficMbps != null) {
    return Math.min(100, (props.trafficMbps / props.capMbps) * 100)
  }
  const max = props.max > 0 ? props.max : 100
  return Math.min(100, (props.value / max) * 100)
})

const displayRight = computed(() => {
  if (props.capMbps > 0 && props.trafficMbps != null) {
    const pct = (props.trafficMbps / props.capMbps) * 100
    return `${props.trafficMbps.toFixed(2)} Mbps · ${pct.toFixed(1)}%`
  }
  const v = props.value?.toFixed?.(1) ?? props.value
  return `${v}${props.unit}`
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
      <span class="font-mono">{{ displayRight }}</span>
    </div>
    <div class="h-2 bg-slate-100 rounded-full overflow-hidden">
      <div
        class="h-full rounded-full transition-all duration-500"
        :class="colors[color] || colors.blue"
        :style="{ width: `${barWidth}%` }"
      />
    </div>
  </div>
</template>
