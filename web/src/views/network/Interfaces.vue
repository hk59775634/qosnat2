<script setup>
import { onMounted, onUnmounted, ref } from 'vue'
import { api } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'
import DashboardWidget from '@/components/DashboardWidget.vue'
import ProgressBar from '@/components/ProgressBar.vue'
import TrafficSparkline from '@/components/TrafficSparkline.vue'

const devLan = ref('')
const devWan = ref('')
const ifaces = ref([])
const trafficHistory = ref([])
const err = ref('')
const ok = ref('')
const saving = ref(false)

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
const RATES_CAP_OPTIONS = [
  { value: 'auto', label: '自动（协商线速）' },
  { value: '10', label: '10 Mbps' },
  { value: '100', label: '100 Mbps' },
  { value: '1000', label: '1 Gbps (1000 Mbps)' },
  { value: '2500', label: '2.5 Gbps (2500 Mbps)' },
  { value: '10000', label: '10 Gbps (10000 Mbps)' },
  { value: '25000', label: '25 Gbps (25000 Mbps)' },
  { value: '40000', label: '40 Gbps (40000 Mbps)' },
  { value: '100000', label: '100 Gbps (100000 Mbps)' },
]

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

async function loadInterfaces() {
  try {
    const d = await api.interfaces.list()
    devLan.value = d.dev_lan || ''
    devWan.value = d.dev_wan || ''
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
    ok.value = 'Ring 缓冲已更新'
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
    ok.value = 'Offload 已更新'
    await loadEthtool(editDev.value)
  } catch (e) {
    err.value = e.message
  } finally {
    saving.value = false
  }
}

function roleLabel(iface) {
  if (iface.role) return iface.role
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
      title="接口"
      description="网卡近 4 小时流量趋势（LAN/WAN）；点击「实时速率」查看各口当前吞吐。IP 请在 netplan/系统侧配置。"
    />
    <p v-if="err" class="text-red-600 text-sm mb-4">{{ err }}</p>
    <p v-if="ok" class="text-green-700 text-sm mb-4">{{ ok }}</p>

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
          <dt class="text-slate-500 text-xs">地址</dt>
          <dd class="font-mono text-xs break-all mt-0.5">{{ addrLines(iface) }}</dd>
        </dl>

        <div v-if="historyFields(iface).ok" class="mt-3 grid grid-cols-2 gap-3" @click.stop>
          <TrafficSparkline
            tall
            :history="trafficHistory"
            :field="historyFields(iface).rx"
            label="近 4 小时 · 接收"
            color="bg-blue-400"
          />
          <TrafficSparkline
            tall
            :history="trafficHistory"
            :field="historyFields(iface).tx"
            label="近 4 小时 · 发送"
            color="bg-amber-400"
          />
        </div>
        <p v-else class="text-xs text-slate-400 mt-3">近 4 小时趋势仅统计 LAN/WAN 口</p>

        <button
          type="button"
          class="btn-secondary text-xs mt-3 w-full"
          @click="openRatesModal(iface, $event)"
        >
          实时速率
        </button>

        <p class="text-xs text-slate-400 mt-2">RSS 队列 {{ iface.rss_channels ?? 0 }}</p>
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
                <span v-if="ratesModal.role">{{ ratesModal.role }} · </span>
                {{ ratesModal.operstate }}
                <span v-if="ratesModal.link_speed_mbps > 0" class="ml-1">
                  · 协商 {{ ratesModal.link_speed_mbps }} Mbps
                </span>
                <span v-else class="ml-1 text-amber-700">· 协商未知</span>
              </p>
            </div>
            <button type="button" class="text-slate-400 hover:text-slate-600 text-xl leading-none" @click="closeRatesModal">
              ×
            </button>
          </div>

          <div class="mb-4">
            <label class="text-xs text-slate-500 block mb-1">基准线速（占比分母）</label>
            <select
              v-model="ratesCapChoice"
              class="input-field w-full text-sm"
              @change="onRatesCapChange"
            >
              <option
                v-for="opt in RATES_CAP_OPTIONS"
                :key="opt.value"
                :value="opt.value"
              >
                {{ opt.label }}
              </option>
            </select>
            <p class="text-xs text-slate-400 mt-1">
              当前基准 <span class="font-mono text-slate-600">{{ capMbps(ratesModal) }} Mbps</span>
              <span v-if="ratesCapChoice === 'auto'">（自动）</span>
              <span v-else>（手动）</span>
            </p>
          </div>

          <div class="space-y-2">
            <ProgressBar
              label="RX（占线速）"
              :traffic-mbps="ratesModal.traffic?.rx_mbps ?? 0"
              :cap-mbps="capMbps(ratesModal)"
              color="blue"
            />
            <ProgressBar
              label="TX（占线速）"
              :traffic-mbps="ratesModal.traffic?.tx_mbps ?? 0"
              :cap-mbps="capMbps(ratesModal)"
              color="amber"
            />
          </div>

          <p class="text-xs text-slate-400 mt-4 text-center">
            每 2 秒刷新；无流量时显示 0（首次打开约需 0.2s 建立基线）
          </p>
        </div>
      </div>
    </Teleport>

    <DashboardWidget v-if="editDev" id="iface-ethtool" title="ethtool 环缓冲" class="mb-3">
      <p class="text-sm text-slate-600 mb-3">
        设备 <span class="font-mono">{{ editDev }}</span>
        <span v-if="eth?.ring"> · 当前 RX {{ eth.ring.rx_current }} / TX {{ eth.ring.tx_current }}</span>
      </p>
      <div class="flex flex-wrap gap-3 items-end text-sm">
        <div>
          <label class="text-xs text-slate-500">RX ring</label>
          <input v-model.number="ringRx" type="number" class="input-field mt-1 w-28" />
        </div>
        <div>
          <label class="text-xs text-slate-500">TX ring</label>
          <input v-model.number="ringTx" type="number" class="input-field mt-1 w-28" />
        </div>
        <button type="button" class="btn-secondary" :disabled="saving" @click="saveRing">应用 ring</button>
      </div>
      <div class="mt-4 pt-3 border-t border-slate-200">
        <p class="text-xs text-slate-500 mb-2">Offload（ethtool -K，选 on / off）</p>
        <div class="flex flex-wrap gap-3 items-end text-sm">
          <div>
            <label class="text-xs text-slate-500">GRO</label>
            <select v-model="offGRO" class="input-field mt-1 w-24">
              <option value="">—</option>
              <option value="on">on</option>
              <option value="off">off</option>
            </select>
          </div>
          <div>
            <label class="text-xs text-slate-500">GSO</label>
            <select v-model="offGSO" class="input-field mt-1 w-24">
              <option value="">—</option>
              <option value="on">on</option>
              <option value="off">off</option>
            </select>
          </div>
          <div>
            <label class="text-xs text-slate-500">TX csum</label>
            <select v-model="offTx" class="input-field mt-1 w-24">
              <option value="">—</option>
              <option value="on">on</option>
              <option value="off">off</option>
            </select>
          </div>
          <div>
            <label class="text-xs text-slate-500">RX csum</label>
            <select v-model="offRx" class="input-field mt-1 w-24">
              <option value="">—</option>
              <option value="on">on</option>
              <option value="off">off</option>
            </select>
          </div>
          <button type="button" class="btn-secondary" :disabled="saving" @click="saveOffloads">应用 offload</button>
        </div>
      </div>
    </DashboardWidget>

    <div class="grid lg:grid-cols-2 gap-4">
      <DashboardWidget id="iface-links-lan" title="LAN 相关配置">
        <ul class="text-sm space-y-2">
          <li>
            <router-link to="/network/dhcp" class="text-blue-600 hover:underline">DHCP 服务</router-link>
            <span class="text-slate-500"> — 通常绑定 {{ devLan || '内网口' }}</span>
          </li>
          <li>
            <router-link to="/shaper/profiles" class="text-blue-600 hover:underline">QoS 策略</router-link>
          </li>
          <li>
            <router-link to="/interfaces/queues" class="text-blue-600 hover:underline">RSS / 多队列</router-link>
          </li>
        </ul>
      </DashboardWidget>

      <DashboardWidget id="iface-links-wan" title="WAN / NAT">
        <ul class="text-sm space-y-2">
          <li>
            <router-link to="/nat/outbound" class="text-blue-600 hover:underline">Outbound NAT</router-link>
          </li>
          <li>
            <router-link to="/nat/forwards" class="text-blue-600 hover:underline">端口转发</router-link>
          </li>
          <li>
            <router-link to="/network/routes" class="text-blue-600 hover:underline">静态路由</router-link>
          </li>
        </ul>
      </DashboardWidget>
    </div>

    <p class="text-xs text-slate-400 mt-2">流量历史每 5 秒采样，保留约 4 小时（LAN/WAN）</p>
  </div>
</template>
