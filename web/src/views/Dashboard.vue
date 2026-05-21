<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { api } from '@/api/client'
import { useWidgetOrder } from '@/composables/useWidgetOrder'
import StatCard from '@/components/StatCard.vue'
import DashboardWidget from '@/components/DashboardWidget.vue'
import ProgressBar from '@/components/ProgressBar.vue'
import StatusBadge from '@/components/StatusBadge.vue'
import PageHeader from '@/components/PageHeader.vue'
import TrafficSparkline from '@/components/TrafficSparkline.vue'

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

const quickLinks = [
  { path: '/network/interfaces', label: '接口', desc: 'LAN/WAN 状态' },
  { path: '/nat/forwards', label: '端口转发', desc: 'DNAT 规则' },
  { path: '/shaper/profiles', label: 'QoS 模板', desc: 'profile_lpm' },
  { path: '/network/dhcp', label: 'DHCP', desc: 'dnsmasq' },
  { path: '/vpn/wireguard', label: 'WireGuard', desc: 'VPN 隧道' },
  { path: '/diagnostics/capture', label: '抓包', desc: 'tcpdump' },
]

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
    <PageHeader
      title="Dashboard"
      description="系统、网络、服务与 QoS 概览。小组件可折叠；标题栏 ↑↓ 调整顺序（保存在浏览器）。"
    />
    <p v-if="err" class="text-red-600 text-sm mb-4">{{ err }}</p>

    <div class="grid grid-cols-2 lg:grid-cols-4 gap-2">
      <StatCard label="活跃 Per-IP" :value="data?.active_hosts ?? '—'" sub="eBPF active_host" />
      <StatCard label="Conntrack" :value="data?.system?.conntrack ?? '—'" sub="会话表项" />
      <StatCard label="QoS 规则" :value="data?.shaper?.profile_rules ?? '—'" sub="网段模板数" />
      <StatCard :label="`阶段 ${phase}`" :value="health?.bpf ? 'BPF ON' : 'BPF —'" :sub="uptime" />
    </div>

    <div class="grid lg:grid-cols-2 gap-3">
      <template v-for="wid in mainOrder" :key="wid">
      <DashboardWidget
        v-if="wid === 'system'"
        id="system"
        title="系统状态"
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
          label="内存"
          :value="data?.system?.mem_percent ?? 0"
          :color="memColor"
        />
        <p class="text-xs text-slate-500 mt-2">
          运行时间 <span class="font-mono">{{ uptime }}</span>
          · 控制面 <span class="font-mono">{{ health?.service || 'qosnatd' }}</span>
        </p>
      </DashboardWidget>

      <DashboardWidget
        v-else-if="wid === 'network'"
        id="network"
        title="网络状态"
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
            label="LAN 下行"
            color="bg-emerald-400"
          />
          <TrafficSparkline
            :history="data?.traffic_history"
            field="wan_tx_mbps"
            label="WAN 上行"
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
              RSS {{ data?.interfaces?.lan?.channels ?? 0 }} 队列
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
              RSS {{ data?.interfaces?.wan?.channels ?? 0 }} 队列
            </p>
          </div>
        </div>
        <router-link to="/network/interfaces" class="text-xs text-blue-600 hover:underline mt-3 inline-block">
          查看接口详情 →
        </router-link>
      </DashboardWidget>

      <DashboardWidget
        v-else-if="wid === 'services'"
        id="services"
        title="服务状态"
        reorderable
        :can-move-up="canUpMain('services')"
        :can-move-down="canDownMain('services')"
        @move-up="mainUp('services')"
        @move-down="mainDown('services')"
      >
        <StatusBadge label="qosnatd / eBPF" :ok="!!health?.bpf" :detail="health?.tc_attach ? 'TC 已附加' : ''" />
        <StatusBadge
          label="DHCP (dnsmasq)"
          :ok="!!dhcp?.status?.active"
          :detail="dhcp?.config?.enabled ? dhcp?.config?.interface : '未启用'"
        />
        <StatusBadge
          label="WireGuard"
          :ok="!!wg?.status?.up"
          :detail="wg?.config?.enabled ? wg?.config?.interface : '未启用'"
        />
        <StatusBadge
          label="Mark 隔离审计"
          :ok="data?.mark_policy?.rules_ok"
          :detail="data?.mark_policy?.rules_ok ? 'nft 规则正常' : '请检查规则'"
        />
      </DashboardWidget>

      <DashboardWidget
        v-else-if="wid === 'qos'"
        id="qos"
        title="流量整形 (QoS)"
        reorderable
        :can-move-up="canUpMain('qos')"
        :can-move-down="canDownMain('qos')"
        @move-up="mainUp('qos')"
        @move-down="mainDown('qos')"
      >
        <dl class="grid grid-cols-2 gap-2 text-sm">
          <dt class="text-slate-500">策略网段</dt>
          <dd class="font-mono text-right">{{ data?.shaper?.policy_cidr || '—' }}</dd>
          <dt class="text-slate-500">空闲超时</dt>
          <dd class="font-mono text-right">{{ data?.shaper?.idle_timeout_sec ?? '—' }}s</dd>
          <dt class="text-slate-500">eBPF Map</dt>
          <dd class="text-right">{{ data?.ebpf?.loaded ? '已加载' : '—' }}</dd>
        </dl>
        <div class="flex flex-wrap gap-2 mt-3">
          <router-link to="/shaper/profiles" class="text-xs text-blue-600 hover:underline">QoS 策略</router-link>
          <router-link to="/status/active" class="text-xs text-blue-600 hover:underline">活跃池</router-link>
        </div>
      </DashboardWidget>
      </template>
    </div>

    <div class="grid lg:grid-cols-3 gap-4 mb-4">
      <template v-for="wid in bottomOrder" :key="'b-' + wid">
      <DashboardWidget
        v-if="wid === 'quick'"
        id="quick"
        title="快捷入口"
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
        title="Top 活跃主机"
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
                <th>下行 Mbps</th>
                <th>上行 Mbps</th>
                <th>累计</th>
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
                <td colspan="4" class="text-slate-400 py-4 text-center">暂无活跃流量</td>
              </tr>
            </tbody>
          </table>
        </div>
      </DashboardWidget>
      </template>
    </div>
  </div>
</template>
