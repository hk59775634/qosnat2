<script setup>
import { onMounted, ref } from 'vue'
import { api } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'

const maps = ref(null)
const programs = ref([])
const err = ref('')
const ok = ref('')
const reloading = ref(false)

async function load() {
  err.value = ''
  try {
    ;[maps.value, programs.value] = await Promise.all([
      api.ebpfMaps(),
      api.ebpfPrograms().catch(() => []),
    ])
  } catch (e) {
    err.value = e.message
  }
}

async function reload() {
  reloading.value = true
  err.value = ''
  ok.value = ''
  try {
    await api.post('/api/v1/ebpf/reload', {})
    ok.value = 'eBPF 已重新附加到 LAN'
    await load()
  } catch (e) {
    err.value = e.message
  } finally {
    reloading.value = false
  }
}

onMounted(load)
</script>

<template>
  <div class="page-stack">
    <PageHeader
      title="eBPF"
      description="Map 状态与 TC 程序挂载。需要已编译并 pin classify.bpf.o。"
    />
    <p v-if="ok" class="text-green-700 text-sm mb-2">{{ ok }}</p>
    <p v-if="err" class="text-red-600 text-sm mb-2">{{ err }}</p>

    <div class="flex gap-2 mb-4">
      <button type="button" class="btn-primary" :disabled="reloading" @click="reload">
        {{ reloading ? '重载中…' : '重新附加 TC' }}
      </button>
      <button type="button" class="btn-secondary" @click="load">刷新</button>
    </div>

    <div class="grid lg:grid-cols-2 gap-4">
      <section class="card p-4">
        <h3 class="font-medium mb-2 text-sm">TC 程序</h3>
        <ul v-if="programs?.length" class="text-sm space-y-2">
          <li v-for="(p, i) in programs" :key="i" class="font-mono text-xs border-b border-slate-100 pb-2">
            <div>{{ p.name }}</div>
            <div class="text-slate-500">{{ p.attached || '未附加' }}</div>
          </li>
        </ul>
        <p v-else class="text-slate-400 text-sm">无程序信息（BPF 未加载）</p>
      </section>
      <section class="card p-4">
        <h3 class="font-medium mb-2 text-sm">Map 摘要</h3>
        <pre class="text-xs overflow-auto bg-slate-50 p-3 rounded max-h-64">{{ JSON.stringify(maps, null, 2) }}</pre>
      </section>
    </div>
  </div>
</template>
