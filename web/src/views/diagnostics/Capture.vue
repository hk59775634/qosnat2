<script setup>
import { onMounted, ref } from 'vue'
import { api } from '@/api/client'

const list = ref([])
const dev = ref('')
const filter = ref('')
const duration = ref(30)
const err = ref('')

async function load() {
  list.value = await api.get('/api/v1/diagnostics/captures')
}

async function start() {
  err.value = ''
  try {
    await api.post('/api/v1/diagnostics/captures', {
      device: dev.value,
      filter: filter.value,
      duration_sec: duration.value,
    })
    await load()
  } catch (e) {
    err.value = e.message
  }
}

async function stop(id) {
  await api.del(`/api/v1/diagnostics/captures?id=${id}`)
  await load()
}

async function download(id) {
  err.value = ''
  try {
    const res = await fetch(`/api/v1/diagnostics/captures/${id}/download`, { credentials: 'include' })
    if (!res.ok) throw new Error(res.statusText)
    const blob = await res.blob()
    const a = document.createElement('a')
    a.href = URL.createObjectURL(blob)
    a.download = `capture-${id.slice(-8)}.pcap`
    a.click()
    URL.revokeObjectURL(a.href)
  } catch (e) {
    err.value = e.message
  }
}

function fmtSize(n) {
  if (!n) return '0'
  if (n < 1024) return n + ' B'
  return (n / 1024).toFixed(1) + ' KB'
}

onMounted(async () => {
  try {
    const h = await api.get('/api/v1/health')
    dev.value = h.dev_lan || ''
  } catch { /* ignore */ }
  await load()
})
</script>

<template>
  <div>
    <h2 class="text-xl font-semibold mb-4">抓包 (tcpdump)</h2>
    <p class="text-sm text-slate-600 mb-4">文件保存在 /var/lib/qosnat2/captures/，最长 300 秒。</p>
    <p v-if="err" class="text-red-600 text-sm mb-2">{{ err }}</p>

    <div class="card p-4 mb-6 max-w-xl grid gap-3">
      <input v-model="dev" class="input-field font-mono" placeholder="接口 ens19" />
      <input v-model="filter" class="input-field font-mono" placeholder="bpf 过滤，如 host 10.0.0.1" />
      <input v-model.number="duration" type="number" class="input-field w-32" placeholder="秒" />
      <button type="button" class="btn-primary w-fit" @click="start">开始抓包</button>
    </div>

    <div class="card table-wrap p-4">
      <table class="data w-full">
        <thead>
          <tr>
            <th>ID</th>
            <th>接口</th>
            <th>大小</th>
            <th>状态</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="c in list" :key="c.id">
            <td class="font-mono text-xs">{{ c.id.slice(-8) }}</td>
            <td>{{ c.device }}</td>
            <td>{{ fmtSize(c.size_bytes) }}</td>
            <td>{{ c.running ? '进行中' : '已结束' }}</td>
            <td>
              <button v-if="c.running" type="button" class="text-xs text-red-600 mr-2" @click='stop(c.id)'>停止</button>
              <button type="button" class="text-xs text-blue-600" @click='download(c.id)'>下载 pcap</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
