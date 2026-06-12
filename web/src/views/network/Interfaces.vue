<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { api } from '@/api/client'

const { t } = useI18n()
const route = useRoute()
import PageHeader from '@/components/PageHeader.vue'
import DashboardWidget from '@/components/DashboardWidget.vue'
import ProgressBar from '@/components/ProgressBar.vue'
import TrafficSparkline from '@/components/TrafficSparkline.vue'
import IfaceRssDiagnostics from '@/components/IfaceRssDiagnostics.vue'

const rssDiagExpanded = computed(() => route.query.diag === 'rss')

const devLan = ref('')
const devWan = ref('')
const roleLan = ref('')
const roleWan = ref('')
const ifaces = ref([])
const trafficHistory = ref([])
const err = ref('')
const ok = ref('')
const saving = ref(false)
const savingRoles = ref(false)

const editDev = ref('')
const eth = ref(null)
const ringRx = ref(0)
const ringTx = ref(0)
const offGRO = ref('')
const offGSO = ref('')
const offTx = ref('')
const offRx = ref('')

const ratesModal = ref(null)
/** 进度条基准：'auto' 或 Mbps 数字（字符串绑定 select） */
const ratesCapChoice = ref('auto')
let ratesPollTimer = null

const RATES_CAP_STORAGE = 'qosnat2.iface_rates_cap'
const ratesCapOptions = computed(() => [
  { value: 'auto', label: t('network.interfaces.autoNegotiate') },
  { value: '10', label: t('network.interfaces.cap10') },
  { value: '100', label: t('network.interfaces.cap100') },
  { value: '1000', label: t('network.interfaces.cap1g') },
  { value: '2500', label: t('network.interfaces.cap2_5g') },
  { value: '10000', label: t('network.interfaces.cap10g') },
  { value: '25000', label: t('network.interfaces.cap25g') },
  { value: '40000', label: t('network.interfaces.cap40g') },
  { value: '100000', label: t('network.interfaces.cap100g') },
])

function loadRatesCapPrefs() {
  try {
    return JSON.parse(localStorage.getItem(RATES_CAP_STORAGE) || '{}')
  } catch {
    return {}
  }
}

function saveRatesCapPref(dev, choice) {
  const prefs = loadRatesCapPrefs()
  if (!dev) return
  if (choice === 'auto') {
    delete prefs[dev]
  } else {
    prefs[dev] = choice
  }
  localStorage.setItem(RATES_CAP_STORAGE, JSON.stringify(prefs))
}

function initRatesCapChoice(iface) {
  const saved = loadRatesCapPrefs()[iface?.name]
  if (saved && saved !== 'auto') {
    ratesCapChoice.value = String(saved)
    return
  }
  ratesCapChoice.value = 'auto'
}

function onRatesCapChange() {
  if (ratesModal.value?.name) {
    saveRatesCapPref(ratesModal.value.name, ratesCapChoice.value)
  }
}

async function saveRoles() {
  if (!roleWan.value) {
    err.value = t('network.interfaces.wanRequired')
    return
  }
  if (roleLan.value && roleLan.value === roleWan.value) {
    err.value = t('network.interfaces.lanWanDiff')
    return
  }
  savingRoles.value = true
  err.value = ''
  ok.value = ''
  try {
    const res = await api.interfaces.setRoles({
      dev_lan: roleLan.value,
      dev_wan: roleWan.value,
      apply: true,
    })
    if (res.apply_error) {
      err.value = `${t('network.interfaces.rolesSavedApplyFail')}: ${res.apply_error}`
    } else {
      ok.value = t('network.interfaces.rolesApplied')
    }
    await loadInterfaces()
  } catch (e) {
    err.value = e.message
  } finally {
    savingRoles.value = false
  }
}

async function loadInterfaces() {
  try {
    const d = await api.interfaces.list()
    devLan.value = d.dev_lan || ''
    devWan.value = d.dev_wan || ''
    roleLan.value = d.dev_lan || ''
    roleWan.value = d.dev_wan || ''
    ifaces.value = d.interfaces || []
    trafficHistory.value = d.traffic_history || []
    err.value = ''
  } catch (e) {
    err.value = e.message
  }
}

async function refreshModalIface() {
  if (!ratesModal.value?.name) return
  try {
    const d = await api.interfaces.list()
    const found = (d.interfaces || []).find((i) => i.name === ratesModal.value.name)
    if (found) ratesModal.value = found
    trafficHistory.value = d.traffic_history || trafficHistory.value
  } catch {
    /* ignore poll errors in modal */
  }
}

function openRatesModal(iface, e) {
  e?.stopPropagation?.()
  ratesModal.value = { ...iface }
  initRatesCapChoice(iface)
  refreshModalIface()
  if (ratesPollTimer) clearInterval(ratesPollTimer)
  ratesPollTimer = setInterval(refreshModalIface, 2000)
}

function closeRatesModal() {
  ratesModal.value = null
  if (ratesPollTimer) {
    clearInterval(ratesPollTimer)
    ratesPollTimer = null
  }
}

async function loadEthtool(dev) {
  if (!dev) {
    eth.value = null
    return
  }
  try {
    eth.value = await api.interfacesEthtool(dev)
    ringRx.value = eth.value?.ring?.rx_current || eth.value?.ring?.rx_max || 0
    ringTx.value = eth.value?.ring?.tx_current || eth.value?.ring?.tx_max || 0
    const o = eth.value?.offloads || {}
    offGRO.value = o.gro || ''
    offGSO.value = o.gso || ''
    offTx.value = o.tx_checksum || ''
    offRx.value = o.rx_checksum || ''
  } catch {
    eth.value = null
  }
}

function selectEdit(iface) {
  editDev.value = iface.name
  loadEthtool(iface.name)
}

async function saveRing() {
  if (!editDev.value) return
  saving.value = true
  err.value = ''
  try {
    await api.setEthtool(editDev.value, { rx_ring: ringRx.value, tx_ring: ringTx.value })
    ok.value = t('network.interfaces.ringSaved')
    await loadEthtool(editDev.value)
  } catch (e) {
    err.value = e.message
  } finally {
    saving.value = false
  }
}

async function saveOffloads() {
  if (!editDev.value) return
  saving.value = true
  err.value = ''
  try {
    await api.setEthtool(editDev.value, {
      offloads: {
        gro: offGRO.value,
        gso: offGSO.value,
        tx_checksum: offTx.value,
        rx_checksum: offRx.value,
      },
    })
    ok.value = t('network.interfaces.offloadSaved')
    await loadEthtool(editDev.value)
  } catch (e) {
    err.value = e.message
  } finally {
    saving.value = false
  }
}

function roleLabel(iface) {
  if (iface.role === 'LAN') return t('security.firewall.roleLan')
  if (iface.role === 'WAN') return t('security.firewall.roleWan')
  return ''
}

function addrLines(iface) {
  return (iface.addrs || []).map((a) => `${a.cidr}${a.scope ? ` (${a.scope})` : ''}`).join(', ') || '—'
}

/** 协商线速；未知时 1000 Mbps */
function detectedCapMbps(iface) {
  const s = iface?.link_speed_mbps
  if (s > 0) return s
  return 1000
}

/** 进度条上限：手动选择优先，否则自动 */
function capMbps(iface) {
  if (ratesCapChoice.value !== 'auto') {
    const n = Number(ratesCapChoice.value)
    if (n > 0) return n
  }
  return detectedCapMbps(iface)
}

/** LAN/WAN 与 traffic_history 字段对应 */
function historyFields(iface) {
  if (iface.role === 'LAN') {
    return { rx: 'lan_rx_mbps', tx: 'lan_tx_mbps', ok: true }
  }
  if (iface.role === 'WAN') {
    return { rx: 'wan_rx_mbps', tx: 'wan_tx_mbps', ok: true }
  }
  return { ok: false }
}

onMounted(loadInterfaces)

onUnmounted(() => {
  closeRatesModal()
})
</script>

<template>
  <div class="page-stack">
    <PageHeader
      :title="t('network.interfaces.title')"
      :description="t('network.interfaces.description')" :ok="ok" :err="err" />

    <div class="card p-4 mb-4">
      <h3 class="text-sm font-semibold text-slate-800 mb-3">{{ t('network.interfaces.wanLan') }}</h3>
      <div class="grid sm:grid-cols-2 gap-3 text-sm max-w-2xl">
        <div>
          <label class="text-xs text-slate-500">{{ t('network.interfaces.wan') }} *</label>
          <select v-model="roleWan" class="input-field mt-1">
            <option value="">{{ t('network.interfaces.choose') }}</option>
            <option v-for="i in ifaces" :key="'rw-' + i.name" :value="i.name">
              {{ i.name }} {{ i.up ? '(UP)' : '' }}
            </option>
          </select>
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('network.interfaces.lan') }}</label>
          <select v-model="roleLan" class="input-field mt-1">
            <option value="">{{ t('network.interfaces.notMapped') }}</option>
            <option v-for="i in ifaces" :key="'rl-' + i.name" :value="i.name">
              {{ i.name }} {{ i.up ? '(UP)' : '' }}
            </option>
          </select>
        </div>
      </div>
      <p class="text-xs text-slate-500 mt-2">{{ t('network.interfaces.rolesApplyHint') }}</p>
      <button
        type="button"
        class="btn-primary mt-3"
        :disabled="savingRoles || !roleWan"
        @click="saveRoles"
      >
        {{ savingRoles ? t('common.processing') : t('common.save') }}
      </button>
    </div>

    <div class="grid md:grid-cols-2 gap-4 mb-3">
      <div
        v-for="iface in ifaces"
        :key="iface.name"
        class="card p-5 border-l-4 cursor-pointer hover:shadow-md transition-shadow"
        :class="{
          'border-l-blue-500': iface.role === 'LAN',
          'border-l-amber-500': iface.role === 'WAN',
          'border-l-slate-300': !iface.role,
          'ring-2 ring-blue-400': editDev === iface.name,
        }"
        @click="selectEdit(iface)"
      >
        <div class="flex items-start justify-between gap-2">
          <div>
            <h3 class="text-lg font-mono font-semibold">{{ iface.name }}</h3>
            <p class="text-xs text-slate-500 mt-1">
              <span v-if="roleLabel(iface)" class="font-medium text-slate-700">{{ roleLabel(iface) }} · </span>
              {{ iface.operstate }}
              <span v-if="iface.mac" class="ml-2 font-mono">{{ iface.mac }}</span>
            </p>
          </div>
          <span
            class="text-xs px-2 py-1 rounded shrink-0"
            :class="iface.up ? 'bg-green-100 text-green-800' : 'bg-slate-100 text-slate-600'"
          >
            {{ iface.up ? 'UP' : 'DOWN' }}
          </span>
        </div>

        <dl class="mt-3 text-sm">
          <dt class="text-slate-500 text-xs">{{ t('network.interfaces.address') }}</dt>
          <dd class="font-mono text-xs break-all mt-0.5">{{ addrLines(iface) }}</dd>
        </dl>

        <div v-if="historyFields(iface).ok" class="mt-3 grid grid-cols-2 gap-3" @click.stop>
          <TrafficSparkline
            tall
            :history="trafficHistory"
            :field="historyFields(iface).rx"
            :label="t('network.interfaces.traffic4hRx')"
            color="bg-blue-400"
          />
          <TrafficSparkline
            tall
            :history="trafficHistory"
            :field="historyFields(iface).tx"
            :label="t('network.interfaces.traffic4hTx')"
            color="bg-amber-400"
          />
        </div>
        <p v-else class="text-xs text-slate-400 mt-3">{{ t('network.interfaces.traffic4hRolesOnly') }}</p>

        <button
          type="button"
          class="btn-secondary text-xs mt-3 w-full"
          @click="openRatesModal(iface, $event)"
        >
          {{ t('network.interfaces.liveRate') }}
        </button>

        <p class="text-xs text-slate-400 mt-2">{{ t('network.interfaces.rssQueues') }} {{ iface.rss_channels ?? 0 }}</p>
      </div>
    </div>

    <!-- 实时速率悬浮窗 -->
    <Teleport to="body">
      <div
        v-if="ratesModal"
        class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/40"
        role="dialog"
        aria-modal="true"
        @click.self="closeRatesModal"
      >
        <div class="card w-full max-w-md p-5 shadow-xl" @click.stop>
          <div class="flex items-start justify-between gap-2 mb-4">
            <div>
              <h3 class="text-lg font-mono font-semibold">{{ ratesModal.name }}</h3>
              <p class="text-xs text-slate-500 mt-0.5">
                <span v-if="roleLabel(ratesModal)">{{ roleLabel(ratesModal) }} · </span>
                {{ ratesModal.operstate }}
                <span v-if="ratesModal.link_speed_mbps > 0" class="ml-1">
                  · {{ t('network.interfaces.negotiateMbps', { n: ratesModal.link_speed_mbps }) }}
                </span>
                <span v-else class="ml-1 text-amber-700">· {{ t('network.interfaces.negotiateUnknown') }}</span>
              </p>
            </div>
            <button type="button" class="text-slate-400 hover:text-slate-600 text-xl leading-none" @click="closeRatesModal">
              ×
            </button>
          </div>

          <div class="mb-4">
            <label class="text-xs text-slate-500 block mb-1">{{ t('network.interfaces.baseline') }}</label>
            <select
              v-model="ratesCapChoice"
              class="input-field w-full text-sm"
              @change="onRatesCapChange"
            >
              <option
                v-for="opt in ratesCapOptions"
                :key="opt.value"
                :value="opt.value"
              >
                {{ opt.label }}
              </option>
            </select>
            <p class="text-xs text-slate-400 mt-1">
              {{ t('network.interfaces.currentBaseline') }}
              <span class="font-mono text-slate-600">{{ capMbps(ratesModal) }} Mbps</span>
              <span v-if="ratesCapChoice === 'auto'">{{ t('network.interfaces.baselineAuto') }}</span>
              <span v-else>{{ t('network.interfaces.baselineManual') }}</span>
            </p>
          </div>

          <div class="space-y-2">
            <ProgressBar
              :label="t('network.interfaces.rxPctCap')"
              :traffic-mbps="ratesModal.traffic?.rx_mbps ?? 0"
              :cap-mbps="capMbps(ratesModal)"
              color="blue"
            />
            <ProgressBar
              :label="t('network.interfaces.txPctCap')"
              :traffic-mbps="ratesModal.traffic?.tx_mbps ?? 0"
              :cap-mbps="capMbps(ratesModal)"
              color="amber"
            />
          </div>

          <p class="text-xs text-slate-400 mt-4 text-center">{{ t('network.interfaces.refreshHint') }}</p>
        </div>
      </div>
    </Teleport>

    <DashboardWidget v-if="editDev" id="iface-ethtool" :title="t('network.interfaces.ethtoolRing')" class="mb-3">
      <p class="text-sm text-slate-600 mb-3">
        {{ t('network.interfaces.device') }} <span class="font-mono">{{ editDev }}</span>
        <span v-if="eth?.ring">
          ·
          {{
            t('network.interfaces.currentRing', {
              rx: eth.ring.rx_current,
              tx: eth.ring.tx_current,
            })
          }}
        </span>
      </p>
      <div class="flex flex-wrap gap-3 items-end text-sm">
        <div>
          <label class="text-xs text-slate-500">{{ t('network.interfaces.rxRing') }}</label>
          <input v-model.number="ringRx" type="number" class="input-field mt-1 w-28" />
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('network.interfaces.txRing') }}</label>
          <input v-model.number="ringTx" type="number" class="input-field mt-1 w-28" />
        </div>
        <button type="button" class="btn-secondary" :disabled="saving" @click="saveRing">
          {{ t('network.interfaces.applyRingBtn') }}
        </button>
      </div>
      <div class="mt-4 pt-3 border-t border-slate-200">
        <p class="text-xs text-slate-500 mb-2">{{ t('network.interfaces.offloadSection') }}</p>
        <div class="flex flex-wrap gap-3 items-end text-sm">
          <div>
            <label class="text-xs text-slate-500">{{ t('network.interfaces.gro') }}</label>
            <select v-model="offGRO" class="input-field mt-1 w-24">
              <option value="">—</option>
              <option value="on">on</option>
              <option value="off">off</option>
            </select>
          </div>
          <div>
            <label class="text-xs text-slate-500">{{ t('network.interfaces.gso') }}</label>
            <select v-model="offGSO" class="input-field mt-1 w-24">
              <option value="">—</option>
              <option value="on">on</option>
              <option value="off">off</option>
            </select>
          </div>
          <div>
            <label class="text-xs text-slate-500">{{ t('network.interfaces.txCsum') }}</label>
            <select v-model="offTx" class="input-field mt-1 w-24">
              <option value="">—</option>
              <option value="on">on</option>
              <option value="off">off</option>
            </select>
          </div>
          <div>
            <label class="text-xs text-slate-500">{{ t('network.interfaces.rxCsum') }}</label>
            <select v-model="offRx" class="input-field mt-1 w-24">
              <option value="">—</option>
              <option value="on">on</option>
              <option value="off">off</option>
            </select>
          </div>
          <button type="button" class="btn-secondary" :disabled="saving" @click="saveOffloads">
            {{ t('network.interfaces.applyOffloadBtn') }}
          </button>
        </div>
      </div>
    </DashboardWidget>

    <IfaceRssDiagnostics :default-expanded="rssDiagExpanded" />

    <div class="grid lg:grid-cols-2 gap-4">
      <DashboardWidget id="iface-links-lan" :title="t('network.interfaces.linksLanTitle')">
        <ul class="text-sm space-y-2">
          <li>
            <router-link to="/network/dhcp" class="text-blue-600 hover:underline">{{ t('nav.dhcp') }}</router-link>
            <span class="text-slate-500">
              —
              {{
                t('network.interfaces.dhcpBindHint', {
                  iface: devLan || t('network.interfaces.lanIfacePlaceholder'),
                })
              }}
            </span>
          </li>
          <li>
            <router-link to="/shaper/profiles" class="text-blue-600 hover:underline">{{ t('nav.qosProfiles') }}</router-link>
          </li>
        </ul>
      </DashboardWidget>

      <DashboardWidget id="iface-links-wan" :title="t('network.interfaces.linksWanTitle')">
        <ul class="text-sm space-y-2">
          <li>
            <router-link to="/nat/outbound" class="text-blue-600 hover:underline">{{ t('nav.outboundNat') }}</router-link>
          </li>
          <li>
            <router-link to="/nat/forwards" class="text-blue-600 hover:underline">{{ t('nav.portForwards') }}</router-link>
          </li>
          <li>
            <router-link to="/network/routes" class="text-blue-600 hover:underline">{{ t('nav.routes') }}</router-link>
          </li>
        </ul>
      </DashboardWidget>
    </div>

    <p class="text-xs text-slate-400 mt-2">{{ t('network.interfaces.historyHint') }}</p>
  </div>
</template>
