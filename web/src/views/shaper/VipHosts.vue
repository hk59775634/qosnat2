<script setup>
import { onMounted, ref } from 'vue'
import { api, bpsLabel } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'

const hosts = ref([])
const form = ref({ ip: '', down: '50mbit', up: '50mbit' })
const err = ref('')
const ok = ref('')

async function load() {
  hosts.value = await api.shaper.hosts.list()
}

async function add() {
  err.value = ''
  ok.value = ''
  try {
    await api.shaper.hosts.put(form.value.ip, { down: form.value.down, up: form.value.up })
    ok.value = `已设置 VIP ${form.value.ip}`
    form.value.ip = ''
    await load()
  } catch (e) {
    err.value = e.message
  }
}

async function remove(ip) {
  if (!confirm(`删除 VIP ${ip}？`)) return
  await api.shaper.hosts.del(ip)
  await load()
}

onMounted(load)
</script>

<template>
  <div class="page-stack">
    <PageHeader
      title="VIP 主机 (/32)"
      description="单 IP 覆盖网段模板 · host_exact + HTB · 最长前缀优先"
    />
    <p v-if="ok" class="text-green-700 text-xs">{{ ok }}</p>
    <p v-if="err" class="text-red-600 text-xs">{{ err }}</p>

    <div class="card card-body flex flex-wrap gap-2 items-end">
      <div class="flex-1 min-w-[8rem]">
        <label class="text-xs text-slate-600">内网 IP</label>
        <input v-model="form.ip" class="input-field mt-0.5 font-mono" placeholder="10.0.0.100" />
      </div>
      <div>
        <label class="text-xs text-slate-600">下行</label>
        <input v-model="form.down" class="input-field mt-0.5 w-24" />
      </div>
      <div>
        <label class="text-xs text-slate-600">上行</label>
        <input v-model="form.up" class="input-field mt-0.5 w-24" />
      </div>
      <button type="button" class="btn-primary" @click="add">添加 / 更新</button>
    </div>

    <div class="card table-wrap card-body !p-2">
      <table class="data w-full">
        <thead>
          <tr>
            <th>IP</th>
            <th>下行</th>
            <th>上行</th>
            <th>速率</th>
            <th class="w-24 text-right">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="h in hosts" :key="h.ip">
            <td class="font-mono">{{ h.ip }}</td>
            <td>{{ h.down || '—' }}</td>
            <td>{{ h.up || '—' }}</td>
            <td class="text-xs text-slate-500">
              {{ bpsLabel(h.down_bps) }} / {{ bpsLabel(h.up_bps) }}
            </td>
            <td class="text-right">
              <button type="button" class="btn-danger" @click="remove(h.ip)">删除</button>
            </td>
          </tr>
          <tr v-if="!hosts.length">
            <td colspan="5" class="text-slate-500 py-3 text-center">暂无 VIP 主机</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
