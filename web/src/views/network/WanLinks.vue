<script setup>
import { onMounted, ref } from 'vue'
import { api } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'

const links = ref([])
const devWan = ref('')
const err = ref('')
const ok = ref('')
const form = ref({
  name: 'WAN2',
  device: '',
  gateway: '',
  metric: 200,
  tier: 2,
  weight: 1,
  enabled: true,
})

async function load() {
  const d = await api.network.wanLinks.list()
  links.value = d.wan_links || []
  devWan.value = d.dev_wan || ''
  if (!form.value.device && devWan.value) form.value.device = devWan.value
}

async function add() {
  err.value = ''
  try {
    await api.network.wanLinks.add({ ...form.value })
    ok.value = '已添加并同步默认路由'
    await load()
  } catch (e) {
    err.value = e.message
  }
}

async function remove(id) {
  if (!confirm('删除该 WAN 链路？')) return
  await api.network.wanLinks.del(id)
  await load()
}

onMounted(load)
</script>

<template>
  <div>
    <PageHeader
      title="多 WAN"
      description="Tier 越小越优先；Metric 用于 ip route。启用项会同步为 default 路由（comment qosnat-wan:…）。"
    />
    <p v-if="ok" class="text-green-700 text-sm mb-2">{{ ok }}</p>
    <p v-if="err" class="text-red-600 text-sm mb-2">{{ err }}</p>

    <div class="card p-4 mb-6 max-w-2xl space-y-3 text-sm">
      <div class="grid sm:grid-cols-2 gap-3">
        <div>
          <label class="text-xs text-slate-500">名称</label>
          <input v-model="form.name" class="input-field mt-1" />
        </div>
        <div>
          <label class="text-xs text-slate-500">网卡</label>
          <input v-model="form.device" class="input-field mt-1 font-mono" />
        </div>
        <div>
          <label class="text-xs text-slate-500">网关</label>
          <input v-model="form.gateway" class="input-field mt-1 font-mono" />
        </div>
        <div>
          <label class="text-xs text-slate-500">Metric</label>
          <input v-model.number="form.metric" type="number" class="input-field mt-1" />
        </div>
        <div>
          <label class="text-xs text-slate-500">Tier</label>
          <input v-model.number="form.tier" type="number" class="input-field mt-1" />
        </div>
        <div>
          <label class="text-xs text-slate-500">权重</label>
          <input v-model.number="form.weight" type="number" class="input-field mt-1" />
        </div>
        <label class="flex items-center gap-2 sm:col-span-2">
          <input v-model="form.enabled" type="checkbox" /> 启用
        </label>
      </div>
      <button type="button" class="btn-primary" @click="add">添加 WAN</button>
    </div>

    <div class="table-wrap card">
      <table class="data w-full text-sm">
        <thead>
          <tr>
            <th>名称</th>
            <th>设备</th>
            <th>网关</th>
            <th>Tier</th>
            <th>Metric</th>
            <th>权重</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="w in links" :key="w.id">
            <td>{{ w.name }}</td>
            <td class="font-mono">{{ w.device }}</td>
            <td class="font-mono">{{ w.gateway }}</td>
            <td>{{ w.tier }}</td>
            <td>{{ w.metric }}</td>
            <td>{{ w.weight }}</td>
            <td><button type="button" class="text-red-600 text-xs" @click="remove(w.id)">删除</button></td>
          </tr>
          <tr v-if="!links.length">
            <td colspan="7" class="text-center text-slate-400 py-6">未配置额外 WAN</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
