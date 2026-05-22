<script setup>
import { onMounted, ref } from 'vue'
import { api } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'

const vlans = ref([])
const netplanPath = ref('')
const err = ref('')
const ok = ref('')
const editing = ref(null)
const form = ref({ parent: '', vid: 100, ipv4: '', up: true })

function parseIPv4(text) {
  return text
    .split(/[\n,]+/)
    .map((s) => s.trim())
    .filter(Boolean)
}

async function load() {
  const d = await api.network.vlans.list()
  vlans.value = d.vlans || []
  netplanPath.value = d.netplan_path || ''
}

function resetForm() {
  editing.value = null
  form.value = { parent: '', vid: 100, ipv4: '', up: true }
}

function startEdit(v) {
  editing.value = v.id
  form.value = {
    parent: v.parent,
    vid: v.vid,
    ipv4: (v.ipv4 || []).join('\n'),
    up: v.up !== false,
  }
}

async function submit() {
  err.value = ''
  ok.value = ''
  const payload = {
    parent: form.value.parent,
    vid: form.value.vid,
    ipv4: parseIPv4(form.value.ipv4),
    up: form.value.up,
  }
  try {
    if (editing.value) {
      await api.network.vlans.put(editing.value, payload)
      ok.value = 'VLAN 已更新（netplan apply 失败会自动回滚）'
    } else {
      await api.network.vlans.add(payload)
      ok.value = 'VLAN 已创建'
    }
    resetForm()
    await load()
  } catch (e) {
    err.value = e.message
  }
}

async function remove(id) {
  if (!confirm('删除 VLAN 子接口？')) return
  err.value = ''
  try {
    await api.network.vlans.del(id)
    if (editing.value === id) resetForm()
    await load()
  } catch (e) {
    err.value = e.message
  }
}

async function previewNetplan() {
  const d = await api.network.netplan.preview()
  ok.value = `预览 ${d.path || netplanPath.value}（${d.vlans} VLAN / ${d.ifaces} 口）`
}

onMounted(load)
</script>

<template>
  <div class="page-stack">
    <PageHeader
      title="VLAN"
      description="802.1Q 写入 99-qosnat2.yaml；与 cloud-init 50-cloud-init 合并，同名口以本文件为准。apply 失败会回滚 state 与 netplan 备份。"
    />
    <p v-if="ok" class="text-green-700 text-sm mb-2">{{ ok }}</p>
    <p v-if="err" class="text-red-600 text-sm mb-2">{{ err }}</p>

    <div class="card card-body mb-0 space-y-3 text-sm">
      <h3 class="font-medium">{{ editing ? '编辑 VLAN' : '新建 VLAN' }}</h3>
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
      <div class="flex flex-wrap gap-2">
        <button type="button" class="btn-primary" @click="submit">{{ editing ? '保存' : '创建 VLAN' }}</button>
        <button v-if="editing" type="button" class="btn-secondary" @click="resetForm">取消</button>
        <button type="button" class="btn-secondary text-xs" @click="previewNetplan">预览 netplan</button>
      </div>
    </div>

    <div class="table-wrap card">
      <table class="data w-full text-sm">
        <thead>
          <tr><th>名称</th><th>父接口</th><th>VID</th><th>IPv4</th><th></th></tr>
        </thead>
        <tbody>
          <tr v-for="v in vlans" :key="v.id" :class="{ 'bg-blue-50': editing === v.id }">
            <td class="font-mono">{{ v.name || `${v.parent}.${v.vid}` }}</td>
            <td class="font-mono">{{ v.parent }}</td>
            <td>{{ v.vid }}</td>
            <td class="font-mono text-xs">{{ (v.ipv4 || []).join(', ') || '—' }}</td>
            <td class="text-right whitespace-nowrap space-x-2">
              <button type="button" class="text-xs text-blue-600" @click="startEdit(v)">编辑</button>
              <button type="button" class="text-red-600 text-xs" @click="remove(v.id)">删除</button>
            </td>
          </tr>
          <tr v-if="!vlans.length">
            <td colspan="5" class="text-center text-slate-400 py-3">无 VLAN</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
