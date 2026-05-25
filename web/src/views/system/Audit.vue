<script setup>
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'

const { t } = useI18n()
const data = ref(null)

async function load() {
  data.value = await api.system.audit.list()
}

onMounted(load)
</script>

<template>
  <div class="page-stack">
    <PageHeader :title="t('system.audit.title')" :description="t('system.audit.description')" />
    <p v-if="data" class="text-xs text-slate-500 mb-3 font-mono">{{ data.path }}</p>
    <div class="card overflow-x-auto">
      <table class="data w-full text-sm">
        <thead>
          <tr>
            <th>{{ t('system.audit.colTime') }}</th>
            <th>{{ t('system.audit.colUser') }}</th>
            <th>{{ t('system.audit.colAction') }}</th>
            <th>{{ t('system.audit.colDetail') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(e, i) in data?.entries || []" :key="i">
            <td class="text-xs whitespace-nowrap">{{ e.time }}</td>
            <td>{{ e.user }}</td>
            <td class="font-mono text-xs">{{ e.action }}</td>
            <td class="text-xs text-slate-600">{{ e.detail }}</td>
          </tr>
          <tr v-if="!(data?.entries?.length)">
            <td colspan="4" class="text-slate-500 py-4">{{ t('system.audit.empty') }}</td>
          </tr>
        </tbody>
      </table>
    </div>
    <button type="button" class="btn-secondary text-sm mt-3" @click="load">{{ t('common.refresh') }}</button>
  </div>
</template>
