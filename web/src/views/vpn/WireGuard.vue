<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'
import SnmpTrafficChart from '@/components/SnmpTrafficChart.vue'

const { t } = useI18n()

const instances = ref([])
const selectedId = ref('default')
const cfg = ref(null)
const status = ref(null)
const peers = ref([])
const err = ref('')
const ok = ref('')
function defaultPeerForm() {
  return {
    name: '',
    allowed_ips: '198.19.0.10/32',
    private_key: '',
    public_key: '',
    endpoint: '',
    persistent_keepalive: 25,
    rate: { down: '8mbit', up: '8mbit' },
  }
}

const peerForm = ref(defaultPeerForm())
const peerModalOpen = ref(false)
const peerModalErr = ref('')
const peerEditing = ref(false)
const peerHadPrivateKey = ref(false)
const serverEndpoint = ref('')
const activeTab = ref('server')

const trafficModal = ref(null)
const trafficData = ref(null)
const trafficPeriod = ref('7d')
const trafficLoading = ref(false)
const trafficErr = ref('')
const trafficPoll = ref(null)
const trafficLastUpdated = ref(null)
const TRAFFIC_POLL_MS = 5000
const trafficLiveEnabled = ref(false)
const trafficLivePoll = ref(null)
const trafficLiveSeries = ref([])
const trafficLiveCounters = ref(null)
const trafficLiveErr = ref('')
const TRAFFIC_LIVE_POLL_MS = 2000
const TRAFFIC_LIVE_WINDOW_SEC = 300

const peerStatusByName = ref({})
const peerStatusPoll = ref(null)
const PEER_STATUS_POLL_MS = 5000
const statusModalOpen = ref(false)
const statusModalPeer = ref(null) // null = instance status; string = peer name
const statusModalLoading = ref(false)
const statusModalErr = ref('')
const statusModalData = ref(null)

const tabs = computed(() => [
  { id: 'server', label: t('vpn.wg.tabServer') },
  { id: 'peers', label: t('vpn.wg.tabPeers') },
])

const selectedInstanceMeta = computed(() => instances.value.find((x) => x.id === selectedId.value) || null)

const instanceRuntimeUp = computed(() => {
  if (status.value?.up) return true
  return !!selectedInstanceMeta.value?.status?.up
})

const instanceRuntimeInstalled = computed(() => {
  if (status.value) return !!status.value.installed
  return !!selectedInstanceMeta.value?.status?.installed
})

function peerRuntime(name) {
  return peerStatusByName.value[name] || null
}

function instanceApiBase() {
  const id = encodeURIComponent(selectedId.value || 'default')
  return `/api/v1/vpn/wireguard/instances/${id}`
}

async function loadInstances() {
  const d = await api.get('/api/v1/vpn/wireguard/instances')
  instances.value = d.instances || []
  if (!instances.value.some((x) => x.id === selectedId.value) && instances.value.length) {
    selectedId.value = instances.value[0].id
  }
}

async function load() {
  await loadInstances()
  const d = await api.get(instanceApiBase())
  cfg.value = d.config
  status.value = d.status
  peers.value = (d.config?.peers || []).map((p) => ({
    ...p,
    rate: p.rate || { down: '', up: '' },
  }))
  serverEndpoint.value = d.config?.server_endpoint || ''
  if (activeTab.value === 'peers') {
    await loadPeerStatuses(true)
  }
}

async function loadPeerStatuses(silent = false) {
  if (!selectedId.value) return
  try {
    const d = await api.get(`${instanceApiBase()}/peers/status`)
    if (d?.up != null || d?.installed != null) {
      status.value = {
        ...(status.value || {}),
        installed: d.installed,
        up: d.up,
        raw: d.raw || status.value?.raw,
      }
    }
    const map = {}
    for (const p of d.peers || []) {
      if (p?.name) map[p.name] = p
    }
    peerStatusByName.value = map
    if (statusModalOpen.value && statusModalPeer.value) {
      statusModalData.value = map[statusModalPeer.value] || statusModalData.value
    }
  } catch (e) {
    if (!silent) err.value = e.message
  }
}

function startPeerStatusPoll() {
  stopPeerStatusPoll()
  if (activeTab.value !== 'peers') return
  peerStatusPoll.value = setInterval(() => loadPeerStatuses(true), PEER_STATUS_POLL_MS)
}

function stopPeerStatusPoll() {
  if (peerStatusPoll.value) {
    clearInterval(peerStatusPoll.value)
    peerStatusPoll.value = null
  }
}

async function openInstanceStatusModal() {
  statusModalPeer.value = null
  statusModalOpen.value = true
  statusModalLoading.value = true
  statusModalErr.value = ''
  statusModalData.value = null
  try {
    const d = await api.get(`${instanceApiBase()}/peers/status`)
    statusModalData.value = d
    status.value = {
      installed: d.installed,
      up: d.up,
      raw: d.raw,
    }
  } catch (e) {
    statusModalErr.value = e.message
  } finally {
    statusModalLoading.value = false
  }
}

async function openPeerStatusModal(name) {
  statusModalPeer.value = name
  statusModalOpen.value = true
  statusModalLoading.value = true
  statusModalErr.value = ''
  statusModalData.value = peerRuntime(name)
  try {
    await loadPeerStatuses(true)
    statusModalData.value = peerRuntime(name)
    if (!statusModalData.value) {
      statusModalErr.value = t('vpn.wg.peerStatusNotFound')
    }
  } catch (e) {
    statusModalErr.value = e.message
  } finally {
    statusModalLoading.value = false
  }
}

function closeStatusModal() {
  statusModalOpen.value = false
  statusModalPeer.value = null
  statusModalData.value = null
  statusModalErr.value = ''
}

function formatHandshake(unix) {
  if (!unix || unix <= 0) return t('vpn.wg.handshakeNever')
  try {
    return new Date(unix * 1000).toLocaleString()
  } catch {
    return String(unix)
  }
}

watch(selectedId, async () => {
  err.value = ''
  ok.value = ''
  peerStatusByName.value = {}
  closeStatusModal()
  closePeerTraffic()
  closePeerModal()
  await load()
})

watch(activeTab, async (tab) => {
  if (tab === 'peers') {
    await loadPeerStatuses(true)
    startPeerStatusPoll()
  } else {
    stopPeerStatusPoll()
  }
})

async function createInstance() {
  err.value = ''
  ok.value = ''
  try {
    const name = window.prompt(t('vpn.wg.newInstanceName'), '') ?? ''
    const body = { name: name.trim() || 'wg', mode: 'server' }
    const res = await api.post('/api/v1/vpn/wireguard/instances', body)
    ok.value = t('vpn.wg.instanceCreated')
    selectedId.value = res.id
    await load()
  } catch (e) {
    err.value = e.message
  }
}

async function deleteInstance() {
  if (instances.value.length <= 1) return
  err.value = ''
  ok.value = ''
  if (!window.confirm(t('vpn.wg.confirmDeleteInstance'))) return
  try {
    await api.del(instanceApiBase())
    ok.value = t('vpn.wg.instanceDeleted')
    selectedId.value = 'default'
    await load()
  } catch (e) {
    err.value = e.message
  }
}

function ensurePeerRate(p) {
  if (!p.rate) p.rate = { down: '', up: '' }
}

async function genKeys() {
  err.value = ''
  ok.value = ''
  try {
    const kp = await api.post(`${instanceApiBase()}/keys`, {})
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
    await api.put(instanceApiBase(), cfg.value)
    ok.value = t('vpn.wg.configSaved')
    if (cfg.value.enabled) {
      try {
        await api.post(`${instanceApiBase()}/apply`, {})
        ok.value = t('vpn.wg.savedApplied')
      } catch (e) {
        err.value = `${t('vpn.wg.saveApplyFailed')}: ${e.message}`
      }
    } else {
      try {
        await api.post(`${instanceApiBase()}/apply`, {})
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
  peerModalErr.value = ''
  err.value = ''
  try {
    const kp = await api.post(`${instanceApiBase()}/peer-genkey`, {})
    peerForm.value.private_key = kp.private_key
    peerForm.value.public_key = kp.public_key
    ok.value = t('vpn.wg.peerKeysGenerated')
  } catch (e) {
    peerModalErr.value = e.message
  }
}

async function savePeer() {
  peerModalErr.value = ''
  err.value = ''
  ok.value = ''
  try {
    if (!peerForm.value.name?.trim()) {
      peerModalErr.value = t('vpn.wg.peerNameRequired')
      return
    }
    const body = {
      name: peerForm.value.name.trim(),
      allowed_ips: String(peerForm.value.allowed_ips || '')
        .split(/[\s,]+/)
        .map((s) => s.trim())
        .filter(Boolean),
      persistent_keepalive: peerForm.value.persistent_keepalive,
      rate: peerForm.value.rate,
    }
    const priv = String(peerForm.value.private_key || '').trim()
    const pub = String(peerForm.value.public_key || '').trim()
    if (priv) body.private_key = priv
    if (pub) body.public_key = pub
    if (peerForm.value.endpoint?.trim()) {
      body.endpoint = peerForm.value.endpoint.trim()
    } else if (peerEditing.value) {
      body.endpoint = ''
    }
    const wasEditing = peerEditing.value
    const res = await api.post(`${instanceApiBase()}/peers`, body)
    peerForm.value = defaultPeerForm()
    closePeerModal()
    await load()
    ok.value = wasEditing || res?.updated ? t('vpn.wg.peerUpdated') : t('vpn.wg.peerAdded')
  } catch (e) {
    peerModalErr.value = e.message
  }
}

async function delPeer(name) {
  err.value = ''
  if (!window.confirm(t('vpn.wg.confirmDelete', { name }))) return
  try {
    await api.del(`${instanceApiBase()}/peers?name=${encodeURIComponent(name)}`)
    await load()
    ok.value = t('vpn.wg.peerDeleted')
  } catch (e) {
    err.value = e.message
  }
}

function downloadConf(name) {
  window.open(`${instanceApiBase()}/peers/${encodeURIComponent(name)}/conf`, '_blank')
}

function applyTrafficToPeerList(data) {
  if (!data?.peer) return
  const rx = Number(data.summary?.total_rx_bytes) || 0
  const tx = Number(data.summary?.total_tx_bytes) || 0
  const idx = peers.value.findIndex((p) => p.name === data.peer)
  if (idx < 0) return
  peers.value[idx] = {
    ...peers.value[idx],
    total_rx_bytes: rx,
    total_tx_bytes: tx,
    total_bytes: rx + tx,
  }
}

async function loadPeerTraffic(silent = false) {
  if (!trafficModal.value) return
  if (!silent) {
    trafficLoading.value = true
    trafficErr.value = ''
  }
  try {
    const q = new URLSearchParams({
      name: trafficModal.value,
      period: trafficPeriod.value,
    })
    const data = await api.get(`${instanceApiBase()}/peers/traffic?${q}`)
    trafficData.value = data
    trafficLastUpdated.value = Date.now()
    applyTrafficToPeerList(data)
    if (trafficLiveEnabled.value && !data.current) {
      stopTrafficLivePoll()
      trafficLiveEnabled.value = false
      resetTrafficLive()
      trafficLiveErr.value = t('vpn.wg.trafficLiveNeedCounters')
    }
  } catch (e) {
    if (!silent) {
      trafficErr.value = e.message
      trafficData.value = null
    }
  } finally {
    if (!silent) trafficLoading.value = false
  }
}

function parseCounterBytes(v) {
  if (v == null || v === '') return 0
  const n = Number(v)
  if (Number.isFinite(n) && n >= 0) return n
  const s = String(v).trim()
  if (/^\d+$/.test(s)) return Number(s)
  return 0
}

function resetTrafficLive() {
  trafficLiveSeries.value = []
  trafficLiveCounters.value = null
  trafficLiveErr.value = ''
}

function stopTrafficLivePoll() {
  if (trafficLivePoll.value) {
    clearInterval(trafficLivePoll.value)
    trafficLivePoll.value = null
  }
}

function appendTrafficLiveSample(current) {
  const rx = parseCounterBytes(current?.RX ?? current?.rx)
  const tx = parseCounterBytes(current?.TX ?? current?.tx)
  const now = Date.now()
  const ts = Math.floor(now / 1000)
  const prev = trafficLiveCounters.value
  if (prev) {
    const dtSec = Math.max(0.5, (now - prev.at) / 1000)
    const drx = rx >= prev.rx ? rx - prev.rx : rx
    const dtx = tx >= prev.tx ? tx - prev.tx : tx
    const rxMbps = (drx * 8) / (dtSec * 1_000_000)
    const txMbps = (dtx * 8) / (dtSec * 1_000_000)
    const cut = ts - TRAFFIC_LIVE_WINDOW_SEC
    trafficLiveSeries.value = [
      ...trafficLiveSeries.value,
      { ts, rx_mbps: rxMbps, tx_mbps: txMbps },
    ].filter((p) => p.ts >= cut)
  }
  trafficLiveCounters.value = { rx, tx, at: now }
}

async function pollTrafficLive() {
  if (!trafficModal.value || !trafficLiveEnabled.value) return
  try {
    const q = new URLSearchParams({
      name: trafficModal.value,
      period: '24h',
    })
    const data = await api.get(`${instanceApiBase()}/peers/traffic?${q}`)
    if (trafficData.value) {
      trafficData.value.online = data.online
      trafficData.value.current = data.current
    }
    if (!data.current) {
      trafficLiveErr.value = t('vpn.wg.trafficLiveNeedCounters')
      stopTrafficLivePoll()
      trafficLiveEnabled.value = false
      resetTrafficLive()
      return
    }
    trafficLiveErr.value = ''
    appendTrafficLiveSample(data.current)
    trafficLastUpdated.value = Date.now()
  } catch (e) {
    trafficLiveErr.value = e.message || t('vpn.wg.trafficLiveFailed')
  }
}

function startTrafficLivePoll() {
  stopTrafficLivePoll()
  resetTrafficLive()
  pollTrafficLive()
  trafficLivePoll.value = setInterval(pollTrafficLive, TRAFFIC_LIVE_POLL_MS)
}

const trafficChartSeries = computed(() => {
  if (trafficLiveEnabled.value) return trafficLiveSeries.value
  return trafficData.value?.series || []
})

/** 有 wg 内核 transfer 计数即可做实时曲线（握手「在线」可能为 false） */
const trafficLiveCanEnable = computed(() => {
  const c = trafficData.value?.current
  if (!c || typeof c !== 'object') return false
  const rx = c.RX ?? c.rx ?? c.raw_rx
  const tx = c.TX ?? c.tx ?? c.raw_tx
  return rx != null && tx != null
})

function onTrafficLiveToggle() {
  if (trafficLiveEnabled.value) {
    if (!trafficLiveCanEnable.value) {
      trafficLiveEnabled.value = false
      trafficLiveErr.value = t('vpn.wg.trafficLiveNeedCounters')
      return
    }
    startTrafficLivePoll()
  } else {
    stopTrafficLivePoll()
    resetTrafficLive()
  }
}

function stopTrafficPoll() {
  if (trafficPoll.value) {
    clearInterval(trafficPoll.value)
    trafficPoll.value = null
  }
}

function startTrafficPoll() {
  stopTrafficPoll()
  trafficPoll.value = setInterval(() => loadPeerTraffic(true), TRAFFIC_POLL_MS)
}

function openPeerTraffic(name) {
  trafficModal.value = name
  trafficPeriod.value = '7d'
  trafficData.value = null
  trafficErr.value = ''
  trafficLastUpdated.value = null
  trafficLiveEnabled.value = false
  resetTrafficLive()
  stopTrafficLivePoll()
  loadPeerTraffic()
  startTrafficPoll()
}

function closePeerTraffic() {
  stopTrafficPoll()
  stopTrafficLivePoll()
  trafficLiveEnabled.value = false
  resetTrafficLive()
  trafficModal.value = null
  trafficData.value = null
  trafficErr.value = ''
  trafficLastUpdated.value = null
}

function formatTrafficUpdatedAt(ts) {
  if (!ts) return ''
  return new Date(ts).toLocaleTimeString()
}

function formatBytes(n) {
  const num = Number(n)
  if (!Number.isFinite(num) || num < 0) return '—'
  if (num === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let v = num
  let i = 0
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  const digits = v >= 100 ? 0 : v >= 10 ? 1 : 2
  return `${v.toFixed(digits).replace(/\.?0+$/, '')} ${units[i]}`
}

function formatTraffic(pretty, raw) {
  if (pretty != null && String(pretty).trim() !== '') return String(pretty).trim()
  if (raw == null || raw === '') return '—'
  if (typeof raw === 'string' && /[kmgt]b/i.test(raw) && !/^\d+$/.test(raw.trim())) return raw.trim()
  return formatBytes(raw)
}

watch(trafficPeriod, () => {
  if (trafficModal.value && !trafficLiveEnabled.value) loadPeerTraffic()
})

function openAddPeerModal() {
  peerModalErr.value = ''
  peerEditing.value = false
  peerHadPrivateKey.value = false
  peerForm.value = defaultPeerForm()
  peerModalOpen.value = true
}

function openEditPeerModal(p) {
  peerModalErr.value = ''
  peerEditing.value = true
  peerHadPrivateKey.value = !!p.private_key_set
  peerForm.value = {
    name: p.name || '',
    allowed_ips: (p.allowed_ips || []).join(', ') || '',
    private_key: '',
    public_key: p.public_key || '',
    endpoint: p.endpoint || '',
    persistent_keepalive: p.persistent_keepalive || 25,
    rate: {
      down: p.rate?.down || '',
      up: p.rate?.up || '',
    },
  }
  peerModalOpen.value = true
}

function closePeerModal() {
  peerModalOpen.value = false
  peerModalErr.value = ''
  peerEditing.value = false
  peerHadPrivateKey.value = false
}

onMounted(async () => {
  await load()
  if (activeTab.value === 'peers') startPeerStatusPoll()
})
onUnmounted(() => {
  stopTrafficPoll()
  stopTrafficLivePoll()
  stopPeerStatusPoll()
})
</script>

<template>
  <div class="page-stack">
    <PageHeader :title="t('vpn.wg.title')" :description="t('vpn.wg.description')" :ok="ok" :err="err" />

    <div v-if="instances.length" class="flex flex-wrap items-center gap-2 mb-4 text-sm">
      <label class="text-slate-600 shrink-0">{{ t('vpn.wg.instanceLabel') }}</label>
      <select v-model="selectedId" class="input-field w-auto min-w-[12rem]">
        <option v-for="x in instances" :key="x.id" :value="x.id">
          {{ x.name || x.id }} · {{ x.interface }}
          {{ x.enabled ? '●' : '○' }}
          {{ x.status?.up ? t('vpn.wg.runtimeUpShort') : t('vpn.wg.runtimeDownShort') }}
        </option>
      </select>
      <span
        class="text-xs px-2 py-0.5 rounded font-medium"
        :class="
          instanceRuntimeUp
            ? 'bg-emerald-100 text-emerald-800'
            : instanceRuntimeInstalled
              ? 'bg-amber-50 text-amber-800'
              : 'bg-slate-100 text-slate-600'
        "
      >
        {{
          instanceRuntimeUp
            ? t('vpn.wg.running')
            : instanceRuntimeInstalled
              ? t('vpn.wg.installedNotUp')
              : t('vpn.wg.notInstalled')
        }}
      </span>
      <button type="button" class="btn-secondary text-sm" @click="openInstanceStatusModal">
        {{ t('vpn.wg.viewStatus') }}
      </button>
      <button type="button" class="btn-secondary text-sm" @click="createInstance">{{ t('vpn.wg.newInstance') }}</button>
      <button
        v-if="instances.length > 1"
        type="button"
        class="text-sm text-red-600 hover:underline"
        @click="deleteInstance"
      >
        {{ t('vpn.wg.deleteInstance') }}
      </button>
    </div>

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
            <label class="text-xs text-slate-500 block mb-1">{{ t('vpn.wg.modeLabel') }}</label>
            <select v-model="cfg.mode" class="input-field">
              <option value="server">{{ t('vpn.wg.modeServer') }}</option>
              <option value="client">{{ t('vpn.wg.modeClient') }}</option>
            </select>
          </div>
          <div class="sm:col-span-2 flex flex-wrap items-center gap-3">
            <div>
              <span class="text-slate-500 mr-2">{{ t('vpn.wg.statusLabel') }}</span>
              <span
                class="text-xs px-2 py-0.5 rounded font-medium"
                :class="
                  instanceRuntimeUp
                    ? 'bg-emerald-100 text-emerald-800'
                    : instanceRuntimeInstalled
                      ? 'bg-amber-50 text-amber-800'
                      : 'bg-slate-100 text-slate-600'
                "
              >
                {{
                  instanceRuntimeUp
                    ? t('vpn.wg.running')
                    : instanceRuntimeInstalled
                      ? t('vpn.wg.installedNotUp')
                      : t('vpn.wg.notInstalled')
                }}
              </span>
              <span class="text-xs text-slate-500 ml-2">
                {{ cfg.enabled ? t('vpn.wg.configEnabled') : t('vpn.wg.configDisabled') }}
              </span>
            </div>
            <button type="button" class="btn-secondary text-xs" @click="openInstanceStatusModal">
              {{ t('vpn.wg.viewStatus') }}
            </button>
          </div>
          <div>
            <label class="text-xs text-slate-500">{{ t('vpn.wg.iface') }}</label>
            <input v-model="cfg['interface']" class="input-field" />
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
      <section class="card table-wrap p-4">
        <div class="flex flex-wrap items-center justify-between gap-2 mb-3">
          <h3 class="font-medium">{{ t('vpn.wg.peerList') }}</h3>
          <button type="button" class="btn-primary text-sm" @click="openAddPeerModal">{{ t('vpn.wg.addPeer') }}</button>
        </div>
        <table class="data w-full">
          <thead>
            <tr>
              <th>{{ t('vpn.wg.colName') }}</th>
              <th>{{ t('vpn.wg.colStatus') }}</th>
              <th>{{ t('vpn.wg.colEndpoint') }}</th>
              <th>{{ t('vpn.wg.colPubkey') }}</th>
              <th>{{ t('vpn.wg.colAllowed') }}</th>
              <th>{{ t('vpn.wg.colTotalRx') }}</th>
              <th>{{ t('vpn.wg.colTotalTx') }}</th>
              <th>{{ t('vpn.wg.colDown') }}</th>
              <th>{{ t('vpn.wg.colUp') }}</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="p in peers" :key="p.name">
              <td>{{ p.name }}</td>
              <td>
                <span
                  class="text-xs px-2 py-0.5 rounded"
                  :class="
                    peerRuntime(p.name)?.online
                      ? 'bg-emerald-100 text-emerald-800'
                      : 'bg-slate-100 text-slate-600'
                  "
                >
                  {{
                    peerRuntime(p.name)?.online
                      ? t('ocserv.onlineNow')
                      : t('ocserv.offlineNow')
                  }}
                </span>
              </td>
              <td class="font-mono text-xs max-w-[10rem] truncate">
                {{ peerRuntime(p.name)?.endpoint || p.endpoint || '—' }}
              </td>
              <td class="font-mono text-xs max-w-xs truncate">{{ p.public_key }}</td>
              <td class="font-mono text-xs">{{ (p.allowed_ips || []).join(', ') }}</td>
              <td class="font-mono text-xs whitespace-nowrap">{{ formatBytes(p.total_rx_bytes) }}</td>
              <td class="font-mono text-xs whitespace-nowrap">{{ formatBytes(p.total_tx_bytes) }}</td>
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
                <button type="button" class="text-sky-700 text-xs mr-2" @click="openPeerStatusModal(p.name)">
                  {{ t('vpn.wg.viewStatus') }}
                </button>
                <button type="button" class="text-indigo-600 text-xs mr-2" @click="openEditPeerModal(p)">{{ t('common.edit') }}</button>
                <button type="button" class="text-emerald-700 text-xs mr-2" @click="openPeerTraffic(p.name)">{{ t('vpn.wg.traffic') }}</button>
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

    <Teleport to="body">
      <div
        v-if="statusModalOpen"
        class="fixed inset-0 z-[60] flex items-center justify-center p-4 bg-black/40"
        role="presentation"
        @click.self="closeStatusModal"
      >
        <div
          class="bg-white rounded-xl shadow-xl w-full max-w-2xl max-h-[90vh] overflow-y-auto border border-slate-200"
          role="dialog"
          aria-modal="true"
          @click.stop
        >
          <div class="flex items-center justify-between px-4 py-3 border-b border-slate-100 sticky top-0 bg-white z-10">
            <h3 class="font-medium">
              {{
                statusModalPeer
                  ? t('vpn.wg.peerStatusTitle', { peer: statusModalPeer })
                  : t('vpn.wg.instanceStatusTitle')
              }}
            </h3>
            <button type="button" class="text-slate-500 hover:text-slate-800 text-xl leading-none px-2" @click="closeStatusModal">
              ×
            </button>
          </div>
          <div class="p-4 space-y-4 text-sm">
            <p v-if="statusModalErr" class="text-red-600">{{ statusModalErr }}</p>
            <p v-else-if="statusModalLoading" class="text-slate-500">{{ t('common.loading') }}</p>
            <template v-else-if="!statusModalPeer && statusModalData">
              <dl class="grid sm:grid-cols-2 gap-3">
                <div>
                  <dt class="text-xs text-slate-500">{{ t('vpn.wg.iface') }}</dt>
                  <dd class="font-mono">{{ statusModalData.interface || '—' }}</dd>
                </div>
                <div>
                  <dt class="text-xs text-slate-500">{{ t('vpn.wg.statusLabel') }}</dt>
                  <dd>
                    <span
                      class="text-xs px-2 py-0.5 rounded"
                      :class="statusModalData.up ? 'bg-emerald-100 text-emerald-800' : 'bg-slate-100 text-slate-600'"
                    >
                      {{ statusModalData.up ? t('vpn.wg.running') : statusModalData.installed ? t('vpn.wg.installedNotUp') : t('vpn.wg.notInstalled') }}
                    </span>
                    <span class="text-xs text-slate-500 ml-2">
                      {{ statusModalData.enabled ? t('vpn.wg.configEnabled') : t('vpn.wg.configDisabled') }}
                    </span>
                  </dd>
                </div>
                <div class="sm:col-span-2">
                  <dt class="text-xs text-slate-500 mb-1">{{ t('vpn.wg.peerOnlineCount') }}</dt>
                  <dd>
                    {{ (statusModalData.peers || []).filter((p) => p.online).length }}
                    /
                    {{ (statusModalData.peers || []).length }}
                  </dd>
                </div>
              </dl>
              <div>
                <p class="text-xs text-slate-500 mb-1">{{ t('vpn.wg.wgShowRaw') }}</p>
                <pre class="text-xs font-mono bg-slate-50 border border-slate-200 rounded-lg p-3 overflow-x-auto whitespace-pre-wrap max-h-80">{{ statusModalData.raw || '—' }}</pre>
              </div>
            </template>
            <template v-else-if="statusModalPeer && statusModalData">
              <div class="flex items-center gap-2">
                <span
                  class="text-xs px-2 py-0.5 rounded"
                  :class="statusModalData.online ? 'bg-emerald-100 text-emerald-800' : 'bg-slate-100 text-slate-600'"
                >
                  {{ statusModalData.online ? t('ocserv.onlineNow') : t('ocserv.offlineNow') }}
                </span>
                <span v-if="statusModalData.in_kernel" class="text-xs text-slate-500">{{ t('vpn.wg.inKernel') }}</span>
                <span v-else class="text-xs text-amber-700">{{ t('vpn.wg.notInKernel') }}</span>
              </div>
              <dl class="grid sm:grid-cols-2 gap-3">
                <div>
                  <dt class="text-xs text-slate-500">{{ t('vpn.wg.peerEndpoint') }}</dt>
                  <dd class="font-mono text-xs break-all">{{ statusModalData.endpoint || '—' }}</dd>
                </div>
                <div>
                  <dt class="text-xs text-slate-500">{{ t('vpn.wg.lastHandshake') }}</dt>
                  <dd class="font-mono text-xs">
                    {{ statusModalData.last_handshake || formatHandshake(statusModalData.last_handshake_unix) }}
                  </dd>
                </div>
                <div>
                  <dt class="text-xs text-slate-500">RX</dt>
                  <dd class="font-mono">{{ formatBytes(statusModalData.rx_bytes) }}</dd>
                </div>
                <div>
                  <dt class="text-xs text-slate-500">TX</dt>
                  <dd class="font-mono">{{ formatBytes(statusModalData.tx_bytes) }}</dd>
                </div>
                <div class="sm:col-span-2">
                  <dt class="text-xs text-slate-500">{{ t('vpn.wg.peerAllowed') }}</dt>
                  <dd class="font-mono text-xs break-all">{{ (statusModalData.allowed_ips || []).join(', ') || '—' }}</dd>
                </div>
                <div class="sm:col-span-2">
                  <dt class="text-xs text-slate-500">{{ t('vpn.wg.peerPubkey') }}</dt>
                  <dd class="font-mono text-xs break-all">{{ statusModalData.public_key || '—' }}</dd>
                </div>
              </dl>
              <p class="text-xs text-slate-400">{{ t('vpn.wg.peerStatusHint') }}</p>
            </template>
          </div>
        </div>
      </div>
    </Teleport>

    <Teleport to="body">
      <div
        v-if="peerModalOpen"
        class="fixed inset-0 z-[60] flex items-center justify-center p-4 bg-black/40"
        role="presentation"
        @click.self="closePeerModal"
      >
        <div
          class="bg-white rounded-xl shadow-xl w-full max-w-2xl max-h-[90vh] overflow-y-auto border border-slate-200"
          role="dialog"
          aria-modal="true"
          aria-labelledby="wg-peer-modal-title"
          @click.stop
        >
          <div class="flex items-center justify-between px-4 py-3 border-b border-slate-100 sticky top-0 bg-white z-10">
            <h3 id="wg-peer-modal-title" class="font-medium">
              {{ peerEditing ? t('vpn.wg.editPeerModalTitle') : t('vpn.wg.addPeerModalTitle') }}
            </h3>
            <button
              type="button"
              class="text-slate-500 hover:text-slate-800 text-xl leading-none px-2"
              :aria-label="t('common.cancel')"
              @click="closePeerModal"
            >
              ×
            </button>
          </div>
          <div class="p-4 space-y-4">
            <p class="text-xs text-slate-500">
              {{ peerEditing ? t('vpn.wg.editPeerFormHint') : t('vpn.wg.peerFormHint') }}
            </p>
            <p v-if="peerModalErr" class="text-sm text-red-600">{{ peerModalErr }}</p>
            <div class="grid sm:grid-cols-2 gap-3 text-sm">
              <div>
                <label class="text-xs text-slate-500">{{ t('vpn.wg.peerName') }} *</label>
                <input
                  v-model="peerForm.name"
                  class="input-field"
                  placeholder="client-1"
                  :disabled="peerEditing"
                />
              </div>
              <div>
                <label class="text-xs text-slate-500">{{ t('vpn.wg.peerAllowedLabel') }}</label>
                <input v-model="peerForm.allowed_ips" class="input-field font-mono text-xs" placeholder="198.19.0.10/32" />
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
                  :placeholder="
                    peerEditing
                      ? peerHadPrivateKey
                        ? t('vpn.wg.privKeyKeepPh')
                        : t('vpn.wg.privKeyPh')
                      : t('vpn.wg.privKeyPh')
                  "
                  spellcheck="false"
                />
                <p v-if="peerEditing && peerHadPrivateKey" class="text-xs text-slate-500 mt-1">{{ t('vpn.wg.peerKeyKeepHint') }}</p>
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
            <div class="flex flex-wrap justify-end gap-2 pt-2 border-t border-slate-100">
              <button type="button" class="btn-secondary" @click="closePeerModal">{{ t('common.cancel') }}</button>
              <button type="button" class="btn-secondary" @click="genPeerKeys">{{ t('vpn.wg.autoKeypair') }}</button>
              <button type="button" class="btn-primary" @click="savePeer">
                {{ peerEditing ? t('vpn.wg.editPeerSubmit') : t('vpn.wg.addPeerSubmit') }}
              </button>
            </div>
          </div>
        </div>
      </div>
    </Teleport>

    <Teleport to="body">
      <div
        v-if="trafficModal"
        class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/40"
        @click.self="closePeerTraffic"
      >
        <div
          class="bg-white rounded-xl shadow-xl w-full max-w-2xl max-h-[90vh] overflow-y-auto border border-slate-200"
          role="dialog"
          aria-labelledby="wg-traffic-modal-title"
        >
          <div class="flex items-center justify-between px-4 py-3 border-b border-slate-100 bg-white shrink-0">
            <h3 id="wg-traffic-modal-title" class="font-medium">
              {{ t('vpn.wg.trafficTitle', { peer: trafficModal }) }}
            </h3>
            <button type="button" class="text-slate-500 hover:text-slate-800 text-xl leading-none px-2" @click="closePeerTraffic">×</button>
          </div>
          <div class="p-4 space-y-4">
            <div class="flex flex-wrap items-center gap-2 text-sm">
              <span class="text-slate-600">{{ t('vpn.wg.trafficRange') }}</span>
              <select v-model="trafficPeriod" class="input text-sm py-1 w-auto" :disabled="trafficLiveEnabled">
                <option value="24h">{{ t('ocserv.period24h') }}</option>
                <option value="7d">{{ t('ocserv.period7d') }}</option>
                <option value="30d">{{ t('ocserv.period30d') }}</option>
                <option value="365d">{{ t('ocserv.period365d') }}</option>
              </select>
              <span
                v-if="trafficData"
                class="text-xs px-2 py-0.5 rounded"
                :class="trafficData.online ? 'bg-emerald-100 text-emerald-800' : 'bg-slate-100 text-slate-600'"
              >
                {{ trafficData.online ? t('ocserv.onlineNow') : t('ocserv.offlineNow') }}
              </span>
              <span
                v-if="trafficData && trafficLastUpdated"
                class="text-xs text-slate-500 ml-auto"
              >
                {{ t('ocserv.trafficLastUpdated', { time: formatTrafficUpdatedAt(trafficLastUpdated) }) }}
              </span>
            </div>
            <p v-if="trafficErr" class="text-sm text-red-600">{{ trafficErr }}</p>
            <p v-else-if="trafficLoading" class="text-sm text-slate-500">{{ t('common.loading') }}</p>
            <template v-else-if="trafficData">
              <div class="grid grid-cols-2 sm:grid-cols-4 gap-2 text-sm">
                <div class="rounded-lg border p-2 bg-slate-50">
                  <p class="text-xs text-slate-500">{{ t('ocserv.todayRx') }}</p>
                  <p class="font-mono font-medium">{{ formatBytes(trafficData.summary?.today_rx_bytes) }}</p>
                </div>
                <div class="rounded-lg border p-2 bg-slate-50">
                  <p class="text-xs text-slate-500">{{ t('ocserv.todayTx') }}</p>
                  <p class="font-mono font-medium">{{ formatBytes(trafficData.summary?.today_tx_bytes) }}</p>
                </div>
                <div class="rounded-lg border p-2 bg-slate-50">
                  <p class="text-xs text-slate-500">{{ t('ocserv.periodRx') }}</p>
                  <p class="font-mono font-medium">{{ formatBytes(trafficData.summary?.period_rx_bytes) }}</p>
                </div>
                <div class="rounded-lg border p-2 bg-slate-50">
                  <p class="text-xs text-slate-500">{{ t('ocserv.periodTx') }}</p>
                  <p class="font-mono font-medium">{{ formatBytes(trafficData.summary?.period_tx_bytes) }}</p>
                </div>
                <div class="rounded-lg border p-2 bg-emerald-50/50 sm:col-span-2">
                  <p class="text-xs text-slate-500">{{ t('ocserv.totalRx') }}</p>
                  <p class="font-mono font-medium text-emerald-800">{{ formatBytes(trafficData.summary?.total_rx_bytes) }}</p>
                </div>
                <div class="rounded-lg border p-2 bg-sky-50/50 sm:col-span-2">
                  <p class="text-xs text-slate-500">{{ t('ocserv.totalTx') }}</p>
                  <p class="font-mono font-medium text-sky-800">{{ formatBytes(trafficData.summary?.total_tx_bytes) }}</p>
                </div>
              </div>
              <div v-if="trafficData.online && trafficData.current" class="text-sm rounded border border-emerald-100 bg-emerald-50/40 p-3">
                <p class="text-xs text-slate-600 mb-1">{{ t('vpn.wg.currentCounters') }}</p>
                <p>
                  RX {{ formatTraffic(trafficData.current._RX, trafficData.current.RX ?? trafficData.current.rx) }}
                  · TX {{ formatTraffic(trafficData.current._TX, trafficData.current.TX ?? trafficData.current.tx) }}
                </p>
              </div>
              <div class="space-y-2">
                <div class="flex flex-wrap items-center gap-3 text-sm rounded-lg border border-slate-200 bg-slate-50/80 px-3 py-2">
                  <label class="flex items-center gap-2 cursor-pointer select-none relative z-[1]">
                    <input
                      v-model="trafficLiveEnabled"
                      type="checkbox"
                      class="rounded shrink-0"
                      :disabled="!trafficLiveCanEnable"
                      @change="onTrafficLiveToggle"
                    />
                    <span class="font-medium text-slate-700">{{ t('ocserv.trafficLive') }}</span>
                  </label>
                  <span v-if="trafficLiveEnabled" class="text-xs text-emerald-700">
                    {{ t('ocserv.trafficLiveActive') }}
                  </span>
                  <span v-else-if="trafficData && !trafficLiveCanEnable" class="text-xs text-slate-500">
                    {{ t('vpn.wg.trafficLiveNeedCounters') }}
                  </span>
                </div>
                <p v-if="trafficLiveErr" class="text-xs text-amber-700">{{ trafficLiveErr }}</p>
                <p v-else-if="trafficLiveEnabled && trafficLiveSeries.length < 2" class="text-xs text-slate-500">
                  {{ t('ocserv.trafficLiveWarming') }}
                </p>
                <SnmpTrafficChart
                  :series="trafficChartSeries"
                  :empty-label="trafficLiveEnabled ? t('ocserv.trafficLiveWarming') : undefined"
                  :footer-label="trafficLiveEnabled ? t('ocserv.trafficLiveFooter') : undefined"
                />
              </div>
            </template>
            <p class="text-xs text-slate-500">
              {{ t('vpn.wg.trafficFoot') }} · {{ t('vpn.wg.trafficAutoRefresh') }}
            </p>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>
