<script setup>
import { computed, ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { api } from '@/api/client'

const router = useRouter()
const route = useRoute()
const open = ref(true)
const isApiDocs = computed(() => route.name === 'docs-api')

/** 对齐 UI开发建议 第十五节：现代 SDN/QoS 控制台菜单 */
const menu = [
  {
    title: 'Dashboard',
    items: [{ path: '/', label: '仪表盘' }],
  },
  {
    title: 'Network',
    items: [
      { path: '/network/interfaces', label: '接口' },
      { path: '/network/routes', label: '路由' },
      { path: '/network/dhcp', label: 'DHCP' },
      { path: '/network/vlans', label: 'VLAN' },
      { path: '/network/wan-links', label: '多 WAN' },
      { path: '/interfaces/queues', label: 'RSS / 多队列' },
    ],
  },
  {
    title: 'Security',
    items: [
      { path: '/nat/outbound', label: 'Outbound NAT' },
      { path: '/nat/forwards', label: '端口转发' },
      { path: '/firewall/rules', label: '防火墙规则' },
      { path: '/firewall/aliases', label: 'Aliases' },
      { path: '/firewall/geoip', label: 'GeoIP' },
    ],
  },
  {
    title: 'Traffic',
    items: [
      { path: '/shaper/profiles', label: 'QoS 策略' },
      { path: '/shaper/vip', label: 'VIP 主机' },
      { path: '/status/active', label: '活跃 Per-IP' },
    ],
  },
  {
    title: 'VPN',
    items: [{ path: '/vpn/wireguard', label: 'WireGuard' }],
  },
  {
    title: 'Observability',
    items: [
      { path: '/status/ebpf', label: 'eBPF Maps' },
      { path: '/status/mark', label: 'Mark 隔离' },
      { path: '/diagnostics/conntrack', label: '连接跟踪' },
      { path: '/diagnostics/capture', label: '抓包' },
    ],
  },
  {
    title: 'System',
    items: [
      { path: '/system/general', label: '常规设置' },
      { path: '/system/advanced', label: '高级设置' },
      { path: '/system/api-keys', label: 'API 密钥' },
      { path: '/system/audit', label: '审计日志' },
      { path: '/docs/api', label: 'API / OpenAPI' },
    ],
  },
]

async function logout() {
  try {
    await api.logout()
  } finally {
    router.push({ name: 'login' })
  }
}

function isActive(path) {
  if (path === '/') return route.path === '/'
  return route.path.startsWith(path)
}
</script>

<template>
  <div class="min-h-screen flex flex-col bg-slate-100">
    <header class="bg-pfsense-nav text-white shadow-md shrink-0">
      <div class="flex items-center justify-between px-4 py-3">
        <div class="flex items-center gap-3">
          <button type="button" class="lg:hidden text-white/80" aria-label="菜单" @click="open = !open">☰</button>
          <h1 class="text-lg font-semibold tracking-tight">qosnat2</h1>
          <span class="text-xs text-blue-200 hidden sm:inline">QoS · NAT · eBPF</span>
        </div>
        <button type="button" class="text-sm text-blue-100 hover:text-white" @click="logout">登出</button>
      </div>
      <div class="bg-pfsense-bar px-4 py-1.5 text-xs text-blue-100 hidden sm:flex gap-4">
        <span>HTB / IFB</span>
        <span>nftables</span>
        <span>profile_lpm</span>
        <span class="text-blue-200">API-first 控制面</span>
      </div>
    </header>

    <div class="flex flex-1 min-h-0">
      <aside
        v-if="!isApiDocs"
        :class="[
          'bg-slate-800 text-slate-200 w-56 shrink-0 overflow-y-auto transition-all',
          open ? 'block' : 'hidden lg:block',
        ]"
      >
        <nav class="py-3 text-sm">
          <div v-for="group in menu" :key="group.title" class="mb-4">
            <div class="px-4 py-1.5 text-[10px] uppercase tracking-widest text-slate-500 font-semibold">
              {{ group.title }}
            </div>
            <router-link
              v-for="item in group.items"
              :key="item.path"
              :to="item.path"
              class="block px-4 py-2 hover:bg-slate-700/80 transition-colors"
              :class="
                isActive(item.path)
                  ? 'bg-slate-700 text-white border-l-4 border-blue-400 pl-3'
                  : 'border-l-4 border-transparent'
              "
            >
              {{ item.label }}
            </router-link>
          </div>
        </nav>
      </aside>

      <main
        :class="
          isApiDocs
            ? 'flex-1 min-h-0 min-w-0 flex flex-col overflow-hidden p-2 lg:p-3'
            : 'flex-1 p-4 lg:p-6 overflow-auto'
        "
      >
        <router-view />
      </main>
    </div>
  </div>
</template>
