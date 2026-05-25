<script setup>
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'

const { t } = useI18n()
const data = ref(null)

onMounted(async () => {
  data.value = await api.markPolicy()
})
</script>

<template>
  <div class="page-stack">
    <PageHeader :title="t('status.mark.title')" :description="t('status.mark.description')" />
    <div v-if="data" class="card p-4">
      <p :class="data.rules_ok ? 'text-green-700 font-medium' : 'text-red-600 font-medium'">
        {{ data.rules_ok ? `✓ ${t('status.mark.ok')}` : `✗ ${t('status.mark.issues')}` }}
      </p>
      <ul v-if="data.issues?.length" class="mt-2 text-sm text-red-600 list-disc pl-5">
        <li v-for="(i, n) in data.issues" :key="n">{{ i }}</li>
      </ul>
      <pre class="mt-4 text-xs bg-slate-50 p-3 rounded overflow-auto">{{ JSON.stringify(data, null, 2) }}</pre>
    </div>
  </div>
</template>
