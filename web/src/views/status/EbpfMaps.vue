<script setup>
import { onMounted, ref } from 'vue'
import { api } from '@/api/client'

const maps = ref(null)

onMounted(async () => {
  maps.value = await api.ebpfMaps()
})
</script>

<template>
  <div>
    <h2 class="text-xl font-semibold mb-4">eBPF Map 监视器</h2>
    <pre class="card p-4 text-xs overflow-auto bg-slate-50">{{ JSON.stringify(maps, null, 2) }}</pre>
  </div>
</template>
