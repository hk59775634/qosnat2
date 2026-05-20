<script setup>
import { onMounted, ref } from 'vue'
import { api } from '@/api/client'

const managed = ref([])
const live = ref([])
const devLan = ref('')
const devWan = ref('')
const err = ref('')
const ok = ref('')
const form = ref({
  dest: '10.0.0.0/8',
  gateway: '',
  device: '',
  metric: '',
  comment: '',
  enabled: true,
})

async function load() {
  const d = await api.get('/api/v1/routes')
  managed.value = d.managed || []
  live.value = d.live || []
  devLan.value = d.dev_lan || ''
  devWan.value = d.dev_wan || ''
}

async function addRoute() {
  err.value = ''
  ok.value = ''
  try {
    await api.post('/api/v1/routes', {
      dest: form.value.dest,
      gateway: form.value.gateway || undefined,
      device: form.value.device || undefined,
      metric: form.value.metric ? Number(form.value.metric) : 0,
      comment: form.value.comment,
      enabled: form.value.enabled,
    })
    ok.value = '已添加并应用路由'
    await load()
  } catch (e) {
    err.value = e.message
  }
}

async function toggleEnabled(r) {
  await api.put(`/api/v1/routes/${r.id}`, { ...r, enabled: !r.enabled })
  await load()
}

async function remove(id) {
  if (!confirm('删除该托管路由并从内核移除？')) return
  await api.del(`/api/v1/routes/${id}`)
  await load()
}

async function applyAll() {
  err.value = ''
  try {
    await api.post('/api/v1/routes/apply', {})
    ok.value = '已回放全部托管路由'
    await load()
  } catch (e) {
    err.value = e.message
  }
}

function fmtRoute(r) {
  let s = r.dest || r.Dest
  if (r.gateway) s += ` via ${r.gateway}`
  if (r.device) s += ` dev ${r.device}`
  else if (r.dev) s += ` dev ${r.dev}`
  if (r.metric) s += ` metric ${r.metric}`
  return s
}

onMounted(load)
</script>

<template>
  <div>
    <h2 class="text-xl font-semibold mb-4">路由管理</h2>
    <p class="text-sm text-slate-600 mb-4">
      管理宿主机 <code class="text-xs bg-slate-100 px-1 rounded">main</code> 表静态路由（<code class="text-xs">ip route</code>）。
      NAT「策略网段」仍在 <router-link to="/nat/outbound" class="text-blue-600">Outbound NAT</router-link> 配置。
      当前 LAN=<span class="font-mono">{{ devLan }}</span> WAN=<span class="font-mono">{{ devWan }}</span>
    </p>
    <p v-if="ok" class="text-green-700 text-sm mb-2">{{ ok }}</p>
    <p v-if="err" class="text-red-600 text-sm mb-2">{{ err }}</p>

    <div class="card p-4 mb-6 max-w-2xl">
      <h3 class="font-medium mb-3">添加静态路由</h3>
      <div class="grid sm:grid-cols-2 gap-3 text-sm">
        <div>
          <label class="text-xs text-slate-500">目标</label>
          <input v-model="form.dest" class="input-field font-mono" placeholder="default 或 10.0.0.0/8" />
        </div>
        <div>
          <label class="text-xs text-slate-500">网关</label>
          <input v-model="form.gateway" class="input-field font-mono" placeholder="可选" />
        </div>
        <div>
          <label class="text-xs text-slate-500">接口</label>
          <input v-model="form.device" class="input-field font-mono" :placeholder="devLan || 'ens19'" />
        </div>
        <div>
          <label class="text-xs text-slate-500">Metric</label>
          <input v-model="form.metric" type="number" class="input-field" placeholder="0" />
        </div>
        <div class="sm:col-span-2">
          <label class="text-xs text-slate-500">备注</label>
          <input v-model="form.comment" class="input-field" />
        </div>
        <label class="flex items-center gap-2 sm:col-span-2">
          <input v-model="form.enabled" type="checkbox" /> 启用
        </label>
      </div>
      <div class="flex gap-2 mt-4">
        <button type="button" class="btn-primary" @click="addRoute">添加并应用</button>
        <button type="button" class="btn-secondary" @click="applyAll">回放全部托管路由</button>
      </div>
    </div>

    <div class="grid lg:grid-cols-2 gap-6">
      <section class="card table-wrap p-4">
        <h3 class="font-medium mb-3">托管路由</h3>
        <table class="data w-full text-sm">
          <thead>
            <tr>
              <th>目标</th>
              <th>下一跳</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="r in managed" :key="r.id">
              <td class="font-mono text-xs">{{ r.dest }}</td>
              <td class="font-mono text-xs">
                {{ r.gateway || '—' }} / {{ r.device || '—' }}
                <span v-if="r.comment" class="text-slate-400 block">{{ r.comment }}</span>
              </td>
              <td class="whitespace-nowrap text-xs">
                <button type="button" class="text-blue-600 mr-2" @click="toggleEnabled(r)">
                  {{ r.enabled ? '禁用' : '启用' }}
                </button>
                <button type="button" class="text-red-600" @click="remove(r.id)">删除</button>
              </td>
            </tr>
            <tr v-if="!managed.length">
              <td colspan="3" class="text-center text-slate-400 py-4">暂无</td>
            </tr>
          </tbody>
        </table>
      </section>

      <section class="card table-wrap p-4">
        <h3 class="font-medium mb-3">内核 main 表（只读）</h3>
        <table class="data w-full text-xs">
          <thead>
            <tr>
              <th>路由</th>
              <th>协议</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(r, i) in live" :key="i" :class="{ 'bg-green-50': r.managed }">
              <td class="font-mono">{{ fmtRoute(r) }}</td>
              <td>{{ r.protocol || '—' }}</td>
              <td>
                <span v-if="r.managed" class="text-green-700">托管</span>
              </td>
            </tr>
          </tbody>
        </table>
      </section>
    </div>
  </div>
</template>
