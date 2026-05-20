<script setup>
import { onMounted, onUnmounted, ref } from 'vue'
import { api } from '@/api/client'
import StatCard from '@/components/StatCard.vue'

const data = ref(null)
const health = ref(null)
const err = ref('')
let timer

async function load() {
  try {
    ;[data.value, health.value] = await Promise.all([api.dashboard(), api.health()])
    err.value = ''
  } catch (e) {
    err.value = e.message
  }
}

onMounted(() => {
  load()
  timer = setInterval(load, 5000)
})
onUnmounted(() => clearInterval(timer))
</script>

<template>
  <div>
    <h2 class="text-xl font-semibold mb-4">仪表大厅</h2>
    <p v-if="err" class="text-red-600 mb-4">{{ err }}</p>

    <div class="grid grid-cols-2 lg:grid-cols-4 gap-4 mb-6">
      <StatCard label="活跃 Per-IP" :value="data?.active_hosts ?? '—'" />
      <StatCard label="CPU" :value="data ? `${data.system.cpu_percent.toFixed(1)}%` : '—'" />
      <StatCard label="内存" :value="data ? `${data.system.mem_percent.toFixed(1)}%` : '—'" />
      <StatCard label="Conntrack" :value="data?.system?.conntrack ?? '—'" />
    </div>

    <div class="grid lg:grid-cols-2 gap-4 mb-6">
      <div class="card p-4">
        <h3 class="font-medium mb-2">LAN ({{ health?.dev_lan }})</h3>
        <p class="text-sm text-slate-600">
          ↓ {{ data?.lan?.rx_mbps?.toFixed(2) ?? 0 }} Mbps · ↑ {{ data?.lan?.tx_mbps?.toFixed(2) ?? 0 }} Mbps
        </p>
      </div>
      <div class="card p-4">
        <h3 class="font-medium mb-2">WAN ({{ health?.dev_wan }})</h3>
        <p class="text-sm text-slate-600">
          ↓ {{ data?.wan?.rx_mbps?.toFixed(2) ?? 0 }} Mbps · ↑ {{ data?.wan?.tx_mbps?.toFixed(2) ?? 0 }} Mbps
        </p>
      </div>
    </div>

    <div v-if="data?.mark_policy" class="card p-4 mb-6 flex items-center justify-between">
      <span class="text-sm">Mark 隔离审计</span>
      <span :class="data.mark_policy.rules_ok ? 'text-green-700 text-sm font-medium' : 'text-red-600 text-sm'">
        {{ data.mark_policy.rules_ok ? '通过' : '异常' }}
      </span>
    </div>

    <div v-if="data?.interfaces" class="grid lg:grid-cols-2 gap-4 mb-6">
      <div class="card p-4 text-sm">
        <h3 class="font-medium mb-1">LAN RSS</h3>
        <p>{{ data.interfaces.lan?.channels || 0 }} 队列 · IRQ 行 {{ data.interfaces.lan?.irq_lines?.length || 0 }}</p>
      </div>
      <div class="card p-4 text-sm">
        <h3 class="font-medium mb-1">WAN RSS</h3>
        <p>{{ data.interfaces.wan?.channels || 0 }} 队列 · IRQ 行 {{ data.interfaces.wan?.irq_lines?.length || 0 }}</p>
      </div>
    </div>

    <div class="card p-4">
      <h3 class="font-medium mb-3">Top 活跃主机</h3>
      <div class="table-wrap">
        <table class="data w-full">
          <thead>
            <tr>
              <th>IP</th>
              <th>下行 Mbps</th>
              <th>上行 Mbps</th>
              <th>累计 down</th>
              <th>累计 up</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="h in data?.top_hosts || []" :key="h.ip">
              <td class="font-mono">{{ h.ip }}</td>
              <td>{{ h.down_mbps?.toFixed(2) ?? '—' }}</td>
              <td>{{ h.up_mbps?.toFixed(2) ?? '—' }}</td>
              <td>{{ (h.bytes_down / 1024 / 1024).toFixed(2) }} MB</td>
              <td>{{ (h.bytes_up / 1024 / 1024).toFixed(2) }} MB</td>
            </tr>
            <tr v-if="!(data?.top_hosts?.length)">
              <td colspan="5" class="text-slate-400 py-4 text-center">暂无活跃流量</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>
