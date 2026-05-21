<script setup>
import { onMounted, ref } from 'vue'
import { api } from '@/api/client'

const data = ref(null)

onMounted(async () => {
  data.value = await api.markPolicy()
})
</script>

<template>
  <div class="page-stack">
    <h2 class="text-xl font-semibold mb-4">Mark 隔离策略</h2>
    <p class="text-sm text-slate-600 mb-4">
      nft 仅可使用 mark 低 30 位；QoS 使用 tc_classid + IFB bpf_redirect，不用 skb->mark 分流。
    </p>
    <div v-if="data" class="card p-4">
      <p :class="data.rules_ok ? 'text-green-700 font-medium' : 'text-red-600 font-medium'">
        {{ data.rules_ok ? '✓ 规则审计通过' : '✗ 规则审计发现问题' }}
      </p>
      <ul v-if="data.issues?.length" class="mt-2 text-sm text-red-600 list-disc pl-5">
        <li v-for="(i, n) in data.issues" :key="n">{{ i }}</li>
      </ul>
      <pre class="mt-4 text-xs bg-slate-50 p-3 rounded overflow-auto">{{ JSON.stringify(data, null, 2) }}</pre>
    </div>
  </div>
</template>
