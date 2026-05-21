<script setup>
import { onMounted, onUnmounted, ref } from 'vue'
import { api } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'
import DashboardWidget from '@/components/DashboardWidget.vue'
import ProgressBar from '@/components/ProgressBar.vue'

const devLan = ref('')
const devWan = ref('')
const ifaces = ref([])
const err = ref('')
const ok = ref('')
const saving = ref(false)
const pollTimer = ref(null)

const editDev = ref('')
const editIPv4 = ref('')
const editUp = ref(true)
const eth = ref(null)
const ringRx = ref(0)
const ringTx = ref(0)
const offGRO = ref('')
const offGSO = ref('')
const offTx = ref('')
const offRx = ref('')

async function loadRates() {
  try {
    const d = await api.interfaces.list()
    devLan.value = d.dev_lan || ''
    devWan.value = d.dev_wan || ''
    ifaces.value = d.interfaces || []
    err.value = ''
  } catch (e) {
    err.value = e.message
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
  editUp.value = iface.up
  const v4 = (iface.addrs || []).filter((a) => a.family === 'inet').map((a) => a.cidr)
  editIPv4.value = v4.join('\n')
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

async function saveIP() {
  if (!editDev.value) return
  saving.value = true
  ok.value = ''
  err.value = ''
  try {
    const ipv4 = editIPv4.value
      .split(/[\n,]+/)
      .map((s) => s.trim())
      .filter(Boolean)
    await api.interfaces.update({
      device: editDev.value,
      ipv4,
      up: editUp.value,
    })
    ok.value = `已更新 ${editDev.value} 的 IP 配置`
    await loadRates()
    const cur = ifaces.value.find((i) => i.name === editDev.value)
    if (cur) selectEdit(cur)
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

function maxMbps(iface) {
  const t = iface.traffic || {}
  return Math.max(t.rx_mbps || 0, t.tx_mbps || 0, 0.1)
}

onMounted(() => {
  loadRates()
  pollTimer.value = setInterval(loadRates, 2000)
})

onUnmounted(() => {
  if (pollTimer.value) clearInterval(pollTimer.value)
})
</script>

<template>
  <div>
    <PageHeader
      title="接口"
      description="查看网卡实时速率；IPv4 通过 netplan（/etc/netplan/99-qosnat2.yaml）写入并 netplan apply。LAN/WAN 由 DEV_LAN / DEV_WAN 标识。"
    />
    <p v-if="err" class="text-red-600 text-sm mb-4">{{ err }}</p>
    <p v-if="ok" class="text-green-700 text-sm mb-4">{{ ok }}</p>

    <div class="grid md:grid-cols-2 gap-4 mb-6">
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

        <dl class="grid grid-cols-2 gap-2 mt-3 text-sm">
          <div class="col-span-2">
            <dt class="text-slate-500 text-xs">地址</dt>
            <dd class="font-mono text-xs break-all">{{ addrLines(iface) }}</dd>
          </div>
          <div>
            <dt class="text-slate-500 text-xs">接收</dt>
            <dd class="font-mono">{{ (iface.traffic?.rx_mbps ?? 0).toFixed(2) }} Mbps</dd>
          </div>
          <div>
            <dt class="text-slate-500 text-xs">发送</dt>
            <dd class="font-mono">{{ (iface.traffic?.tx_mbps ?? 0).toFixed(2) }} Mbps</dd>
          </div>
        </dl>

        <div class="mt-3 space-y-1">
          <ProgressBar
            label="RX"
            :value="iface.traffic?.rx_mbps ?? 0"
            :max="maxMbps(iface)"
            unit=" Mbps"
            color="blue"
          />
          <ProgressBar
            label="TX"
            :value="iface.traffic?.tx_mbps ?? 0"
            :max="maxMbps(iface)"
            unit=" Mbps"
            color="amber"
          />
        </div>

        <p class="text-xs text-slate-400 mt-2">RSS 队列 {{ iface.rss_channels ?? 0 }}</p>
      </div>
    </div>

    <DashboardWidget v-if="editDev" id="iface-ethtool" title="ethtool 环缓冲" class="mb-6">
      <p class="text-sm text-slate-600 mb-3">
        设备 <span class="font-mono">{{ editDev }}</span>
        <span v-if="eth?.ring"> · 当前 RX {{ eth.ring.rx_current }} / TX {{ eth.ring.tx_current }}</span>
      </p>
      <div class="flex flex-wrap gap-3 items-end text-sm max-w-lg">
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

    <DashboardWidget id="iface-edit" title="修改 IP 地址" class="mb-6">
      <p class="text-sm text-slate-600 mb-4">
        点击上方网卡选择接口。保存后写入 <code class="text-xs bg-slate-100 px-1 rounded">99-qosnat2.yaml</code>
        并执行 <code class="text-xs">netplan apply</code>（每行一个 CIDR，如 <code class="text-xs">192.168.1.10/24</code>）。
      </p>
      <form class="max-w-lg space-y-4" @submit.prevent="saveIP">
        <div>
          <label class="text-sm">网卡</label>
          <input v-model="editDev" class="input-field mt-1 font-mono" readonly placeholder="点击卡片选择" />
        </div>
        <div>
          <label class="text-sm">IPv4 地址（每行一个 CIDR）</label>
          <textarea
            v-model="editIPv4"
            class="input-field mt-1 font-mono h-24"
            placeholder="192.168.1.10/24"
          />
        </div>
        <label class="flex items-center gap-2 text-sm">
          <input v-model="editUp" type="checkbox" />
          接口 UP（netplan 应用后由 networkd 拉起；取消勾选则 apply 后 <code class="text-xs">ip link set down</code>）
        </label>
        <button type="submit" class="btn-primary" :disabled="!editDev || saving">
          {{ saving ? '保存中…' : '保存 IP 配置' }}
        </button>
      </form>
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

    <p class="text-xs text-slate-400 mt-4">速率每 2 秒自动刷新</p>
  </div>
</template>
