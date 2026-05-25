<script setup>
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'

const { t } = useI18n()
const data = ref(null)
const limit = ref(200)
const filter = ref('')
const err = ref('')
const loading = ref(false)

async function load() {
  loading.value = true
  err.value = ''
  try {
    const q = new URLSearchParams({ limit: String(limit.value) })
    if (filter.value.trim()) q.set('filter', filter.value.trim())
    data.value = await api.get(`/api/v1/diagnostics/conntrack?${q}`)
  } catch (e) {
    err.value = e.message
    data.value = null
  } finally {
    loading.value = false
  }
}

function fmtEp(ip, port) {
  if (!ip) return '—'
  if (port) return `${ip}:${port}`
  return ip
}

onMounted(load)
</script>

<template>
  <div class="page-stack">
    <PageHeader :title="t('diagnostics.conntrack.title')" :description="t('diagnostics.conntrack.description')" />

    <div class="card card-body mb-4 flex flex-wrap gap-3 items-end">
      <div>
        <label class="text-xs text-slate-500">{{ t('diagnostics.conntrack.limit') }}</label>
        <input v-model.number="limit" type="number" min="10" max="2000" class="input-field w-28" />
      </div>
      <div class="flex-1 min-w-[12rem]">
        <label class="text-xs text-slate-500">{{ t('diagnostics.conntrack.filter') }}</label>
        <input v-model="filter" class="input-field font-mono text-xs" />
      </div>
      <button type="button" class="btn-primary" :disabled="loading" @click="load">
        {{ loading ? t('common.loading') : t('common.refresh') }}
      </button>
    </div>

    <p v-if="err" class="text-red-600 text-sm mb-2">{{ err }}</p>

    <div v-if="data" class="card card-body mb-4 text-sm flex flex-wrap gap-4">
      <span>{{ t('diagnostics.conntrack.tableTotal') }}: <strong>{{ data.count }}</strong></span>
      <span>{{ t('diagnostics.conntrack.pageRows') }}: {{ data.entries?.length ?? 0 }} / limit {{ data.limit }}</span>
      <span v-if="data.truncated" class="text-amber-700">{{ t('diagnostics.conntrack.truncated') }}</span>
    </div>

    <div class="card table-wrap p-4 overflow-x-auto">
      <table class="data w-full text-xs">
        <thead>
          <tr>
            <th>{{ t('diagnostics.conntrack.colProto') }}</th>
            <th>{{ t('diagnostics.conntrack.colState') }}</th>
            <th>{{ t('diagnostics.conntrack.colTimeout') }}</th>
            <th>{{ t('diagnostics.conntrack.colOrig') }}</th>
            <th>{{ t('diagnostics.conntrack.colReply') }}</th>
            <th>Mark</th>
            <th>{{ t('diagnostics.conntrack.colFlags') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(e, i) in data?.entries || []" :key="i">
            <td class="font-mono">{{ e.l3_proto ? e.l3_proto + '/' : '' }}{{ e.protocol }}</td>
            <td>{{ e.state || '—' }}</td>
            <td>{{ e.timeout_sec }}s</td>
            <td class="font-mono whitespace-nowrap">{{ fmtEp(e.src, e.sport) }} → {{ fmtEp(e.dst, e.dport) }}</td>
            <td class="font-mono whitespace-nowrap text-slate-600">
              {{ fmtEp(e.reply_src, e.reply_sport) }} → {{ fmtEp(e.reply_dst, e.reply_dport) }}
            </td>
            <td>{{ e.mark ?? 0 }}</td>
            <td>{{ e.flags || '—' }}</td>
          </tr>
          <tr v-if="!loading && !(data?.entries?.length)">
            <td colspan="7" class="text-slate-400 text-center py-6">{{ t('diagnostics.conntrack.noMatch') }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
