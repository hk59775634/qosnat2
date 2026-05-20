<script setup>
import { onMounted, ref } from 'vue'
import { api, bpsLabel } from '@/api/client'

const form = ref({ cidr: '10.0.0.0/16', down: '8mbit', up: '8mbit', mask: 32 })
const profiles = ref([])
const ok = ref('')
const err = ref('')

async function load() {
  try {
    profiles.value = await api.shaper.profiles()
  } catch {
    profiles.value = []
  }
}

async function submit() {
  ok.value = ''
  err.value = ''
  try {
    const res = await api.shaper.wizard(form.value)
    ok.value = res.added
      ? `已添加模板 ${res.cidr}（profile_lpm）`
      : `已更新模板 ${res.cidr}`
    await load()
  } catch (e) {
    err.value = e.message
  }
}

onMounted(load)
</script>

<template>
  <div>
    <h2 class="text-xl font-semibold mb-4">PCQ 向导</h2>
    <p class="text-sm text-slate-600 mb-4">
      向 <code class="text-xs">profile_lpm</code> <strong>添加</strong>网段模板（不覆盖已有策略网段与默认 profile）。
      相同 CIDR 再次提交则更新速率。
    </p>
    <form class="card p-6 max-w-lg space-y-4" @submit.prevent="submit">
      <div>
        <label class="text-sm">主网段 CIDR</label>
        <input v-model="form.cidr" class="input-field mt-1" />
      </div>
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="text-sm">下行</label>
          <input v-model="form.down" class="input-field mt-1" placeholder="8mbit" />
        </div>
        <div>
          <label class="text-sm">上行</label>
          <input v-model="form.up" class="input-field mt-1" placeholder="8mbit" />
        </div>
      </div>
      <div>
        <label class="text-sm">主机掩码</label>
        <input v-model.number="form.mask" type="number" class="input-field mt-1 w-24" />
      </div>
      <p v-if="ok" class="text-green-700 text-sm">{{ ok }}</p>
      <p v-if="err" class="text-red-600 text-sm">{{ err }}</p>
      <button type="submit" class="btn-primary">添加模板</button>
    </form>

    <div v-if="profiles.length" class="card table-wrap p-4 mt-6">
      <h3 class="font-medium mb-3 text-sm">已添加的网段模板</h3>
      <table class="data w-full text-sm">
        <thead>
          <tr>
            <th>CIDR</th>
            <th>下行</th>
            <th>上行</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="p in profiles" :key="p.cidr">
            <td class="font-mono">{{ p.cidr }}</td>
            <td>{{ bpsLabel(p.down_bps) }}</td>
            <td>{{ bpsLabel(p.up_bps) }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
