<script setup>
import { onMounted, ref } from 'vue'
import { api, bpsLabel } from '@/api/client'

const profiles = ref([])
const form = ref({ cidr: '10.0.0.0/16', down: '8mbit', up: '8mbit' })
const err = ref('')

async function load() {
  profiles.value = await api.shaper.profiles()
}

async function save() {
  err.value = ''
  try {
    await api.shaper.putProfile(form.value)
    await load()
  } catch (e) {
    err.value = e.message
  }
}

async function remove(cidr) {
  if (!confirm(`删除模板 ${cidr}?`)) return
  await api.shaper.delProfile(cidr)
  await load()
}

onMounted(load)
</script>

<template>
  <div>
    <h2 class="text-xl font-semibold mb-4">网段模板 (profile_lpm)</h2>

    <div class="card p-4 mb-6 max-w-xl grid gap-2">
      <input v-model="form.cidr" class="input-field font-mono" />
      <div class="grid grid-cols-2 gap-2">
        <input v-model="form.down" class="input-field" />
        <input v-model="form.up" class="input-field" />
      </div>
      <p v-if="err" class="text-red-600 text-sm">{{ err }}</p>
      <button type="button" class="btn-primary w-fit" @click="save">添加/更新</button>
    </div>

    <div class="card table-wrap p-4">
      <table class="data w-full">
        <thead>
          <tr>
            <th>CIDR</th>
            <th>下行</th>
            <th>上行</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="p in profiles" :key="p.cidr">
            <td class="font-mono">{{ p.cidr }}</td>
            <td>{{ bpsLabel(p.down_bps) }}</td>
            <td>{{ bpsLabel(p.up_bps) }}</td>
            <td>
              <button type="button" class="text-red-600 text-xs" @click='remove(p.cidr)'>删除</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
