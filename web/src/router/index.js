import { createRouter, createWebHashHistory } from 'vue-router'
import { api } from '@/api/client'
import { setDisplayName, refreshBrandingFromHealth } from '@/composables/useBranding'
import { setAppVersionFromHealth } from '@/composables/useAppVersion'

refreshBrandingFromHealth(api).catch(() => {})

const routes = [
  { path: '/setup', name: 'setup', component: () => import('@/views/Setup.vue'), meta: { pageWidth: 'full' } },
  { path: '/login', name: 'login', component: () => import('@/views/Login.vue'), meta: { public: true } },
  {
    path: '/',
    component: () => import('@/layouts/AppLayout.vue'),
    children: [
      { path: '', name: 'dashboard', component: () => import('@/views/Dashboard.vue'), meta: { pageWidth: 'full' } },
      { path: 'nat/outbound', name: 'nat-outbound', component: () => import('@/views/nat/Outbound.vue'), meta: { pageWidth: 'full' } },
      { path: 'nat/ipv6', name: 'nat-ipv6', component: () => import('@/views/nat/IPv6.vue'), meta: { pageWidth: 'full' } },
      { path: 'nat/forwards', name: 'nat-forwards', component: () => import('@/views/nat/PortForwards.vue'), meta: { pageWidth: 'full' } },
      { path: 'firewall/rules', name: 'firewall-rules', component: () => import('@/views/security/FirewallRules.vue'), meta: { pageWidth: 'full' } },
      { path: 'firewall/aliases', name: 'firewall-aliases', component: () => import('@/views/security/Aliases.vue'), meta: { pageWidth: 'full' } },
      { path: 'shaper/wizard', redirect: { name: 'shaper-profiles' } },
      { path: 'shaper/profiles', name: 'shaper-profiles', component: () => import('@/views/shaper/Profiles.vue'), meta: { pageWidth: 'full' } },
      { path: 'shaper/tenants', name: 'shaper-tenants', component: () => import('@/views/shaper/Tenants.vue'), meta: { pageWidth: 'full' } },
      { path: 'shaper/vip', redirect: { name: 'shaper-profiles' } },
      { path: 'status/active', name: 'status-active', component: () => import('@/views/status/ActiveHosts.vue'), meta: { pageWidth: 'full' } },
      { path: 'status/ebpf', name: 'status-ebpf', component: () => import('@/views/status/EbpfMaps.vue'), meta: { pageWidth: 'full' } },
      { path: 'status/mark', name: 'status-mark', component: () => import('@/views/status/MarkPolicy.vue'), meta: { pageWidth: 'full' } },
      { path: 'network/interfaces', name: 'network-interfaces', component: () => import('@/views/network/Interfaces.vue'), meta: { pageWidth: 'full' } },
      { path: 'network/queues', name: 'iface-queues', component: () => import('@/views/interfaces/Queues.vue'), meta: { pageWidth: 'full' } },
      { path: 'interfaces/queues', redirect: { name: 'iface-queues' } },
      { path: 'network/routes', name: 'network-routes', component: () => import('@/views/network/Routes.vue'), meta: { pageWidth: 'full' } },
      { path: 'network/dhcp', name: 'network-dhcp', component: () => import('@/views/network/Dhcp.vue'), meta: { pageWidth: 'full' } },
      { path: 'network/vlans', name: 'network-vlans', component: () => import('@/views/network/Vlans.vue'), meta: { pageWidth: 'full' } },
      { path: 'network/vxlan', name: 'network-vxlan', component: () => import('@/views/network/Vxlan.vue'), meta: { pageWidth: 'full' } },
      { path: 'network/wan-links', name: 'network-wan-links', component: () => import('@/views/network/WanLinks.vue'), meta: { pageWidth: 'full' } },
      { path: 'vpn/wireguard', name: 'vpn-wg', component: () => import('@/views/vpn/WireGuard.vue'), meta: { pageWidth: 'full' } },
      { path: 'vpn/ocserv', name: 'vpn-ocserv', component: () => import('@/views/vpn/OCServ.vue'), meta: { pageWidth: 'full' } },
      {
        path: 'vpn/ocserv/vhost/:domain',
        name: 'vpn-ocserv-vhost',
        component: () => import('@/views/vpn/OCServVhostAdvanced.vue'),
        meta: { pageWidth: 'full' },
      },
      { path: 'diagnostics/capture', name: 'diag-capture', component: () => import('@/views/diagnostics/Capture.vue'), meta: { pageWidth: 'full' } },
      { path: 'diagnostics/conntrack', name: 'diag-conntrack', component: () => import('@/views/diagnostics/Conntrack.vue'), meta: { pageWidth: 'full' } },
      { path: 'diagnostics/terminal', redirect: { name: 'system-terminal' } },
      { path: 'system/general', name: 'system-general', component: () => import('@/views/system/General.vue') },
      { path: 'system/certificates', name: 'system-certificates', component: () => import('@/views/system/Certificates.vue'), meta: { pageWidth: 'full' } },
      { path: 'system/advanced', name: 'system-advanced', component: () => import('@/views/system/Advanced.vue'), meta: { pageWidth: 'full' } },
      { path: 'system/snmp', name: 'system-snmp', component: () => import('@/views/system/Snmp.vue'), meta: { pageWidth: 'full' } },
      {
        path: 'system/terminal',
        name: 'system-terminal',
        component: () => import('@/views/diagnostics/Terminal.vue'),
        meta: { pageWidth: 'full' },
      },
      { path: 'system/api-keys', name: 'system-api-keys', component: () => import('@/views/system/ApiKeys.vue'), meta: { pageWidth: 'full' } },
      { path: 'system/audit', name: 'system-audit', component: () => import('@/views/system/Audit.vue'), meta: { pageWidth: 'full' } },
      { path: 'docs/api', name: 'docs-api', component: () => import('@/views/docs/ApiDocs.vue') },
    ],
  },
]

const router = createRouter({
  history: createWebHashHistory(),
  routes,
})

async function hasSession() {
  try {
    await api.session()
    return true
  } catch {
    return false
  }
}

router.beforeEach(async (to) => {
  let setupRequired = false
  try {
    const h = await api.health()
    setDisplayName(h.display_name)
    setAppVersionFromHealth(h)
    setupRequired = h.setup_required === true
  } catch {
    if (to.name !== 'login') return { name: 'login' }
    return true
  }

  if (to.name === 'login') return true

  const authed = await hasSession()

  if (setupRequired) {
    if (!authed) {
      return { name: 'login', query: { redirect: to.name === 'setup' ? '/setup' : to.fullPath } }
    }
    if (to.name !== 'setup') return { name: 'setup' }
    return true
  }

  if (to.name === 'setup') return { name: 'dashboard' }

  if (!authed) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  return true
})

export default router
