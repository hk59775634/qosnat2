<script setup>
import { onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api, bpsLabel } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'

const { t } = useI18n()
const list = ref([])
const err = ref('')
const expanded = ref({})
let timer

function toggle(key) {
  expanded.value = { ...expanded.value, [key]: !expanded.value[key] }
}

function maskLabel(a) {
  if (a.shared) return `/${a.host_mask || '?'}`
  return '/32'
}

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
  <div class="page-stack">
    <PageHeader :title="t('status.active.title')" :description="t('status.active.description')" :err="err" />

    <div class="card table-wrap p-4">
      <table class="data w-full">
        <thead>
          <tr>
            <th>{{ t('status.active.colKey') }}</th>
            <th>{{ t('status.active.colMask') }}</th>
            <th>{{ t('status.active.downCfg') }}</th>
            <th>{{ t('status.active.upCfg') }}</th>
            <th>{{ t('status.active.rateDown') }}</th>
            <th>{{ t('status.active.rateUp') }}</th>
            <th>{{ t('status.active.colHosts') }}</th>
          </tr>
        </thead>
        <tbody>
          <template v-for="a in list" :key="a.key || a.ip">
            <tr>
              <td class="font-mono">
                <span v-if="a.shared" class="mr-1 rounded bg-amber-100 px-1.5 py-0.5 text-[10px] text-amber-800">{{ t('status.active.shared') }}</span>
                {{ a.key || a.ip }}
              </td>
              <td class="font-mono text-center">{{ maskLabel(a) }}</td>
              <td>{{ bpsLabel(a.down_bps) }}</td>
              <td>{{ bpsLabel(a.up_bps) }}</td>
              <td>{{ bpsLabel(a.rate_down_bps ?? a.activity_down) }}</td>
              <td>{{ bpsLabel(a.rate_up_bps ?? a.activity_up) }}</td>
              <td>
                <button
                  v-if="a.hosts?.length && (a.shared || a.hosts.length > 1)"
                  type="button"
                  class="text-xs text-blue-600 hover:underline"
                  @click="toggle(a.key || a.ip)"
                >
                  {{ a.hosts.length }}
                  {{ expanded[a.key || a.ip] ? t('status.active.collapse') : t('status.active.expand') }}
                </button>
                <span v-else-if="!a.shared" class="text-slate-400">1</span>
                <span v-else class="text-slate-400">—</span>
              </td>
            </tr>
            <tr v-if="expanded[a.key || a.ip] && a.hosts?.length">
              <td colspan="7" class="bg-slate-50 px-4 py-2">
                <table class="w-full text-sm">
                  <thead>
                    <tr class="text-slate-500">
                      <th class="text-left font-medium">IP</th>
                      <th class="text-left font-medium">{{ t('status.active.rateDown') }}</th>
                      <th class="text-left font-medium">{{ t('status.active.rateUp') }}</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="h in a.hosts" :key="h.ip">
                      <td class="font-mono py-0.5">{{ h.ip }}</td>
                      <td>{{ bpsLabel(h.rate_down_bps) }}</td>
                      <td>{{ bpsLabel(h.rate_up_bps) }}</td>
                    </tr>
                  </tbody>
                </table>
              </td>
            </tr>
          </template>
          <tr v-if="!list.length">
            <td colspan="7" class="text-center text-slate-400 py-6">{{ t('status.active.noEntries') }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
