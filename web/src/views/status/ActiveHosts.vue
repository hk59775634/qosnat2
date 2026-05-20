<script setup>
import { onMounted, onUnmounted, ref } from 'vue'
import { api, bpsLabel } from '@/api/client'

const list = ref([])
const err = ref('')
let timer

async function load() {
  try {
    list.value = await api.shaper.active()
    err.value = ''
  } catch (e) {
    err.value = e.message
  }
}

onMounted(() => {
  load()
  timer = setInterval(load, 3000)
})
onUnmounted(() => clearInterval(timer))
</script>

<template>
  <div>
    <h2 class="text-xl font-semibold mb-4">eBPF 活跃池 (active_host)</h2>
    <p class="text-sm text-slate-500 mb-4">每 3 秒刷新 · Iterate Map</p>
    <p v-if="err" class="text-red-600 mb-2">{{ err }}</p>

    <div class="card table-wrap p-4">
      <table class="data w-full">
        <thead>
          <tr>
            <th>IP</th>
            <th>class</th>
            <th>下行配置</th>
            <th>上行配置</th>
            <th>bytes↓</th>
            <th>bytes↑</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="a in list" :key="a.ip">
            <td class="font-mono">{{ a.ip }}</td>
            <td>1:{{ (a.class_minor || 0).toString(16) }}</td>
            <td>{{ bpsLabel(a.down_bps) }}</td>
            <td>{{ bpsLabel(a.up_bps) }}</td>
            <td>{{ a.bytes_down }}</td>
            <td>{{ a.bytes_up }}</td>
          </tr>
          <tr v-if="!list.length">
            <td colspan="6" class="text-center text-slate-400 py-6">无活跃条目</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
