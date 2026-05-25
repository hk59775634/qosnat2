<script setup>
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'

const { t } = useI18n()
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
    msg.value = t('common.saved')
    await load()
  } catch (e) {
    err.value = e.message
  }
}

async function delRoute(cidr) {
  if (!confirm(`${t('common.delete')} ${cidr}?`)) return
  await api.policyRoutes.del(cidr)
  await load()
}

async function addIP() {
  err.value = ''
  try {
    await api.sharedIPs.add(newIP.value)
    msg.value = t('common.saved')
    newIP.value = ''
    await load()
  } catch (e) {
    err.value = e.message
  }
}

async function delIP(ip) {
  if (!confirm(`${t('common.delete')} ${ip}?`)) return
  await api.sharedIPs.del(ip)
  await load()
}

async function addStatic() {
  err.value = ''
  try {
    await api.staticMappings.add(staticInner.value, staticOuter.value)
    msg.value = t('common.saved')
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
    msg.value = t('common.saved')
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
  <div class="page-stack">
    <PageHeader :title="t('nat.outbound.title')" :description="t('nat.outbound.description')" />
    <p v-if="msg" class="text-green-700 text-sm mb-2">{{ msg }}</p>
    <p v-if="err" class="text-red-600 text-sm mb-2">{{ err }}</p>

    <div class="grid lg:grid-cols-2 gap-6">
      <section class="card p-4">
        <h3 class="font-medium mb-3">{{ t('nat.outbound.policyCidrs') }}</h3>
        <ul class="mb-4 space-y-1">
          <li v-for="c in routes" :key="c" class="flex justify-between items-center text-sm font-mono">
            {{ c }}
            <button type="button" class="text-red-600 text-xs" @click="delRoute(c)">{{ t('common.delete') }}</button>
          </li>
          <li v-if="!routes.length" class="text-slate-400 text-sm">{{ t('nat.outbound.noPolicy') }}</li>
        </ul>
        <div class="flex gap-2">
          <input v-model="newCidr" class="input-field font-mono" placeholder="10.0.0.0/8" />
          <button type="button" class="btn-primary shrink-0" @click="addRoute">{{ t('common.add') }}</button>
        </div>
      </section>

      <section class="card p-4">
        <h3 class="font-medium mb-3">{{ t('nat.outbound.sharedPool') }}</h3>
        <p class="text-xs text-slate-500 mb-2">{{ t('nat.outbound.masqueradeHint') }} ({{ wanDevice || '—' }})</p>
        <p v-if="sharedAutoWAN && !sharedConfigured.length" class="text-xs text-blue-700 bg-blue-50 border border-blue-100 rounded px-2 py-1 mb-2">
          {{ t('nat.outbound.effective') }}: {{ shared[0] }} ({{ t('nat.outbound.auto') }})
        </p>
        <ul class="mb-4 space-y-1">
          <li v-for="ip in shared" :key="ip" class="flex justify-between font-mono text-sm">
            <span>{{ ip }}<span v-if="sharedAutoWAN && !sharedConfigured.includes(ip)" class="text-slate-400 text-xs ml-1">{{ t('nat.outbound.auto') }}</span></span>
            <button
              v-if="sharedConfigured.includes(ip)"
              type="button"
              class="text-red-600 text-xs"
              @click="delIP(ip)"
            >
              {{ t('common.delete') }}
            </button>
          </li>
          <li v-if="!shared.length" class="text-slate-400 text-sm">{{ t('nat.outbound.noShared') }}</li>
        </ul>
        <div class="flex gap-2">
          <input v-model="newIP" class="input-field font-mono" :placeholder="t('nat.outbound.outerPlaceholder')" />
          <button type="button" class="btn-primary shrink-0" @click="addIP">{{ t('common.add') }}</button>
        </div>
      </section>

      <section class="card p-4">
        <h3 class="font-medium mb-3">{{ t('nat.outbound.static1to1') }}</h3>
        <ul class="mb-4 space-y-1 text-sm font-mono">
          <li v-for="(outer, inner) in staticMap" :key="inner" class="flex justify-between">
            <span>{{ inner }} → {{ outer }}</span>
            <button type="button" class="text-red-600 text-xs" @click="delStatic(inner)">{{ t('common.delete') }}</button>
          </li>
          <li v-if="!Object.keys(staticMap).length" class="text-slate-400">{{ t('nat.outbound.noStatic') }}</li>
        </ul>
        <div class="grid grid-cols-2 gap-2 mb-2">
          <input v-model="staticInner" class="input-field font-mono text-xs" :placeholder="t('nat.outbound.innerPlaceholder')" />
          <input v-model="staticOuter" class="input-field font-mono text-xs" :placeholder="t('nat.outbound.outerPlaceholder')" />
        </div>
        <button type="button" class="btn-secondary text-sm" @click="addStatic">{{ t('nat.outbound.addMapping') }}</button>
      </section>

      <section class="card p-4">
        <h3 class="font-medium mb-3">{{ t('nat.outbound.prefixMap') }}</h3>
        <ul class="mb-4 space-y-1 text-sm font-mono">
          <li v-for="(outer, inner) in prefixMap" :key="inner" class="flex justify-between">
            <span>{{ inner }} → {{ outer }}</span>
            <button type="button" class="text-red-600 text-xs" @click="delPrefix(inner)">{{ t('common.delete') }}</button>
          </li>
          <li v-if="!Object.keys(prefixMap).length" class="text-slate-400">{{ t('nat.outbound.noPrefix') }}</li>
        </ul>
        <div class="grid grid-cols-2 gap-2 mb-2">
          <input v-model="prefixInner" class="input-field font-mono text-xs" placeholder="10.0.0.0/24" />
          <input v-model="prefixOuter" class="input-field font-mono text-xs" placeholder="203.0.113.0/24" />
        </div>
        <button type="button" class="btn-secondary text-sm" @click="addPrefix">{{ t('nat.outbound.addMapping') }}</button>
      </section>
    </div>
  </div>
</template>
