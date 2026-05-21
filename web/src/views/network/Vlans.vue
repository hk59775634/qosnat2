<script setup>
import { onMounted, ref } from 'vue'
import { api } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'

const vlans = ref([])
const err = ref('')
const ok = ref('')
const form = ref({ parent: '', vid: 100, ipv4: '', up: true })

async function load() {
  const d = await api.network.vlans.list()
  vlans.value = d.vlans || []
}

async function add() {
  err.value = ''
  try {
    const ipv4 = form.value.ipv4
      .split(/[\n,]+/)
      .map((s) => s.trim())
      .filter(Boolean)
    await api.network.vlans.add({
      parent: form.value.parent,
      vid: form.value.vid,
      ipv4,
      up: form.value.up,
    })
    ok.value = 'VLAN 已创建'
    await load()
  } catch (e) {
    err.value = e.message
  }
}

async function remove(id) {
  if (!confirm('删除 VLAN 子接口？')) return
  await api.network.vlans.del(id)
  await load()
}

onMounted(load)
</script>

<template>
  <div>
    <PageHeader
      title="VLAN"
      description="802.1Q 子接口由 netplan 定义（vlans: link/id/addresses），写入 99-qosnat2.yaml 后 netplan apply。"
    />
    <p v-if="ok" class="text-green-700 text-sm mb-2">{{ ok }}</p>
    <p v-if="err" class="text-red-600 text-sm mb-2">{{ err }}</p>

    <div class="card p-4 mb-6 max-w-lg space-y-3 text-sm">
      <div class="grid sm:grid-cols-2 gap-3">
        <div>
          <label class="text-xs text-slate-500">父接口</label>
          <input v-model="form.parent" class="input-field mt-1 font-mono" placeholder="ens19" />
        </div>
        <div>
          <label class="text-xs text-slate-500">VID</label>
          <input v-model.number="form.vid" type="number" min="1" max="4094" class="input-field mt-1" />
        </div>
        <div class="sm:col-span-2">
          <label class="text-xs text-slate-500">IPv4（可选，每行 CIDR）</label>
          <textarea v-model="form.ipv4" class="input-field mt-1 font-mono h-16" />
        </div>
        <label class="flex items-center gap-2">
          <input v-model="form.up" type="checkbox" /> 创建后 UP
        </label>
      </div>
      <button type="button" class="btn-primary" @click="add">创建 VLAN</button>
    </div>

    <div class="table-wrap card">
      <table class="data w-full text-sm">
        <thead>
          <tr><th>名称</th><th>父接口</th><th>VID</th><th>IPv4</th><th></th></tr>
        </thead>
        <tbody>
          <tr v-for="v in vlans" :key="v.id">
            <td class="font-mono">{{ v.name || `${v.parent}.${v.vid}` }}</td>
            <td class="font-mono">{{ v.parent }}</td>
            <td>{{ v.vid }}</td>
            <td class="font-mono text-xs">{{ (v.ipv4 || []).join(', ') || '—' }}</td>
            <td><button type="button" class="text-red-600 text-xs" @click="remove(v.id)">删除</button></td>
          </tr>
          <tr v-if="!vlans.length">
            <td colspan="5" class="text-center text-slate-400 py-6">无 VLAN</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
