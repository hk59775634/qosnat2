import { createRouter, createWebHashHistory } from 'vue-router'
import { api } from '@/api/client'

const routes = [
  { path: '/setup', name: 'setup', component: () => import('@/views/Setup.vue'), meta: { public: true } },
  { path: '/login', name: 'login', component: () => import('@/views/Login.vue'), meta: { public: true } },
  {
    path: '/',
    component: () => import('@/layouts/AppLayout.vue'),
    children: [
      { path: '', name: 'dashboard', component: () => import('@/views/Dashboard.vue') },
      { path: 'nat/outbound', name: 'nat-outbound', component: () => import('@/views/nat/Outbound.vue') },
      { path: 'nat/forwards', name: 'nat-forwards', component: () => import('@/views/nat/PortForwards.vue') },
      { path: 'firewall/rules', name: 'firewall-rules', component: () => import('@/views/security/FirewallRules.vue') },
      { path: 'firewall/aliases', name: 'firewall-aliases', component: () => import('@/views/security/Aliases.vue') },
      { path: 'firewall/geoip', redirect: { name: 'firewall-rules' } },
      { path: 'shaper/wizard', redirect: { name: 'shaper-profiles' } },
      { path: 'shaper/profiles', name: 'shaper-profiles', component: () => import('@/views/shaper/Profiles.vue') },
      { path: 'shaper/vip', redirect: { name: 'shaper-profiles' } },
      { path: 'status/active', name: 'status-active', component: () => import('@/views/status/ActiveHosts.vue') },
      { path: 'status/ebpf', name: 'status-ebpf', component: () => import('@/views/status/EbpfMaps.vue') },
      { path: 'status/mark', name: 'status-mark', component: () => import('@/views/status/MarkPolicy.vue') },
      { path: 'network/interfaces', name: 'network-interfaces', component: () => import('@/views/network/Interfaces.vue') },
      { path: 'interfaces/queues', name: 'iface-queues', component: () => import('@/views/interfaces/Queues.vue') },
      { path: 'network/routes', name: 'network-routes', component: () => import('@/views/network/Routes.vue') },
      { path: 'network/dhcp', name: 'network-dhcp', component: () => import('@/views/network/Dhcp.vue') },
      { path: 'network/vlans', name: 'network-vlans', component: () => import('@/views/network/Vlans.vue') },
      { path: 'network/wan-links', name: 'network-wan-links', component: () => import('@/views/network/WanLinks.vue') },
      { path: 'vpn/wireguard', name: 'vpn-wg', component: () => import('@/views/vpn/WireGuard.vue') },
      { path: 'diagnostics/capture', name: 'diag-capture', component: () => import('@/views/diagnostics/Capture.vue') },
      { path: 'diagnostics/conntrack', name: 'diag-conntrack', component: () => import('@/views/diagnostics/Conntrack.vue') },
      { path: 'system/general', name: 'system-general', component: () => import('@/views/system/General.vue') },
      { path: 'system/advanced', name: 'system-advanced', component: () => import('@/views/system/Advanced.vue') },
      { path: 'system/api-keys', name: 'system-api-keys', component: () => import('@/views/system/ApiKeys.vue') },
      { path: 'system/audit', name: 'system-audit', component: () => import('@/views/system/Audit.vue') },
      { path: 'docs/api', name: 'docs-api', component: () => import('@/views/docs/ApiDocs.vue') },
    ],
  },
]

const router = createRouter({
  history: createWebHashHistory(),
  routes,
})

router.beforeEach(async (to) => {
  let setupRequired = false
  try {
    const h = await api.health()
    setupRequired = h.setup_required === true
  } catch {
    if (!to.meta.public) return { name: 'setup' }
  }

  if (setupRequired) {
    if (to.name !== 'setup') return { name: 'setup' }
    return true
  }

  if (to.name === 'setup') return { name: 'login' }

  if (to.meta.public) return true
  try {
    await api.session()
    return true
  } catch {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
})

export default router
