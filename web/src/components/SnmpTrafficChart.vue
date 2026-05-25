<script setup>
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

const props = defineProps({
  series: { type: Array, default: () => [] },
  height: { type: Number, default: 220 },
})

const hoverIdx = ref(-1)

const W = 640
const H = computed(() => props.height)
const pad = { l: 48, r: 16, t: 24, b: 32 }

const plot = computed(() => {
  const pts = props.series || []
  if (!pts.length) return null
  const maxPts = 400
  let data = pts
  if (pts.length > maxPts) {
    const step = Math.ceil(pts.length / maxPts)
    data = pts.filter((_, i) => i % step === 0)
  }
  const maxMbps = Math.max(
    0.1,
    ...data.flatMap((p) => [Number(p.rx_mbps) || 0, Number(p.tx_mbps) || 0]),
  )
  const innerW = W - pad.l - pad.r
  const innerH = H.value - pad.t - pad.b
  const n = data.length
  const x = (i) => pad.l + (n <= 1 ? innerW / 2 : (i / (n - 1)) * innerW)
  const y = (v) => pad.t + innerH - (v / maxMbps) * innerH

  const linePath = (key) => {
    if (!n) return ''
    return data
      .map((p, i) => `${i === 0 ? 'M' : 'L'}${x(i).toFixed(1)},${y(Number(p[key]) || 0).toFixed(1)}`)
      .join(' ')
  }
  const areaPath = (key) => {
    if (!n) return ''
    const top = data
      .map((p, i) => `${i === 0 ? 'M' : 'L'}${x(i).toFixed(1)},${y(Number(p[key]) || 0).toFixed(1)}`)
      .join(' ')
    const base = pad.t + innerH
    return `${top} L${x(n - 1).toFixed(1)},${base} L${x(0).toFixed(1)},${base} Z`
  }

  const yTicks = []
  for (let i = 0; i <= 4; i++) {
    const v = (maxMbps * i) / 4
    yTicks.push({ v, y: y(v), label: formatMbps(v) })
  }

  const xLabels = []
  if (n >= 2) {
    const picks = [0, Math.floor((n - 1) / 2), n - 1]
    for (const i of picks) {
      xLabels.push({ x: x(i), label: formatTime(data[i].ts) })
    }
  } else if (n === 1) {
    xLabels.push({ x: x(0), label: formatTime(data[0].ts) })
  }

  return {
    data,
    maxMbps,
    innerH,
    x,
    y,
    rxLine: linePath('rx_mbps'),
    txLine: linePath('tx_mbps'),
    rxArea: areaPath('rx_mbps'),
    txArea: areaPath('tx_mbps'),
    yTicks,
    xLabels,
    baseY: pad.t + innerH,
  }
})

function formatMbps(v) {
  if (v >= 1000) return `${(v / 1000).toFixed(1)} G`
  if (v >= 1) return `${v.toFixed(1)} M`
  if (v >= 0.001) return `${(v * 1000).toFixed(0)} K`
  return '0'
}

function formatTime(ts) {
  if (!ts) return '—'
  const d = new Date(ts * 1000)
  const mo = String(d.getMonth() + 1).padStart(2, '0')
  const da = String(d.getDate()).padStart(2, '0')
  const hh = String(d.getHours()).padStart(2, '0')
  const mm = String(d.getMinutes()).padStart(2, '0')
  if (plot.value?.data?.length > 48) {
    return `${mo}-${da}`
  }
  return `${mo}-${da} ${hh}:${mm}`
}

function onMove(ev) {
  const p = plot.value
  if (!p?.data?.length) return
  const rect = ev.currentTarget.getBoundingClientRect()
  const sx = ((ev.clientX - rect.left) / rect.width) * W
  const innerW = W - pad.l - pad.r
  let best = 0
  let bestD = Infinity
  p.data.forEach((_, i) => {
    const px = pad.l + (p.data.length <= 1 ? innerW / 2 : (i / (p.data.length - 1)) * innerW)
    const d = Math.abs(px - sx)
    if (d < bestD) {
      bestD = d
      best = i
    }
  })
  hoverIdx.value = best
}

function onLeave() {
  hoverIdx.value = -1
}

const tooltip = computed(() => {
  const p = plot.value
  if (!p || hoverIdx.value < 0 || hoverIdx.value >= p.data.length) return null
  const pt = p.data[hoverIdx.value]
  return {
    x: p.x(hoverIdx.value),
    time: formatTime(pt.ts),
    rx: Number(pt.rx_mbps) || 0,
    tx: Number(pt.tx_mbps) || 0,
  }
})
</script>

<template>
  <div class="snmp-chart rounded border border-slate-200 bg-[#f4f6f8] overflow-hidden">
    <div class="flex items-center justify-between px-3 py-2 border-b border-slate-200 bg-white/80 text-xs">
      <span class="font-medium text-slate-700">{{ t('components.snmpTrend') }}</span>
      <div class="flex gap-3 text-slate-600">
        <span class="flex items-center gap-1"><span class="w-3 h-0.5 bg-emerald-500 inline-block" /> {{ t('components.snmpLegendRx') }}</span>
        <span class="flex items-center gap-1"><span class="w-3 h-0.5 bg-sky-600 inline-block" /> {{ t('components.snmpLegendTx') }}</span>
      </div>
    </div>
    <div v-if="!plot" class="text-sm text-slate-500 py-12 text-center">
      {{ t('components.snmpEmpty') }}
    </div>
    <svg
      v-else
      :viewBox="`0 0 ${W} ${H}`"
      class="w-full block cursor-crosshair"
      preserveAspectRatio="none"
      @mousemove="onMove"
      @mouseleave="onLeave"
    >
      <defs>
        <linearGradient id="rxGrad" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stop-color="#10b981" stop-opacity="0.35" />
          <stop offset="100%" stop-color="#10b981" stop-opacity="0.05" />
        </linearGradient>
        <linearGradient id="txGrad" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stop-color="#0284c7" stop-opacity="0.3" />
          <stop offset="100%" stop-color="#0284c7" stop-opacity="0.04" />
        </linearGradient>
      </defs>
      <!-- 网格 -->
      <g stroke="#cbd5e1" stroke-width="0.5" opacity="0.8">
        <line
          v-for="(t, i) in plot.yTicks"
          :key="'h' + i"
          :x1="pad.l"
          :y1="t.y"
          :x2="W - pad.r"
          :y2="t.y"
        />
        <line
          v-for="(_, i) in 6"
          :key="'v' + i"
          :x1="pad.l + ((i / 5) * (W - pad.l - pad.r))"
          :y1="pad.t"
          :x2="pad.l + ((i / 5) * (W - pad.l - pad.r))"
          :y2="plot.baseY"
        />
      </g>
      <!-- Y 轴刻度 -->
      <g fill="#64748b" font-size="9" text-anchor="end">
        <text v-for="(t, i) in plot.yTicks" :key="'yl' + i" :x="pad.l - 6" :y="t.y + 3">{{ t.label }}</text>
      </g>
      <!-- 面积 -->
      <path :d="plot.rxArea" fill="url(#rxGrad)" />
      <path :d="plot.txArea" fill="url(#txGrad)" />
      <!-- 曲线 -->
      <path :d="plot.rxLine" fill="none" stroke="#059669" stroke-width="1.5" />
      <path :d="plot.txLine" fill="none" stroke="#0369a1" stroke-width="1.5" />
      <!-- X 轴 -->
      <g fill="#64748b" font-size="9" text-anchor="middle">
        <text v-for="(xl, i) in plot.xLabels" :key="'xl' + i" :x="xl.x" :y="H - 8">{{ xl.label }}</text>
      </g>
      <!-- 悬停 -->
      <g v-if="tooltip">
        <line
          :x1="tooltip.x"
          :y1="pad.t"
          :x2="tooltip.x"
          :y2="plot.baseY"
          stroke="#94a3b8"
          stroke-dasharray="3 2"
        />
        <rect
          :x="Math.min(tooltip.x + 8, W - 130)"
          y="8"
          width="120"
          height="52"
          rx="4"
          fill="rgba(255,255,255,0.95)"
          stroke="#cbd5e1"
        />
        <text :x="Math.min(tooltip.x + 14, W - 124)" y="24" font-size="9" fill="#334155">{{ tooltip.time }}</text>
        <text :x="Math.min(tooltip.x + 14, W - 124)" y="38" font-size="9" fill="#059669">RX {{ tooltip.rx.toFixed(2) }} Mbps</text>
        <text :x="Math.min(tooltip.x + 14, W - 124)" y="52" font-size="9" fill="#0369a1">TX {{ tooltip.tx.toFixed(2) }} Mbps</text>
      </g>
    </svg>
    <p class="text-[10px] text-slate-500 px-3 py-1 border-t border-slate-100">{{ t('components.snmpFooter') }}</p>
  </div>
</template>
