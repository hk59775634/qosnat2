<script setup>
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

const props = defineProps({
  history: { type: Array, default: () => [] },
  field: { type: String, required: true }, // lan_rx_mbps | lan_tx_mbps | wan_rx_mbps | wan_tx_mbps
  label: { type: String, default: '' },
  color: { type: String, default: 'bg-blue-400' },
  /** Taller bars for interface pages */
  tall: { type: Boolean, default: false },
})

const { t } = useI18n()
const barMaxPx = computed(() => (props.tall ? 64 : 48))

const points = computed(() => {
  const hist = props.history || []
  if (!hist.length) return []
  const vals = hist.map((p) => Number(p[props.field]) || 0)
  const max = Math.max(...vals, 0.1)
  const h = barMaxPx.value
  return vals.map((v) => Math.max(4, Math.round((v / max) * h)))
})
</script>

<template>
  <div>
    <p v-if="label" class="text-xs text-slate-500 mb-1">{{ label }}</p>
    <div class="flex items-end gap-px" :class="tall ? 'h-16' : 'h-12'" v-if="points.length">
      <div
        v-for="(h, i) in points"
        :key="i"
        :class="[color, 'w-1 rounded-t opacity-80']"
        :style="{ height: h + 'px' }"
        :title="((props.history[i] && props.history[i][props.field]) || 0).toFixed(2) + ' Mbps'"
      />
    </div>
    <p v-else class="text-xs text-slate-400">{{ t('components.trafficSampling') }}</p>
  </div>
</template>
