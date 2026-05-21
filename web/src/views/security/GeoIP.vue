<script setup>
import { onMounted, ref } from 'vue'
import { api } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'

const rules = ref([])
const dataDir = ref('')
const devWan = ref('')
const err = ref('')
const ok = ref('')
const form = ref({
  country: 'CN',
  action: 'drop',
  custom_cidrs: '',
  comment: '',
  enabled: true,
})

async function load() {
  const d = await api.firewall.geoip.list()
  rules.value = d.rules || []
  dataDir.value = d.data_dir || ''
  devWan.value = d.dev_wan || ''
}

async function add() {
  err.value = ''
  try {
    const custom = form.value.custom_cidrs
      .split(/[\n,]+/)
      .map((s) => s.trim())
      .filter(Boolean)
    await api.firewall.geoip.add({
      country: form.value.country,
      action: form.value.action,
      custom_cidrs: custom.length ? custom : undefined,
      comment: form.value.comment,
      enabled: form.value.enabled,
    })
    ok.value = 'Geo 规则已添加'
    await load()
  } catch (e) {
    err.value = e.message
  }
}

async function remove(id) {
  if (!confirm('删除 Geo 规则？')) return
  await api.firewall.geoip.del(id)
  await load()
}

onMounted(load)
</script>

<template>
  <div class="page-stack">
    <PageHeader
      title="GeoIP 阻断"
      description="按国家码阻断从 WAN 进入的流量。CIDR 列表放在数据目录下的 ISO2.cidr 文件，或在规则中填写自定义 CIDR。"
    />
    <p v-if="ok" class="text-green-700 text-sm mb-2">{{ ok }}</p>
    <p v-if="err" class="text-red-600 text-sm mb-2">{{ err }}</p>
    <p class="text-xs text-slate-500 mb-4">
      数据目录：<span class="font-mono">{{ dataDir }}</span>
      · WAN 入方向 <span class="font-mono">{{ devWan }}</span>
    </p>

    <div class="card card-body mb-0 space-y-3 text-sm">
      <h3 class="font-medium">添加规则</h3>
      <div class="grid sm:grid-cols-2 gap-3">
        <div>
          <label class="text-xs text-slate-500">国家码 (ISO2)</label>
          <input v-model="form.country" class="input-field mt-1 font-mono uppercase" maxlength="2" />
        </div>
        <div>
          <label class="text-xs text-slate-500">动作</label>
          <select v-model="form.action" class="input-field mt-1">
            <option value="drop">drop（阻断）</option>
          </select>
        </div>
        <div class="sm:col-span-2">
          <label class="text-xs text-slate-500">自定义 CIDR（可选，每行一条；留空则读文件）</label>
          <textarea v-model="form.custom_cidrs" class="input-field mt-1 font-mono h-20" placeholder="1.2.3.0/24" />
        </div>
        <div class="sm:col-span-2">
          <label class="text-xs text-slate-500">备注</label>
          <input v-model="form.comment" class="input-field mt-1" />
        </div>
        <label class="flex items-center gap-2">
          <input v-model="form.enabled" type="checkbox" /> 启用
        </label>
      </div>
      <button type="button" class="btn-primary" @click="add">添加并应用 nft</button>
    </div>

    <div class="table-wrap card">
      <table class="data w-full text-sm">
        <thead>
          <tr>
            <th>国家</th>
            <th>动作</th>
            <th>自定义 CIDR</th>
            <th>启用</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="r in rules" :key="r.id">
            <td class="font-mono">{{ r.country }}</td>
            <td>{{ r.action }}</td>
            <td class="text-xs font-mono">{{ r.custom_cidrs?.join(', ') || '（文件）' }}</td>
            <td>{{ r.enabled ? '是' : '否' }}</td>
            <td><button type="button" class="text-red-600 text-xs" @click="remove(r.id)">删除</button></td>
          </tr>
          <tr v-if="!rules.length">
            <td colspan="5" class="text-center text-slate-400 py-3">暂无规则</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
