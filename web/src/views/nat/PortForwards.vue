<script setup>
import { onMounted, ref } from 'vue'
import { api } from '@/api/client'

const list = ref([])
const form = ref({ proto: 'tcp', wan_port: 22, host_ip: '127.0.0.1', host_port: 22, comment: '' })
const err = ref('')

async function load() {
  list.value = await api.wanForwards.list()
}

async function add() {
  err.value = ''
  try {
    await api.wanForwards.add({ ...form.value })
    await load()
  } catch (e) {
    err.value = e.message
  }
}

onMounted(load)
</script>

<template>
  <div>
    <h2 class="text-xl font-semibold mb-4">端口转发 (DNAT)</h2>
    <p v-if="err" class="text-red-600 text-sm mb-2">{{ err }}</p>

    <div class="card p-4 mb-6">
      <h3 class="font-medium mb-3">添加规则</h3>
      <div class="grid sm:grid-cols-2 lg:grid-cols-3 gap-3">
        <div>
          <label class="text-xs text-slate-500">协议</label>
          <select v-model="form.proto" class="input-field">
            <option value="tcp">tcp</option>
            <option value="udp">udp</option>
          </select>
        </div>
        <div>
          <label class="text-xs text-slate-500">WAN 端口</label>
          <input v-model.number="form.wan_port" type="number" class="input-field" />
        </div>
        <div>
          <label class="text-xs text-slate-500">内网 IP</label>
          <input v-model="form.host_ip" class="input-field" />
        </div>
        <div>
          <label class="text-xs text-slate-500">内网端口</label>
          <input v-model.number="form.host_port" type="number" class="input-field" />
        </div>
        <div class="sm:col-span-2">
          <label class="text-xs text-slate-500">备注</label>
          <input v-model="form.comment" class="input-field" />
        </div>
      </div>
      <button type="button" class="btn-primary mt-3" @click="add">添加</button>
    </div>

    <div class="card p-4 table-wrap">
      <table class="data w-full">
        <thead>
          <tr>
            <th>协议</th>
            <th>WAN</th>
            <th>目标</th>
            <th>备注</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(f, i) in list" :key="i">
            <td>{{ f.proto }}</td>
            <td>{{ f.wan_port }}</td>
            <td class="font-mono">{{ f.host_ip }}:{{ f.host_port }}</td>
            <td>{{ f.comment }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
