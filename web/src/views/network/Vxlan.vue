<script setup>
import { onMounted, ref } from 'vue'
import { api } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'

const tunnels = ref([])
const err = ref('')
const ok = ref('')
const editing = ref(null)
const form = ref({
  vni: 100,
  local: '',
  remote: '',
  port: 4789,
  underlay: '',
  ipv4: '',
  up: true,
})

async function load() {
  const d = await api.network.vxlan.list()
  tunnels.value = d.vxlan_tunnels || []
}

function reset() {
  editing.value = null
  form.value = { vni: 100, local: '', remote: '', port: 4789, underlay: '', ipv4: '', up: true }
}

function startEdit(t) {
  editing.value = t.id
  form.value = {
    vni: t.vni,
    local: t.local,
    remote: t.remote,
    port: t.port || 4789,
    underlay: t.underlay || '',
    ipv4: (t.ipv4 || []).join('\n'),
    up: t.up !== false,
  }
}

async function submit() {
  err.value = ''
  const ipv4 = form.value.ipv4.split(/[\n,]+/).map((s) => s.trim()).filter(Boolean)
  const body = {
    vni: form.value.vni,
    local: form.value.local,
    remote: form.value.remote,
    port: form.value.port,
    underlay: form.value.underlay,
    ipv4,
    up: form.value.up,
  }
  try {
    if (editing.value) {
      await api.network.vxlan.put(editing.value, body)
      ok.value = 'VXLAN 已更新'
    } else {
      await api.network.vxlan.add(body)
      ok.value = 'VXLAN 已创建'
    }
    reset()
    await load()
  } catch (e) {
    err.value = e.message
  }
}

async function remove(id) {
  if (!confirm('删除 VXLAN 隧道？')) return
  await api.network.vxlan.del(id)
  await load()
}

onMounted(load)
</script>

<template>
  <div class="page-stack">
    <PageHeader
      title="VXLAN"
      description="L2 VXLAN overlay 写入 netplan tunnels 段；underlay 为物理口上的 VTEP IP。"
    />
    <p v-if="ok" class="text-green-700 text-sm">{{ ok }}</p>
    <p v-if="err" class="text-red-600 text-sm">{{ err }}</p>

    <div class="card card-body grid sm:grid-cols-2 gap-3 text-sm">
      <div>
        <label class="text-xs text-slate-500">VNI</label>
        <input v-model.number="form.vni" type="number" class="input-field mt-1" />
      </div>
      <div>
        <label class="text-xs text-slate-500">UDP 端口</label>
        <input v-model.number="form.port" type="number" class="input-field mt-1" />
      </div>
      <div>
        <label class="text-xs text-slate-500">本端 VTEP IP</label>
        <input v-model="form.local" class="input-field mt-1 font-mono" />
      </div>
      <div>
        <label class="text-xs text-slate-500">对端 VTEP IP</label>
        <input v-model="form.remote" class="input-field mt-1 font-mono" />
      </div>
      <div>
        <label class="text-xs text-slate-500">Underlay 接口（可选）</label>
        <input v-model="form.underlay" class="input-field mt-1 font-mono" placeholder="ens18" />
      </div>
      <div class="sm:col-span-2">
        <label class="text-xs text-slate-500">Overlay IPv4（每行 CIDR）</label>
        <textarea v-model="form.ipv4" class="input-field mt-1 font-mono h-14" />
      </div>
      <label class="flex items-center gap-2">
        <input v-model="form.up" type="checkbox" /> UP
      </label>
      <div class="sm:col-span-2 flex gap-2">
        <button type="button" class="btn-primary" @click="submit">{{ editing ? '保存' : '创建' }}</button>
        <button v-if="editing" type="button" class="btn-secondary" @click="reset">取消</button>
      </div>
    </div>

    <div class="card overflow-x-auto">
      <table class="data w-full text-sm">
        <thead>
          <tr><th>名称</th><th>VNI</th><th>local</th><th>remote</th><th>IPv4</th><th></th></tr>
        </thead>
        <tbody>
          <tr v-for="t in tunnels" :key="t.id">
            <td class="font-mono">{{ t.name }}</td>
            <td>{{ t.vni }}</td>
            <td class="font-mono text-xs">{{ t.local }}</td>
            <td class="font-mono text-xs">{{ t.remote }}</td>
            <td class="font-mono text-xs">{{ (t.ipv4 || []).join(', ') || '—' }}</td>
            <td class="text-right space-x-2">
              <button type="button" class="text-xs text-blue-600" @click="startEdit(t)">编辑</button>
              <button type="button" class="text-red-600 text-xs" @click="remove(t.id)">删除</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
