<script setup>
import { onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api, bpsLabel } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'

const { t } = useI18n()
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
  <div class="page-stack">
    <PageHeader :title="t('status.active.title')" :description="t('status.active.description')" :err="err" />

    <div class="card table-wrap p-4">
      <table class="data w-full">
        <thead>
          <tr>
            <th>IP</th>
            <th>{{ t('status.active.downCfg') }}</th>
            <th>{{ t('status.active.upCfg') }}</th>
            <th>{{ t('status.active.activityDown') }}</th>
            <th>{{ t('status.active.activityUp') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="a in list" :key="a.ip">
            <td class="font-mono">{{ a.ip }}</td>
            <td>{{ bpsLabel(a.down_bps) }}</td>
            <td>{{ bpsLabel(a.up_bps) }}</td>
            <td>{{ bpsLabel(a.activity_down) }}</td>
            <td>{{ bpsLabel(a.activity_up) }}</td>
          </tr>
          <tr v-if="!list.length">
            <td colspan="5" class="text-center text-slate-400 py-6">{{ t('status.active.noEntries') }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
