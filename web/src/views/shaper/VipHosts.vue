<script setup>
import { onMounted, ref } from 'vue'
import { api, bpsLabel } from '@/api/client'

const hosts = ref([])
const form = ref({ ip: '', down: '50mbit', up: '50mbit' })
const err = ref('')

async function load() {
  hosts.value = await api.shaper.hosts()
}

async function save() {
  err.value = ''
  try {
    await api.shaper.putHost(form.value.ip, { down: form.value.down, up: form.value.up })
    form.value.ip = ''
    await load()
  } catch (e) {
    err.value = e.message
  }
}

async function remove(ip) {
  if (!confirm(`删除 VIP ${ip}?`)) return
  await api.shaper.delHost(ip)
  await load()
}

onMounted(load)
</script>

<template>
  <div>
    <h2 class="text-xl font-semibold mb-4">VIP 主机 (/32)</h2>
    <p class="text-sm text-slate-600 mb-4">覆盖 profile 默认速率，写入 host_exact + HTB。</p>

    <div class="card p-4 mb-6 max-w-xl">
      <div class="grid gap-3">
        <input v-model="form.ip" class="input-field font-mono" placeholder="10.0.18.83" />
        <div class="grid grid-cols-2 gap-2">
          <input v-model="form.down" class="input-field" placeholder="50mbit" />
          <input v-model="form.up" class="input-field" placeholder="50mbit" />
        </div>
        <p v-if="err" class="text-red-600 text-sm">{{ err }}</p>
        <button type="button" class="btn-primary w-fit" @click="save">保存 VIP</button>
      </div>
    </div>

    <div class="card table-wrap p-4">
      <table class="data w-full">
        <thead>
          <tr>
            <th>IP</th>
            <th>下行</th>
            <th>上行</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="h in hosts" :key="h.ip">
            <td class="font-mono">{{ h.ip }}</td>
            <td>{{ bpsLabel(h.down_bps) }}</td>
            <td>{{ bpsLabel(h.up_bps) }}</td>
            <td>
              <button type="button" class="text-red-600 text-xs" @click='remove(h.ip)'>删除</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
