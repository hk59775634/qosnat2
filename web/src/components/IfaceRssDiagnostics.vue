<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { api } from '@/api/client'

const props = defineProps({
  defaultExpanded: { type: Boolean, default: false },
})

const { t } = useI18n()
const router = useRouter()
const expanded = ref(props.defaultExpanded)
const data = ref(null)
const err = ref('')
const prevIrq = ref({ lan: {}, wan: {} })
const irqRate = ref({ lan: {}, wan: {} })
let timer

function irqRates(key, iface) {
  const rates = {}
  for (const line of iface?.irq_lines || []) {
    const prev = prevIrq.value[key][line.irq] ?? line.count
    rates[line.irq] = Math.max(0, Math.round((line.count - prev) / 5))
    prevIrq.value[key][line.irq] = line.count
  }
  irqRate.value[key] = rates
}

async function load() {
  try {
    const d = await api.ifaceQueues()
    if (data.value) {
      irqRates('lan', d.lan)
      irqRates('wan', d.wan)
    } else {
      for (const line of d.lan?.irq_lines || []) {
        prevIrq.value.lan[line.irq] = line.count
      }
      for (const line of d.wan?.irq_lines || []) {
        prevIrq.value.wan[line.irq] = line.count
      }
    }
    data.value = d
    err.value = ''
  } catch (e) {
    err.value = e.message
  }
}

function toggle() {
  expanded.value = !expanded.value
  if (expanded.value && !data.value) load()
}

const panels = computed(() => {
  if (!data.value) return []
  return [
    { key: 'lan', label: 'LAN', iface: data.value.lan },
    { key: 'wan', label: 'WAN', iface: data.value.wan },
  ]
})

watch(
  () => props.defaultExpanded,
  (v) => {
    if (v) {
      expanded.value = true
      load()
    }
  },
  { immediate: true },
)

onMounted(() => {
  if (expanded.value) {
    load()
    timer = setInterval(load, 5000)
  }
})

watch(expanded, (open) => {
  if (open && !timer) {
    load()
    timer = setInterval(load, 5000)
  } else if (!open && timer) {
    clearInterval(timer)
    timer = null
  }
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
})
</script>

<template>
  <section id="rss-diag" class="card mb-4">
    <button
      type="button"
      class="w-full flex items-center justify-between gap-2 p-4 text-left"
      @click="toggle"
    >
      <div>
        <h3 class="text-sm font-semibold text-slate-800">{{ t('network.queues.diagTitle') }}</h3>
        <p class="text-xs text-slate-500 mt-0.5">{{ t('network.queues.diagHint') }}</p>
      </div>
      <span class="text-slate-400 text-xs shrink-0">{{ expanded ? '▾' : '▸' }}</span>
    </button>

    <div v-if="expanded" class="border-t border-slate-100 px-4 pb-4">
      <p v-if="err" class="text-red-600 text-sm mt-3">{{ err }}</p>
      <p v-else-if="!data" class="text-sm text-slate-500 mt-3">{{ t('common.loading') }}</p>

      <div v-else class="grid lg:grid-cols-2 gap-4 mt-3">
        <section v-for="p in panels" :key="p.key" class="rounded border border-slate-100 p-3">
          <h4 class="font-medium text-xs uppercase text-slate-500 mb-1">{{ p.label }}</h4>
          <p class="text-sm font-mono mb-2">{{ p.iface?.device || '—' }}</p>
          <p class="text-sm text-slate-700">
            {{ t('network.queues.channel') }}: {{ p.iface?.channels || '—' }}
            · RX: {{ p.iface?.rx_queues ?? '—' }}
            · TX: {{ p.iface?.tx_queues ?? '—' }}
          </p>
          <div class="table-wrap mt-3 max-h-48 overflow-auto">
            <table class="data w-full text-xs">
              <thead>
                <tr>
                  <th>IRQ</th>
                  <th>{{ t('network.queues.count') }}</th>
                  <th>{{ t('network.queues.irqRate') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="irq in p.iface?.irq_lines?.slice(0, 16) || []" :key="irq.irq">
                  <td>{{ irq.irq }}</td>
                  <td>{{ irq.count?.toLocaleString() }}</td>
                  <td>{{ irqRate[p.key][irq.irq] ?? '—' }}</td>
                </tr>
                <tr v-if="!(p.iface?.irq_lines?.length)">
                  <td colspan="3" class="text-center text-slate-400 py-3">—</td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>
      </div>

      <p v-if="data?.lan?.softnet" class="text-xs text-slate-500 mt-3 font-mono">
        softnet —
        {{ t('network.queues.softnetProcessed') }}: {{ data.lan.softnet.processed?.toLocaleString() ?? '—' }}
        · {{ t('network.queues.softnetDropped') }}:
        <span :class="data.lan.softnet.dropped > 0 ? 'text-amber-700' : ''">
          {{ data.lan.softnet.dropped?.toLocaleString() ?? '—' }}
        </span>
        · {{ t('network.queues.softnetTimeSqueeze') }}: {{ data.lan.softnet.time_squeeze?.toLocaleString() ?? '—' }}
      </p>

      <p class="text-xs text-slate-500 mt-3">
        {{ t('network.queues.rpsHint') }}
        <button type="button" class="text-blue-600 hover:underline ml-1" @click="router.push('/system/advanced')">
          {{ t('nav.advanced') }}
        </button>
      </p>
    </div>
  </section>
</template>
