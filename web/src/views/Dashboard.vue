<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/api/client'
import { useWidgetOrder } from '@/composables/useWidgetOrder'
import StatCard from '@/components/StatCard.vue'
import DashboardWidget from '@/components/DashboardWidget.vue'
import ProgressBar from '@/components/ProgressBar.vue'
import StatusBadge from '@/components/StatusBadge.vue'
import PageHeader from '@/components/PageHeader.vue'
import TrafficSparkline from '@/components/TrafficSparkline.vue'

const { t } = useI18n()
const data = ref(null)
const health = ref(null)
const dhcp = ref(null)
const wg = ref(null)
const err = ref('')
let timer

const phase = computed(() => health.value?.phase || data.value?.phase || '—')
const uptime = computed(() => {
  const s = data.value?.system?.uptime_sec
  if (!s) return '—'
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  return `${h}h ${m}m`
})

const cpuColor = computed(() => {
  const v = data.value?.system?.cpu_percent ?? 0
  if (v > 85) return 'red'
  if (v > 60) return 'amber'
  return 'green'
})

const memColor = computed(() => {
  const v = data.value?.system?.mem_percent ?? 0
  if (v > 90) return 'red'
  if (v > 75) return 'amber'
  return 'blue'
})

const MAIN_WIDGETS = ['system', 'network', 'services', 'qos']
const BOTTOM_WIDGETS = ['quick', 'top_hosts']
const { order: mainOrder, moveUp: mainUp, moveDown: mainDown } = useWidgetOrder(
  MAIN_WIDGETS,
  'qosnat2-dash-main',
)
const { order: bottomOrder, moveUp: bottomUp, moveDown: bottomDown } = useWidgetOrder(
  BOTTOM_WIDGETS,
  'qosnat2-dash-bottom',
)

const quickLinks = computed(() => [
  { path: '/network/interfaces', label: t('dashboard.linkInterfaces'), desc: t('dashboard.linkInterfacesDesc') },
  { path: '/nat/forwards', label: t('dashboard.linkForwards'), desc: t('dashboard.linkForwardsDesc') },
  { path: '/shaper/profiles', label: t('dashboard.linkProfiles'), desc: t('dashboard.linkProfilesDesc') },
  { path: '/network/dhcp', label: t('dashboard.linkDhcp'), desc: t('dashboard.linkDhcpDesc') },
  { path: '/vpn/wireguard', label: t('dashboard.linkWg'), desc: t('dashboard.linkWgDesc') },
  { path: '/diagnostics/capture', label: t('dashboard.linkCapture'), desc: t('dashboard.linkCaptureDesc') },
])

function mainIdx(id) {
  return mainOrder.value.indexOf(id)
}
function canUpMain(id) {
  return mainIdx(id) > 0
}
function canDownMain(id) {
  const i = mainIdx(id)
  return i >= 0 && i < mainOrder.value.length - 1
}
function canUpBottom(id) {
  return bottomOrder.value.indexOf(id) > 0
}
function canDownBottom(id) {
  const i = bottomOrder.value.indexOf(id)
  return i >= 0 && i < bottomOrder.value.length - 1
}

async function load() {
  try {
    const [dash, h, dhcpRes, wgRes] = await Promise.all([
      api.dashboard(),
      api.health(),
      api.get('/api/v1/dhcp').catch(() => null),
      api.get('/api/v1/vpn/wireguard').catch(() => null),
    ])
    data.value = dash
    health.value = h
    dhcp.value = dhcpRes
    wg.value = wgRes
    err.value = ''
  } catch (e) {
    err.value = e.message
  }
}

onMounted(() => {
  load()
  timer = setInterval(load, 5000)
})
onUnmounted(() => clearInterval(timer))
</script>

<template>
  <div class="page-stack">
    <PageHeader :title="t('nav.dashboard')" :description="t('dashboard.description')" />
    <p v-if="err" class="text-red-600 text-sm mb-4">{{ err }}</p>

    <div class="grid grid-cols-2 lg:grid-cols-4 gap-2">
      <StatCard :label="t('nav.activePerIp')" :value="data?.active_hosts ?? '—'" sub="eBPF active_host" />
      <StatCard :label="t('dashboard.conntrackEntries')" :value="data?.system?.conntrack ?? '—'" sub="conntrack" />
      <StatCard :label="t('dashboard.profileTemplates')" :value="data?.shaper?.profile_rules ?? '—'" sub="profile_lpm" />
      <StatCard :label="phase" :value="health?.bpf ? 'BPF ON' : 'BPF —'" :sub="uptime" />
    </div>

    <div class="grid lg:grid-cols-2 gap-3">
      <template v-for="wid in mainOrder" :key="wid">
      <DashboardWidget
        v-if="wid === 'system'"
        id="system"
        :title="t('dashboard.systemStatus')"
        reorderable
        :can-move-up="canUpMain('system')"
        :can-move-down="canDownMain('system')"
        @move-up="mainUp('system')"
        @move-down="mainDown('system')"
      >
        <ProgressBar
          label="CPU"
          :value="data?.system?.cpu_percent ?? 0"
          :color="cpuColor"
        />
        <ProgressBar
          :label="t('dashboard.memory')"
          :value="data?.system?.mem_percent ?? 0"
          :color="memColor"
        />
        <p class="text-xs text-slate-500 mt-2">
          {{ t('dashboard.uptime') }}
          <span class="font-mono">{{ uptime }}</span>
          · <span class="font-mono">{{ health?.service || 'qosnatd' }}</span>
        </p>
      </DashboardWidget>

      <DashboardWidget
        v-else-if="wid === 'network'"
        id="network"
        :title="t('dashboard.networkStatus')"
        reorderable
        :can-move-up="canUpMain('network')"
        :can-move-down="canDownMain('network')"
        @move-up="mainUp('network')"
        @move-down="mainDown('network')"
      >
        <div class="grid grid-cols-2 gap-3 mb-4">
          <TrafficSparkline
            :history="data?.traffic_history"
            field="lan_rx_mbps"
            :label="t('dashboard.lanRx')"
            color="bg-emerald-400"
          />
          <TrafficSparkline
            :history="data?.traffic_history"
            field="wan_tx_mbps"
            :label="t('dashboard.wanTx')"
            color="bg-sky-400"
          />
        </div>
        <div class="space-y-3">
          <div>
            <div class="flex justify-between text-sm mb-1">
              <span class="font-medium">LAN <span class="font-mono text-slate-500">{{ health?.dev_lan }}</span></span>
            </div>
            <p class="text-sm text-slate-600">
              ↓ <span class="font-mono">{{ data?.lan?.rx_mbps?.toFixed(2) ?? 0 }}</span> Mbps
              · ↑ <span class="font-mono">{{ data?.lan?.tx_mbps?.toFixed(2) ?? 0 }}</span> Mbps
            </p>
            <p class="text-xs text-slate-400 mt-1">
              {{ t('dashboard.rssQueues', { n: data?.interfaces?.lan?.channels ?? 0 }) }}
            </p>
          </div>
          <div>
            <div class="flex justify-between text-sm mb-1">
              <span class="font-medium">WAN <span class="font-mono text-slate-500">{{ health?.dev_wan }}</span></span>
            </div>
            <p class="text-sm text-slate-600">
              ↓ <span class="font-mono">{{ data?.wan?.rx_mbps?.toFixed(2) ?? 0 }}</span> Mbps
              · ↑ <span class="font-mono">{{ data?.wan?.tx_mbps?.toFixed(2) ?? 0 }}</span> Mbps
            </p>
            <p class="text-xs text-slate-400 mt-1">
              {{ t('dashboard.rssQueues', { n: data?.interfaces?.wan?.channels ?? 0 }) }}
            </p>
          </div>
        </div>
        <router-link to="/network/interfaces" class="text-xs text-blue-600 hover:underline mt-3 inline-block">
          {{ t('dashboard.viewInterfaces') }}
        </router-link>
      </DashboardWidget>

      <DashboardWidget
        v-else-if="wid === 'services'"
        id="services"
        :title="t('dashboard.serviceStatus')"
        reorderable
        :can-move-up="canUpMain('services')"
        :can-move-down="canDownMain('services')"
        @move-up="mainUp('services')"
        @move-down="mainDown('services')"
      >
        <StatusBadge label="qosnatd / eBPF" :ok="!!health?.bpf" :detail="health?.tc_attach ? t('dashboard.tcAttached') : ''" />
        <StatusBadge
          label="DHCP (dnsmasq)"
          :ok="!!dhcp?.status?.active"
          :detail="dhcp?.config?.enabled ? dhcp?.config?.interface : t('common.inactive')"
        />
        <StatusBadge
          label="WireGuard"
          :ok="!!wg?.status?.up"
          :detail="wg?.config?.enabled ? wg?.config?.interface : t('common.inactive')"
        />
        <StatusBadge
          :label="t('nav.markIsolation')"
          :ok="data?.mark_policy?.rules_ok"
          :detail="data?.mark_policy?.rules_ok ? t('dashboard.nftOk') : t('dashboard.nftCheck')"
        />
      </DashboardWidget>

      <DashboardWidget
        v-else-if="wid === 'qos'"
        id="qos"
        :title="t('dashboard.qosShaper')"
        reorderable
        :can-move-up="canUpMain('qos')"
        :can-move-down="canDownMain('qos')"
        @move-up="mainUp('qos')"
        @move-down="mainDown('qos')"
      >
        <dl class="grid grid-cols-2 gap-2 text-sm">
          <dt class="text-slate-500">{{ t('dashboard.policyCidrs') }}</dt>
          <dd class="font-mono text-right">{{ data?.shaper?.policy_cidr || '—' }}</dd>
          <dt class="text-slate-500">{{ t('dashboard.idleTimeout') }}</dt>
          <dd class="font-mono text-right">{{ data?.shaper?.idle_timeout_sec ?? '—' }}s</dd>
          <dt class="text-slate-500">{{ t('dashboard.ebpfMap') }}</dt>
          <dd class="text-right">{{ data?.ebpf?.loaded ? t('dashboard.ebpfLoaded') : '—' }}</dd>
        </dl>
        <div class="flex flex-wrap gap-2 mt-3">
          <router-link to="/shaper/profiles" class="text-xs text-blue-600 hover:underline">{{ t('dashboard.qosPolicy') }}</router-link>
          <router-link to="/status/active" class="text-xs text-blue-600 hover:underline">{{ t('dashboard.activePool') }}</router-link>
        </div>
      </DashboardWidget>
      </template>
    </div>

    <div class="grid lg:grid-cols-3 gap-4 mb-4">
      <template v-for="wid in bottomOrder" :key="'b-' + wid">
      <DashboardWidget
        v-if="wid === 'quick'"
        id="quick"
        :title="t('dashboard.quickLinks')"
        class="lg:col-span-1"
        reorderable
        :can-move-up="canUpBottom('quick')"
        :can-move-down="canDownBottom('quick')"
        @move-up="bottomUp('quick')"
        @move-down="bottomDown('quick')"
      >
        <div class="grid gap-2">
          <router-link
            v-for="link in quickLinks"
            :key="link.path"
            :to="link.path"
            class="block px-3 py-2 rounded border border-slate-200 hover:border-blue-300 hover:bg-blue-50 transition-colors"
          >
            <span class="text-sm font-medium text-slate-800">{{ link.label }}</span>
            <span class="text-xs text-slate-500 block">{{ link.desc }}</span>
          </router-link>
        </div>
      </DashboardWidget>

      <DashboardWidget
        v-else-if="wid === 'top_hosts'"
        id="top_hosts"
        :title="t('dashboard.topHosts')"
        class="lg:col-span-2"
        reorderable
        :can-move-up="canUpBottom('top_hosts')"
        :can-move-down="canDownBottom('top_hosts')"
        @move-up="bottomUp('top_hosts')"
        @move-down="bottomDown('top_hosts')"
      >
        <div class="table-wrap">
          <table class="data w-full text-sm">
            <thead>
              <tr>
                <th>IP</th>
                <th>{{ t('dashboard.colDownMbps') }}</th>
                <th>{{ t('dashboard.colUpMbps') }}</th>
                <th>{{ t('dashboard.colTotal') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="h in data?.top_hosts || []" :key="h.ip">
                <td class="font-mono">{{ h.ip }}</td>
                <td>{{ h.down_mbps?.toFixed(2) ?? '—' }}</td>
                <td>{{ h.up_mbps?.toFixed(2) ?? '—' }}</td>
                <td class="text-xs text-slate-500">
                  ↓{{ (h.bytes_down / 1024 / 1024).toFixed(1) }}M ↑{{ (h.bytes_up / 1024 / 1024).toFixed(1) }}M
                </td>
              </tr>
              <tr v-if="!(data?.top_hosts?.length)">
                <td colspan="4" class="text-slate-400 py-4 text-center">{{ t('dashboard.noActiveTraffic') }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </DashboardWidget>
      </template>
    </div>
  </div>
</template>
