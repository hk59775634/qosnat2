<script setup>
import { computed } from 'vue'

const props = defineProps({
  history: { type: Array, default: () => [] },
  field: { type: String, required: true }, // lan_rx_mbps | lan_tx_mbps | wan_rx_mbps | wan_tx_mbps
  label: { type: String, default: '' },
  color: { type: String, default: 'bg-blue-400' },
})

const points = computed(() => {
  const hist = props.history || []
  if (!hist.length) return []
  const vals = hist.map((p) => Number(p[props.field]) || 0)
  const max = Math.max(...vals, 0.1)
  return vals.map((v) => Math.max(4, Math.round((v / max) * 48)))
})
</script>

<template>
  <div>
    <p v-if="label" class="text-xs text-slate-500 mb-1">{{ label }}</p>
    <div class="flex items-end gap-px h-12" v-if="points.length">
      <div
        v-for="(h, i) in points"
        :key="i"
        :class="[color, 'w-1 rounded-t opacity-80']"
        :style="{ height: h + 'px' }"
        :title="((props.history[i] && props.history[i][props.field]) || 0).toFixed(2) + ' Mbps'"
      />
    </div>
    <p v-else class="text-xs text-slate-400">采集中…（约 5s 后显示）</p>
  </div>
</template>
