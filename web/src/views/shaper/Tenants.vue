<script setup>
import { onMounted, ref } from 'vue'
import { api } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'

const tenants = ref([])
const err = ref('')
const ok = ref('')
const editing = ref(null)
const form = ref({ name: '', cidrsText: '100.64.0.0/24', down: '8mbit', up: '8mbit', device: '' })

async function load() {
  const d = await api.shaper.tenants.list()
  tenants.value = d.tenants || []
}

function reset() {
  editing.value = null
  form.value = { name: '', cidrsText: '100.64.0.0/24', down: '8mbit', up: '8mbit', device: '' }
}

function startEdit(t) {
  editing.value = t.id
  form.value = {
    name: t.name,
    cidrsText: (t.cidrs || []).join('\n'),
    down: t.down,
    up: t.up,
    device: t.device || '',
  }
}

async function submit() {
  err.value = ''
  const cidrs = form.value.cidrsText.split(/[\n,]+/).map((s) => s.trim()).filter(Boolean)
  const body = {
    name: form.value.name,
    cidrs,
    down: form.value.down,
    up: form.value.up,
    device: form.value.device,
  }
  try {
    if (editing.value) {
      await api.shaper.tenants.put(editing.value, body)
      ok.value = '租户已更新（已同步 QoS profile）'
    } else {
      await api.shaper.tenants.add(body)
      ok.value = '租户已创建'
    }
    reset()
    await load()
  } catch (e) {
    err.value = e.message
  }
}

async function remove(id) {
  if (!confirm('删除租户将移除其下所有 QoS profile？')) return
  await api.shaper.tenants.del(id)
  await load()
}

onMounted(load)
</script>

<template>
  <div class="page-stack">
    <PageHeader
      title="租户 QoS"
      description="一组 CIDR 共享上下行速率，自动展开为多条 profile_lpm 并加入 mirred 列表（P4）。"
    />
    <p v-if="ok" class="text-green-700 text-sm">{{ ok }}</p>
    <p v-if="err" class="text-red-600 text-sm">{{ err }}</p>

    <div class="card card-body space-y-3 text-sm">
      <h3 class="font-medium">{{ editing ? '编辑租户' : '新建租户' }}</h3>
      <input v-model="form.name" class="input-field" placeholder="租户名称" />
      <textarea v-model="form.cidrsText" class="input-field font-mono h-20" placeholder="CIDR 每行一个" />
      <div class="grid sm:grid-cols-2 gap-2">
        <input v-model="form.down" class="input-field" placeholder="下行 8mbit" />
        <input v-model="form.up" class="input-field" placeholder="上行 8mbit" />
      </div>
      <input v-model="form.device" class="input-field font-mono" placeholder="绑定网卡（可选）" />
      <div class="flex gap-2">
        <button type="button" class="btn-primary" @click="submit">{{ editing ? '保存' : '创建' }}</button>
        <button v-if="editing" type="button" class="btn-secondary" @click="reset">取消</button>
      </div>
    </div>

    <div class="card overflow-x-auto">
      <table class="data w-full text-sm">
        <thead>
          <tr><th>名称</th><th>CIDR</th><th>下行</th><th>上行</th><th></th></tr>
        </thead>
        <tbody>
          <tr v-for="t in tenants" :key="t.id">
            <td>{{ t.name }}</td>
            <td class="font-mono text-xs">{{ (t.cidrs || []).join(', ') }}</td>
            <td>{{ t.down }}</td>
            <td>{{ t.up }}</td>
            <td class="text-right space-x-2">
              <button type="button" class="text-blue-600 text-xs" @click="startEdit(t)">编辑</button>
              <button type="button" class="text-red-600 text-xs" @click="remove(t.id)">删除</button>
            </td>
          </tr>
          <tr v-if="!tenants.length">
            <td colspan="5" class="text-center text-slate-400 py-3">暂无租户</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
