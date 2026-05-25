<script setup>
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'

const { t } = useI18n()
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
  <div class="page-stack">
    <PageHeader :title="t('diagnostics.capture.title')" :description="t('diagnostics.capture.description')" />
    <p v-if="err" class="text-red-600 text-sm mb-2">{{ err }}</p>

    <div class="card card-body mb-0 grid gap-3">
      <input v-model="dev" class="input-field font-mono" :placeholder="t('diagnostics.capture.ifacePh')" />
      <input v-model="filter" class="input-field font-mono" :placeholder="t('diagnostics.capture.bpfPh')" />
      <input v-model.number="duration" type="number" class="input-field w-32" :placeholder="t('diagnostics.capture.secPh')" />
      <button type="button" class="btn-primary w-fit" @click="start">{{ t('diagnostics.capture.start') }}</button>
    </div>

    <div class="card table-wrap p-4">
      <table class="data w-full">
        <thead>
          <tr>
            <th>ID</th>
            <th>{{ t('diagnostics.capture.colIface') }}</th>
            <th>{{ t('diagnostics.capture.colSize') }}</th>
            <th>{{ t('diagnostics.capture.colStatus') }}</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="c in list" :key="c.id">
            <td class="font-mono text-xs">{{ c.id.slice(-8) }}</td>
            <td>{{ c.device }}</td>
            <td>{{ fmtSize(c.size_bytes) }}</td>
            <td>{{ c.running ? t('diagnostics.capture.inProgress') : t('diagnostics.capture.finished') }}</td>
            <td>
              <button v-if="c.running" type="button" class="text-xs text-red-600 mr-2" @click="stop(c.id)">{{ t('diagnostics.capture.stop') }}</button>
              <button type="button" class="text-xs text-blue-600" @click="download(c.id)">{{ t('diagnostics.capture.download') }}</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
