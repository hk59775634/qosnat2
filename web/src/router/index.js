import { createRouter, createWebHashHistory } from 'vue-router'
import { api } from '@/api/client'

const routes = [
  { path: '/login', name: 'login', component: () => import('@/views/Login.vue'), meta: { public: true } },
  {
    path: '/',
    component: () => import('@/layouts/AppLayout.vue'),
    children: [
      { path: '', name: 'dashboard', component: () => import('@/views/Dashboard.vue') },
      { path: 'nat/outbound', name: 'nat-outbound', component: () => import('@/views/nat/Outbound.vue') },
      { path: 'nat/forwards', name: 'nat-forwards', component: () => import('@/views/nat/PortForwards.vue') },
      { path: 'shaper/wizard', name: 'shaper-wizard', component: () => import('@/views/shaper/Wizard.vue') },
      { path: 'shaper/vip', name: 'shaper-vip', component: () => import('@/views/shaper/VipHosts.vue') },
      { path: 'shaper/profiles', name: 'shaper-profiles', component: () => import('@/views/shaper/Profiles.vue') },
      { path: 'status/active', name: 'status-active', component: () => import('@/views/status/ActiveHosts.vue') },
      { path: 'status/ebpf', name: 'status-ebpf', component: () => import('@/views/status/EbpfMaps.vue') },
      { path: 'status/mark', name: 'status-mark', component: () => import('@/views/status/MarkPolicy.vue') },
      { path: 'interfaces/queues', name: 'iface-queues', component: () => import('@/views/interfaces/Queues.vue') },
      { path: 'vpn/wireguard', name: 'vpn-wg', component: () => import('@/views/vpn/WireGuard.vue') },
      { path: 'diagnostics/capture', name: 'diag-capture', component: () => import('@/views/diagnostics/Capture.vue') },
      { path: 'diagnostics/conntrack', name: 'diag-conntrack', component: () => import('@/views/diagnostics/Conntrack.vue') },
      { path: 'docs/api', name: 'docs-api', component: () => import('@/views/docs/ApiDocs.vue') },
    ],
  },
]

const router = createRouter({
  history: createWebHashHistory(),
  routes,
})

router.beforeEach(async (to) => {
  if (to.meta.public) return true
  try {
    await api.session()
    return true
  } catch {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
})

export default router
