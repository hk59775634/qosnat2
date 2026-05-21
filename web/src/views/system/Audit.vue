<script setup>
import { onMounted, ref } from 'vue'
import { api } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'

const data = ref(null)

async function load() {
  data.value = await api.system.audit.list()
}

onMounted(load)
</script>

<template>
  <div class="page-stack">
    <PageHeader title="审计日志" description="记录配置变更操作（写入 audit.log）" />
    <p v-if="data" class="text-xs text-slate-500 mb-3 font-mono">{{ data.path }}</p>
    <div class="card overflow-x-auto">
      <table class="data w-full text-sm">
        <thead>
          <tr><th>时间</th><th>用户</th><th>操作</th><th>详情</th></tr>
        </thead>
        <tbody>
          <tr v-for="(e, i) in data?.entries || []" :key="i">
            <td class="text-xs whitespace-nowrap">{{ e.time }}</td>
            <td>{{ e.user }}</td>
            <td class="font-mono text-xs">{{ e.action }}</td>
            <td class="text-xs text-slate-600">{{ e.detail }}</td>
          </tr>
          <tr v-if="!(data?.entries?.length)">
            <td colspan="4" class="text-slate-500 py-4">暂无记录</td>
          </tr>
        </tbody>
      </table>
    </div>
    <button type="button" class="btn-secondary text-sm mt-3" @click="load">刷新</button>
  </div>
</template>
