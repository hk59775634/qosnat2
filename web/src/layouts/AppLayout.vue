<script setup>
import { ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { api } from '@/api/client'

const router = useRouter()
const route = useRoute()
const open = ref(true)

const menu = [
  { title: '仪表大厅', items: [{ path: '/', label: 'Dashboard' }] },
  {
    title: '防火墙 / NAT',
    items: [
      { path: '/nat/outbound', label: 'Outbound NAT' },
      { path: '/nat/forwards', label: '端口转发' },
    ],
  },
  {
    title: '流量整形',
    items: [
      { path: '/shaper/wizard', label: 'PCQ 向导' },
      { path: '/shaper/vip', label: 'VIP /32' },
      { path: '/shaper/profiles', label: '网段模板' },
    ],
  },
  {
    title: '接口',
    items: [{ path: '/interfaces/queues', label: 'RSS / 多队列' }],
  },
  {
    title: 'VPN',
    items: [{ path: '/vpn/wireguard', label: 'WireGuard' }],
  },
  {
    title: '状态',
    items: [
      { path: '/status/active', label: 'eBPF 活跃池' },
      { path: '/status/ebpf', label: 'Map 监视器' },
      { path: '/status/mark', label: 'Mark 隔离' },
      { path: '/diagnostics/conntrack', label: '连接状态' },
      { path: '/diagnostics/capture', label: '抓包' },
    ],
  },
  {
    title: '开发',
    items: [{ path: '/docs/api', label: 'API (Scalar)' }],
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
  <div class="min-h-screen flex flex-col">
    <header class="bg-pfsense-nav text-white shadow-md">
      <div class="flex items-center justify-between px-4 py-3">
        <div class="flex items-center gap-3">
          <button type="button" class="lg:hidden text-white/80" @click="open = !open">☰</button>
          <h1 class="text-lg font-semibold tracking-tight">qosnat2</h1>
          <span class="text-xs text-blue-200 hidden sm:inline">pfSense 风格管理</span>
        </div>
        <button type="button" class="text-sm text-blue-100 hover:text-white" @click="logout">登出</button>
      </div>
      <div class="bg-pfsense-bar px-4 py-1 text-xs text-blue-100 hidden sm:block">
        QoS + NAT · HTB / eBPF / nftables
      </div>
    </header>

    <div class="flex flex-1">
      <aside
        :class="[
          'bg-slate-800 text-slate-200 w-56 shrink-0 transition-all',
          open ? 'block' : 'hidden lg:block',
        ]"
      >
        <nav class="py-3 text-sm">
          <div v-for="group in menu" :key="group.title" class="mb-3">
            <div class="px-4 py-1 text-xs uppercase tracking-wider text-slate-400">{{ group.title }}</div>
            <router-link
              v-for="item in group.items"
              :key="item.path"
              :to="item.path"
              class="block px-4 py-2 hover:bg-slate-700"
              :class="isActive(item.path) ? 'bg-slate-700 text-white border-l-4 border-blue-400' : ''"
            >
              {{ item.label }}
            </router-link>
          </div>
        </nav>
      </aside>

      <main class="flex-1 p-4 lg:p-6 overflow-auto">
        <router-view />
      </main>
    </div>
  </div>
</template>
