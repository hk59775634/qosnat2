<script setup>
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/api/client'

const { t } = useI18n()
import PageHeader from '@/components/PageHeader.vue'

const cfg = ref(null)
const status = ref(null)
const peers = ref([])
const err = ref('')
const ok = ref('')
function defaultPeerForm() {
  return {
    name: '',
    allowed_ips: '10.200.0.10/32',
    private_key: '',
    public_key: '',
    endpoint: '',
    persistent_keepalive: 25,
    rate: { down: '8mbit', up: '8mbit' },
  }
}

const peerForm = ref(defaultPeerForm())
const serverEndpoint = ref('')
const activeTab = ref('server')

const tabs = computed(() => [
  { id: 'server', label: t('vpn.wg.tabServer') },
  { id: 'peers', label: t('vpn.wg.tabPeers') },
])

async function load() {
  const d = await api.get('/api/v1/vpn/wireguard')
  cfg.value = d.config
  status.value = d.status
  peers.value = (d.config?.peers || []).map((p) => ({
    ...p,
    rate: p.rate || { down: '', up: '' },
  }))
  serverEndpoint.value = d.config?.server_endpoint || ''
}

function ensurePeerRate(p) {
  if (!p.rate) p.rate = { down: '', up: '' }
}

async function genKeys() {
  err.value = ''
  ok.value = ''
  try {
    const kp = await api.post('/api/v1/vpn/wireguard/keys', {})
    if (!cfg.value) await load()
    cfg.value.private_key = kp.private_key
    cfg.value.public_key = kp.public_key
    ok.value = t('vpn.wg.keysGenerated')
  } catch (e) {
    err.value = e.message
  }
}

async function save() {
  err.value = ''
  ok.value = ''
  try {
    cfg.value.server_endpoint = serverEndpoint.value
    cfg.value.peers = peers.value
    await api.put('/api/v1/vpn/wireguard', cfg.value)
    ok.value = t('vpn.wg.configSaved')
    if (cfg.value.enabled) {
      try {
        await api.post('/api/v1/vpn/wireguard/apply', {})
        ok.value = t('vpn.wg.savedApplied')
      } catch (e) {
        err.value = `${t('vpn.wg.saveApplyFailed')}: ${e.message}`
      }
    } else {
      try {
        await api.post('/api/v1/vpn/wireguard/apply', {})
      } catch {
        /* interface may not exist when down */
      }
      ok.value = t('vpn.wg.savedDisabled')
    }
    await load()
  } catch (e) {
    err.value = e.message
  }
}

async function genPeerKeys() {
  err.value = ''
  try {
    const kp = await api.post('/api/v1/vpn/wireguard/keys', {})
    peerForm.value.private_key = kp.private_key
    peerForm.value.public_key = kp.public_key
    ok.value = t('vpn.wg.peerKeysGenerated')
  } catch (e) {
    err.value = e.message
  }
}

async function addPeer() {
  err.value = ''
  ok.value = ''
  try {
    if (!peerForm.value.name?.trim()) {
      err.value = t('vpn.wg.peerNameRequired')
      return
    }
    const body = {
      name: peerForm.value.name.trim(),
      allowed_ips: [String(peerForm.value.allowed_ips || '').trim()].filter(Boolean),
      persistent_keepalive: peerForm.value.persistent_keepalive,
      rate: peerForm.value.rate,
    }
    const priv = String(peerForm.value.private_key || '').trim()
    const pub = String(peerForm.value.public_key || '').trim()
    if (priv) body.private_key = priv
    if (pub) body.public_key = pub
    if (peerForm.value.endpoint?.trim()) {
      body.endpoint = peerForm.value.endpoint.trim()
    }
    await api.post('/api/v1/vpn/wireguard/peers', body)
    peerForm.value = defaultPeerForm()
    await load()
    ok.value = t('vpn.wg.peerAdded')
  } catch (e) {
    err.value = e.message
  }
}

async function delPeer(name) {
  err.value = ''
  try {
    await api.del(`/api/v1/vpn/wireguard/peers?name=${encodeURIComponent(name)}`)
    await load()
    ok.value = t('vpn.wg.peerDeleted')
  } catch (e) {
    err.value = e.message
  }
}

function downloadConf(name) {
  window.open(`/api/v1/vpn/wireguard/peers/${encodeURIComponent(name)}/conf`, '_blank')
}

onMounted(load)
</script>

<template>
  <div class="page-stack">
    <PageHeader :title="t('vpn.wg.title')" :description="t('vpn.wg.description')" />
    <p v-if="ok" class="text-green-700 text-sm mb-2">{{ ok }}</p>
    <p v-if="err" class="text-red-600 text-sm mb-2">{{ err }}</p>

    <nav v-if="cfg" class="flex flex-wrap gap-1 mb-4 border-b border-slate-200">
      <button
        v-for="tab in tabs"
        :key="tab.id"
        type="button"
        class="px-4 py-2 text-sm border-b-2 -mb-px transition-colors"
        :class="activeTab === tab.id ? 'border-blue-600 text-blue-700 font-medium' : 'border-transparent text-slate-600 hover:text-slate-900'"
        @click="activeTab = tab.id"
      >
        {{ tab.label }}
      </button>
    </nav>

    <div v-if="cfg" class="space-y-6">
      <section v-if="activeTab === 'server'" class="card p-4">
        <h3 class="font-medium mb-3">{{ t('vpn.wg.tabServer') }}</h3>
        <div class="grid sm:grid-cols-2 gap-3 text-sm">
          <label class="flex items-center gap-2">
            <input v-model="cfg.enabled" type="checkbox" /> {{ t('vpn.wg.enable') }}
          </label>
          <div>
            <span class="text-slate-500">{{ t('vpn.wg.statusLabel') }}</span>
            {{
              status?.installed
                ? status?.up
                  ? t('vpn.wg.running')
                  : t('vpn.wg.installed')
                : t('vpn.wg.notInstalled')
            }}
          </div>
          <div>
            <label class="text-xs text-slate-500">{{ t('vpn.wg.iface') }}</label>
            <input v-model="cfg.interface" class="input-field" />
          </div>
          <div>
            <label class="text-xs text-slate-500">{{ t('vpn.wg.listen') }}</label>
            <input v-model.number="cfg.listen_port" type="number" class="input-field" />
          </div>
          <div class="sm:col-span-2">
            <label class="text-xs text-slate-500">{{ t('vpn.wg.tunnelAddress') }}</label>
            <input v-model="cfg.address" class="input-field font-mono" />
          </div>
          <div class="sm:col-span-2">
            <label class="text-xs text-slate-500">{{ t('vpn.wg.serverEndpointPublic') }}</label>
            <input v-model="serverEndpoint" class="input-field font-mono" placeholder="157.15.107.249:51820" />
          </div>
        </div>
        <div class="flex gap-2 mt-4">
          <button type="button" class="btn-secondary" @click="genKeys">{{ t('vpn.wg.genKeys') }}</button>
          <button type="button" class="btn-primary" @click="save">{{ t('vpn.wg.saveApply') }}</button>
        </div>
        <p class="text-xs text-slate-400 mt-2 font-mono truncate">{{ t('vpn.wg.serverPubkey') }}: {{ cfg.public_key || '—' }}</p>
        <p v-if="cfg.server_private_key_set" class="text-xs text-slate-500 mt-1">{{ t('vpn.wg.serverKeyHint') }}</p>
      </section>

      <template v-if="activeTab === 'peers'">
      <section class="card p-4">
        <h3 class="font-medium mb-3">{{ t('vpn.wg.addPeer') }}</h3>
        <p class="text-xs text-slate-500 mb-3">{{ t('vpn.wg.peerFormHint') }}</p>
        <div class="grid sm:grid-cols-2 gap-3 text-sm max-w-3xl">
          <div>
            <label class="text-xs text-slate-500">{{ t('vpn.wg.peerName') }} *</label>
            <input v-model="peerForm.name" class="input-field" placeholder="client-1" />
          </div>
          <div>
            <label class="text-xs text-slate-500">{{ t('vpn.wg.peerAllowedLabel') }}</label>
            <input v-model="peerForm.allowed_ips" class="input-field font-mono text-xs" placeholder="10.200.0.10/32" />
          </div>
          <div>
            <label class="text-xs text-slate-500">{{ t('vpn.wg.rateDown') }}</label>
            <input v-model="peerForm.rate.down" class="input-field font-mono text-xs" placeholder="8mbit" />
          </div>
          <div>
            <label class="text-xs text-slate-500">{{ t('vpn.wg.rateUp') }}</label>
            <input v-model="peerForm.rate.up" class="input-field font-mono text-xs" placeholder="8mbit" />
          </div>
          <div>
            <label class="text-xs text-slate-500">{{ t('vpn.wg.keepaliveSec') }}</label>
            <input v-model.number="peerForm.persistent_keepalive" type="number" class="input-field" />
          </div>
          <div>
            <label class="text-xs text-slate-500">{{ t('vpn.wg.peerEndpointOpt') }}</label>
            <input v-model="peerForm.endpoint" class="input-field font-mono text-xs" placeholder="203.0.113.50:51820" />
          </div>
          <div class="sm:col-span-2">
            <label class="text-xs text-slate-500">{{ t('vpn.wg.clientPrivKey') }}</label>
            <textarea
              v-model="peerForm.private_key"
              class="input-field font-mono text-xs min-h-[4rem]"
              :placeholder="t('vpn.wg.privKeyPh')"
              spellcheck="false"
            />
          </div>
          <div class="sm:col-span-2">
            <label class="text-xs text-slate-500">{{ t('vpn.wg.clientPubKey') }}</label>
            <textarea
              v-model="peerForm.public_key"
              class="input-field font-mono text-xs min-h-[4rem]"
              :placeholder="t('vpn.wg.pubKeyPh')"
              spellcheck="false"
            />
          </div>
        </div>
        <div class="flex flex-wrap gap-2 mt-4">
          <button type="button" class="btn-secondary" @click="genPeerKeys">{{ t('vpn.wg.autoKeypair') }}</button>
          <button type="button" class="btn-primary" @click="addPeer">{{ t('vpn.wg.addPeer') }}</button>
        </div>
      </section>

      <section class="card table-wrap p-4">
        <h3 class="font-medium mb-3">{{ t('vpn.wg.peerList') }}</h3>
        <table class="data w-full">
          <thead>
            <tr>
              <th>{{ t('vpn.wg.colName') }}</th>
              <th>{{ t('vpn.wg.colPubkey') }}</th>
              <th>{{ t('vpn.wg.colAllowed') }}</th>
              <th>{{ t('vpn.wg.colDown') }}</th>
              <th>{{ t('vpn.wg.colUp') }}</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="p in peers" :key="p.name">
              <td>{{ p.name }}</td>
              <td class="font-mono text-xs max-w-xs truncate">{{ p.public_key }}</td>
              <td class="font-mono text-xs">{{ (p.allowed_ips || []).join(', ') }}</td>
              <td>
                <input
                  v-model="p.rate.down"
                  class="input-field w-20 text-xs font-mono"
                  placeholder="8mbit"
                  @focus="ensurePeerRate(p)"
                />
              </td>
              <td>
                <input
                  v-model="p.rate.up"
                  class="input-field w-20 text-xs font-mono"
                  placeholder="8mbit"
                  @focus="ensurePeerRate(p)"
                />
              </td>
              <td class="whitespace-nowrap">
                <button type="button" class="text-blue-600 text-xs mr-2" @click="downloadConf(p.name)">{{ t('vpn.wg.downloadConf') }}</button>
                <button type="button" class="text-red-600 text-xs" @click="delPeer(p.name)">{{ t('common.delete') }}</button>
              </td>
            </tr>
          </tbody>
        </table>
      </section>
      <div class="flex justify-end">
        <button type="button" class="btn-primary" @click="save">{{ t('vpn.wg.saveApply') }}</button>
      </div>
      </template>
    </div>
  </div>
</template>
