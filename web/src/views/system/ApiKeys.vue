<script setup>
import { onMounted, ref } from 'vue'
import { api } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'

const keys = ref([])
const name = ref('')
const created = ref(null)
const err = ref('')

async function load() {
  keys.value = await api.system.apiKeys.list()
}

async function add() {
  err.value = ''
  created.value = null
  try {
    const res = await api.system.apiKeys.create(name.value)
    created.value = res
    name.value = ''
    await load()
  } catch (e) {
    err.value = e.message
  }
}

async function remove(id) {
  if (!confirm('删除此 API Key？')) return
  await api.system.apiKeys.del(id)
  await load()
}

onMounted(load)
</script>

<template>
  <div>
    <PageHeader
      title="API 密钥"
      description="用于 X-API-Key 请求头鉴权（自动化/CI）。创建后请立即复制，不会再次显示完整密钥。"
    />
    <p v-if="err" class="text-red-600 text-sm mb-2">{{ err }}</p>
    <div v-if="created?.key" class="card p-4 mb-4 bg-amber-50 border-amber-200">
      <p class="text-sm font-medium text-amber-900 mb-1">新密钥（仅显示一次）</p>
      <code class="text-xs break-all">{{ created.key }}</code>
    </div>
    <div class="card p-4 mb-6 max-w-xl flex gap-2">
      <input v-model="name" class="input-field flex-1" placeholder="名称，如 ci-deploy" />
      <button type="button" class="btn-primary shrink-0" @click="add">创建</button>
    </div>
    <div class="card overflow-x-auto">
      <table class="data w-full text-sm">
        <thead>
          <tr><th>名称</th><th>前缀</th><th>创建时间</th><th></th></tr>
        </thead>
        <tbody>
          <tr v-for="k in keys" :key="k.id">
            <td>{{ k.name }}</td>
            <td class="font-mono text-xs">{{ k.key_prefix }}</td>
            <td class="text-xs text-slate-500">{{ k.created_at }}</td>
            <td>
              <button type="button" class="text-red-600 text-xs" @click="remove(k.id)">删除</button>
            </td>
          </tr>
          <tr v-if="!keys.length">
            <td colspan="4" class="text-slate-500 py-4">暂无密钥</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
