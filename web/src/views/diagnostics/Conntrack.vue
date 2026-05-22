<script setup>
import { onMounted, ref } from 'vue'
import { api } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'

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
    <PageHeader
      title="连接状态 (conntrack)"
      description="来自 conntrack -L；总数取自 nf_conntrack_count。大量连接时仅返回前 N 条。"
    />

    <div class="card card-body mb-4 flex flex-wrap gap-3 items-end">
      <div>
        <label class="text-xs text-slate-500">条数上限</label>
        <input v-model.number="limit" type="number" min="10" max="2000" class="input-field w-28" />
      </div>
      <div class="flex-1 min-w-[12rem]">
        <label class="text-xs text-slate-500">过滤（src/dst 子串）</label>
        <input v-model="filter" class="input-field font-mono text-xs" placeholder="10.0.0. 或 8.8.8.8" />
      </div>
      <button type="button" class="btn-primary" :disabled="loading" @click="load">
        {{ loading ? '加载中…' : '刷新' }}
      </button>
    </div>

    <p v-if="err" class="text-red-600 text-sm mb-2">{{ err }}</p>

    <div v-if="data" class="card card-body mb-4 text-sm flex flex-wrap gap-4">
      <span>表内总数: <strong>{{ data.count }}</strong></span>
      <span>本页: {{ data.entries?.length ?? 0 }} / limit {{ data.limit }}</span>
      <span v-if="data.truncated" class="text-amber-700">已截断</span>
    </div>

    <div class="card table-wrap p-4 overflow-x-auto">
      <table class="data w-full text-xs">
        <thead>
          <tr>
            <th>协议</th>
            <th>状态</th>
            <th>超时</th>
            <th>原始</th>
            <th>回复</th>
            <th>Mark</th>
            <th>标志</th>
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
            <td colspan="7" class="text-slate-400 text-center py-6">无匹配连接</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
