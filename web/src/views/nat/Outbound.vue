<script setup>
import { onMounted, ref } from 'vue'
import { api } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'

const routes = ref([])
const shared = ref([])
const sharedConfigured = ref([])
const sharedAutoWAN = ref(false)
const wanDevice = ref('')
const staticMap = ref({})
const prefixMap = ref({})
const newCidr = ref('10.0.0.0/8')
const newIP = ref('')
const staticInner = ref('')
const staticOuter = ref('')
const prefixInner = ref('')
const prefixOuter = ref('')
const msg = ref('')
const err = ref('')

async function load() {
  const [r, s, st, px] = await Promise.all([
    api.policyRoutes.list(),
    api.sharedIPs.list(),
    api.staticMappings.list(),
    api.prefixMappings.list(),
  ])
  routes.value = r || []
  shared.value = s?.ips || []
  sharedConfigured.value = s?.configured || []
  sharedAutoWAN.value = !!s?.auto_from_wan
  wanDevice.value = s?.wan_device || ''
  staticMap.value = st || {}
  prefixMap.value = px || {}
}

async function addRoute() {
  err.value = ''
  try {
    await api.policyRoutes.add(newCidr.value)
    msg.value = '已添加策略网段'
    await load()
  } catch (e) {
    err.value = e.message
  }
}

async function delRoute(cidr) {
  if (!confirm(`删除策略网段 ${cidr}?`)) return
  await api.policyRoutes.del(cidr)
  await load()
}

async function addIP() {
  err.value = ''
  try {
    await api.sharedIPs.add(newIP.value)
    msg.value = '已添加共享 IP 并重载 nft'
    newIP.value = ''
    await load()
  } catch (e) {
    err.value = e.message
  }
}

async function delIP(ip) {
  if (!confirm(`删除共享 IP ${ip}?`)) return
  await api.sharedIPs.del(ip)
  await load()
}

async function addStatic() {
  err.value = ''
  try {
    await api.staticMappings.add(staticInner.value, staticOuter.value)
    msg.value = '已添加 1:1 SNAT 映射'
    staticInner.value = ''
    staticOuter.value = ''
    await load()
  } catch (e) {
    err.value = e.message
  }
}

async function delStatic(inner) {
  await api.staticMappings.del(inner)
  await load()
}

async function addPrefix() {
  err.value = ''
  try {
    await api.prefixMappings.add(prefixInner.value, prefixOuter.value)
    msg.value = '已添加网段 SNAT 映射'
    prefixInner.value = ''
    prefixOuter.value = ''
    await load()
  } catch (e) {
    err.value = e.message
  }
}

async function delPrefix(inner) {
  await api.prefixMappings.del(inner)
  await load()
}

onMounted(load)
</script>

<template>
  <div>
    <PageHeader
      title="Outbound NAT"
      description="策略网段 SNAT 池、1:1 映射、网段映射。修改后自动 reload nftables。"
    />
    <p v-if="msg" class="text-green-700 text-sm mb-2">{{ msg }}</p>
    <p v-if="err" class="text-red-600 text-sm mb-2">{{ err }}</p>

    <div class="grid lg:grid-cols-2 gap-6">
      <section class="card p-4">
        <h3 class="font-medium mb-3">策略路由网段 (SNAT 范围)</h3>
        <ul class="mb-4 space-y-1">
          <li v-for="c in routes" :key="c" class="flex justify-between items-center text-sm font-mono">
            {{ c }}
            <button type="button" class="text-red-600 text-xs" @click="delRoute(c)">删除</button>
          </li>
          <li v-if="!routes.length" class="text-slate-400 text-sm">暂无</li>
        </ul>
        <div class="flex gap-2">
          <input v-model="newCidr" class="input-field font-mono" placeholder="10.0.0.0/8" />
          <button type="button" class="btn-primary shrink-0" @click="addRoute">添加</button>
        </div>
      </section>

      <section class="card p-4">
        <h3 class="font-medium mb-3">共享公网 IP 池</h3>
        <p class="text-xs text-slate-500 mb-2">
          多 IP 时按连接轮询 SNAT。未手动添加时自动使用 WAN 口（{{ wanDevice || '—' }}）上的 IPv4。
        </p>
        <p v-if="sharedAutoWAN && !sharedConfigured.length" class="text-xs text-blue-700 bg-blue-50 border border-blue-100 rounded px-2 py-1 mb-2">
          当前生效：{{ shared[0] }}（来自 WAN 口，非持久化）
        </p>
        <ul class="mb-4 space-y-1">
          <li v-for="ip in shared" :key="ip" class="flex justify-between font-mono text-sm">
            <span>{{ ip }}<span v-if="sharedAutoWAN && !sharedConfigured.includes(ip)" class="text-slate-400 text-xs ml-1">自动</span></span>
            <button
              v-if="sharedConfigured.includes(ip)"
              type="button"
              class="text-red-600 text-xs"
              @click="delIP(ip)"
            >
              删除
            </button>
          </li>
          <li v-if="!shared.length" class="text-slate-400 text-sm">WAN 口无 IPv4 时策略网段将走 masquerade</li>
        </ul>
        <div class="flex gap-2">
          <input v-model="newIP" class="input-field font-mono" placeholder="203.0.113.1" />
          <button type="button" class="btn-primary shrink-0" @click="addIP">添加</button>
        </div>
      </section>

      <section class="card p-4">
        <h3 class="font-medium mb-3">1:1 静态 SNAT</h3>
        <p class="text-xs text-slate-500 mb-2">内网单 IP → 公网单 IP</p>
        <ul class="mb-4 space-y-1 text-sm font-mono">
          <li v-for="(outer, inner) in staticMap" :key="inner" class="flex justify-between">
            <span>{{ inner }} → {{ outer }}</span>
            <button type="button" class="text-red-600 text-xs" @click="delStatic(inner)">删除</button>
          </li>
          <li v-if="!Object.keys(staticMap).length" class="text-slate-400">暂无</li>
        </ul>
        <div class="grid grid-cols-2 gap-2 mb-2">
          <input v-model="staticInner" class="input-field font-mono text-xs" placeholder="内网 10.0.0.1" />
          <input v-model="staticOuter" class="input-field font-mono text-xs" placeholder="公网 203.0.113.2" />
        </div>
        <button type="button" class="btn-secondary text-sm" @click="addStatic">添加映射</button>
      </section>

      <section class="card p-4">
        <h3 class="font-medium mb-3">网段 SNAT 映射</h3>
        <p class="text-xs text-slate-500 mb-2">内网 CIDR → 公网 CIDR 前缀映射</p>
        <ul class="mb-4 space-y-1 text-sm font-mono">
          <li v-for="(outer, inner) in prefixMap" :key="inner" class="flex justify-between">
            <span>{{ inner }} → {{ outer }}</span>
            <button type="button" class="text-red-600 text-xs" @click="delPrefix(inner)">删除</button>
          </li>
          <li v-if="!Object.keys(prefixMap).length" class="text-slate-400">暂无</li>
        </ul>
        <div class="grid grid-cols-2 gap-2 mb-2">
          <input v-model="prefixInner" class="input-field font-mono text-xs" placeholder="10.0.0.0/24" />
          <input v-model="prefixOuter" class="input-field font-mono text-xs" placeholder="203.0.113.0/24" />
        </div>
        <button type="button" class="btn-secondary text-sm" @click="addPrefix">添加映射</button>
      </section>
    </div>
  </div>
</template>
