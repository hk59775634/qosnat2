<script setup>
import { computed, ref, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { api } from '@/api/client'
import LanguageSwitcher from '@/components/LanguageSwitcher.vue'
import GitHubProjectLink from '@/components/GitHubProjectLink.vue'
import NotificationTray from '@/components/NotificationTray.vue'
import { displayName } from '@/composables/useBranding'

const router = useRouter()
const route = useRoute()
const { t } = useI18n()

const NAV_COLLAPSED_KEY = 'qosnat2.nav.collapsed'
const NAV_GROUPS_KEY = 'qosnat2.nav.groups'

const mobileOpen = ref(false)
const sidebarCollapsed = ref(localStorage.getItem(NAV_COLLAPSED_KEY) === '1')

const isApiDocs = computed(() => route.name === 'docs-api')

const menu = computed(() => [
  {
    title: t('nav.dashboard'),
    items: [{ path: '/', label: t('nav.dashboardHome') }],
  },
  {
    title: t('nav.network'),
    items: [
      { path: '/network/interfaces', label: t('nav.interfaces') },
      { path: '/network/routes', label: t('nav.routes') },
      { path: '/network/dhcp', label: t('nav.dhcp') },
      { path: '/network/vlans', label: t('nav.vlans') },
      { path: '/network/vxlan', label: t('nav.vxlan') },
      { path: '/network/wan-links', label: t('nav.wanLinks') },
      { path: '/interfaces/queues', label: t('nav.rssQueues') },
    ],
  },
  {
    title: t('nav.security'),
    items: [
      { path: '/nat/outbound', label: t('nav.outboundNat') },
      { path: '/nat/ipv6', label: t('nav.ipv6Nat') },
      { path: '/nat/forwards', label: t('nav.portForwards') },
      { path: '/firewall/rules', label: t('nav.firewallRules') },
      { path: '/firewall/aliases', label: t('nav.aliases') },
    ],
  },
  {
    title: t('nav.traffic'),
    items: [
      { path: '/shaper/profiles', label: t('nav.qosProfiles') },
      { path: '/shaper/tenants', label: t('nav.qosTenants') },
      { path: '/status/active', label: t('nav.activePerIp') },
    ],
  },
  {
    title: t('nav.vpn'),
    items: [
      { path: '/vpn/wireguard', label: t('nav.wireguard') },
      { path: '/vpn/ocserv', label: t('nav.openconnect') },
    ],
  },
  {
    title: t('nav.observability'),
    items: [
      { path: '/status/ebpf', label: t('nav.ebpfMaps') },
      { path: '/status/mark', label: t('nav.markIsolation') },
      { path: '/diagnostics/conntrack', label: t('nav.conntrack') },
      { path: '/diagnostics/capture', label: t('nav.capture') },
    ],
  },
  {
    title: t('nav.system'),
    items: [
      { path: '/system/general', label: t('nav.general') },
      { path: '/system/certificates', label: t('nav.certificates') },
      { path: '/system/advanced', label: t('nav.advanced') },
      { path: '/system/api-keys', label: t('nav.apiKeys') },
      { path: '/system/audit', label: t('nav.audit') },
      { path: '/docs/api', label: t('nav.apiDocs') },
    ],
  },
])

function loadExpandedGroups() {
  try {
    const raw = localStorage.getItem(NAV_GROUPS_KEY)
    if (raw) {
      const parsed = JSON.parse(raw)
      if (Array.isArray(parsed)) return new Set(parsed)
    }
  } catch {
    /* ignore */
  }
  const active = groupForPath(route.path)
  return new Set(active ? [active] : [t('nav.dashboard')])
}

const expandedGroups = ref(loadExpandedGroups())

function persistExpandedGroups() {
  localStorage.setItem(NAV_GROUPS_KEY, JSON.stringify([...expandedGroups.value]))
}

function persistSidebarCollapsed() {
  localStorage.setItem(NAV_COLLAPSED_KEY, sidebarCollapsed.value ? '1' : '0')
}

function groupForPath(path) {
  for (const g of menu.value) {
    if (g.items.some((it) => isActive(it.path, path))) return g.title
  }
  return null
}

function isGroupExpanded(title) {
  return expandedGroups.value.has(title)
}

function toggleGroup(title) {
  const next = new Set(expandedGroups.value)
  if (next.has(title)) next.delete(title)
  else next.add(title)
  expandedGroups.value = next
  persistExpandedGroups()
}

function expandAllGroups() {
  expandedGroups.value = new Set(menu.value.map((g) => g.title))
  persistExpandedGroups()
}

function collapseAllGroups() {
  const active = groupForPath(route.path)
  expandedGroups.value = active ? new Set([active]) : new Set()
  persistExpandedGroups()
}

function toggleSidebarCollapsed() {
  sidebarCollapsed.value = !sidebarCollapsed.value
  persistSidebarCollapsed()
}

watch(
  () => route.path,
  (path) => {
    mobileOpen.value = false
    closeFlyout()
    const g = groupForPath(path)
    if (g && !expandedGroups.value.has(g)) {
      const next = new Set(expandedGroups.value)
      next.add(g)
      expandedGroups.value = next
      persistExpandedGroups()
    }
  },
  { immediate: true },
)

watch(sidebarCollapsed, persistSidebarCollapsed)

async function logout() {
  try {
    await api.logout()
  } finally {
    router.push({ name: 'login' })
  }
}

function isActive(path, current = route.path) {
  if (path === '/') return current === '/'
  return current.startsWith(path)
}

const asideWidthClass = computed(() =>
  sidebarCollapsed.value ? 'w-12' : 'w-64',
)

const flyoutGroup = ref(null)

function toggleFlyout(title) {
  flyoutGroup.value = flyoutGroup.value === title ? null : title
}

function closeFlyout() {
  flyoutGroup.value = null
}

const groupShort = computed(() => ({
  [t('nav.dashboard')]: 'D',
  [t('nav.network')]: 'N',
  [t('nav.security')]: 'S',
  [t('nav.traffic')]: 'T',
  [t('nav.vpn')]: 'V',
  [t('nav.observability')]: 'O',
  [t('nav.system')]: 'Y',
}))
</script>

<template>
  <div class="min-h-screen flex flex-col bg-slate-100">
    <header class="bg-pfsense-nav text-white shadow-md shrink-0">
      <div class="flex items-center justify-between px-4 py-3">
        <div class="flex items-center gap-3">
          <button
            v-if="!isApiDocs"
            type="button"
            class="text-white/80 hover:text-white p-1 -ml-1"
            :aria-label="sidebarCollapsed ? t('common.expandNav') : t('common.collapseNav')"
            :title="sidebarCollapsed ? t('common.expandNav') : t('common.collapseNav')"
            @click="toggleSidebarCollapsed"
          >
            <svg class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path
                v-if="sidebarCollapsed"
                stroke-linecap="round"
                stroke-linejoin="round"
                d="M4 6h16M4 12h16M4 18h16"
              />
              <path
                v-else
                stroke-linecap="round"
                stroke-linejoin="round"
                d="M11 19l-7-7 7-7M19 19l-7-7 7-7"
              />
            </svg>
          </button>
          <button
            v-if="!isApiDocs"
            type="button"
            class="lg:hidden text-white/80 hover:text-white p-1"
            :aria-label="t('common.menu')"
            @click="mobileOpen = !mobileOpen"
          >
            <svg class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" d="M4 6h16M4 12h16M4 18h16" />
            </svg>
          </button>
          <h1 class="text-lg font-semibold tracking-tight">{{ displayName }}</h1>
          <span class="text-xs text-blue-200 hidden sm:inline">QoS · NAT · eBPF</span>
        </div>
        <div class="flex items-center gap-2">
          <NotificationTray />
          <LanguageSwitcher />
          <button type="button" class="text-sm text-blue-100 hover:text-white" @click="logout">{{ t('common.logout') }}</button>
        </div>
      </div>
      <div class="bg-pfsense-bar px-4 py-1.5 text-xs text-blue-100 hidden sm:flex items-center gap-4">
        <span>{{ t('common.techHtb') }}</span>
        <span>{{ t('common.techNft') }}</span>
        <span>{{ t('common.techEbpf') }}</span>
        <span class="text-blue-200">{{ t('common.apiFirstControl') }}</span>
        <GitHubProjectLink variant="inline" class="ml-auto shrink-0" />
      </div>
    </header>

    <div class="flex flex-1 min-h-0">
      <aside
        v-if="!isApiDocs"
        :class="[
          'bg-slate-800 text-slate-200 shrink-0 overflow-y-auto overflow-x-hidden transition-[width] duration-200',
          asideWidthClass,
          mobileOpen ? 'fixed inset-y-0 left-0 z-40 top-[var(--header-h,0)] shadow-xl lg:static lg:shadow-none' : 'hidden lg:block',
        ]"
      >
        <nav
          class="app-sidebar-nav flex flex-col min-h-full"
          :class="sidebarCollapsed ? 'px-1' : 'px-0'"
        >
          <div
            v-if="!sidebarCollapsed"
            class="flex items-center justify-between gap-1 px-3 py-2 mb-1 border-b border-slate-700/80"
          >
            <span class="text-xs font-medium uppercase tracking-wide text-slate-500">{{ t('common.navigation') }}</span>
            <div class="flex gap-0.5">
              <button
                type="button"
                class="app-sidebar-toolbar"
                :title="t('nav.expandGroupTitle')"
                @click="expandAllGroups"
              >
                {{ t('common.expandAll') }}
              </button>
              <button
                type="button"
                class="app-sidebar-toolbar"
                :title="t('nav.collapseGroupTitle')"
                @click="collapseAllGroups"
              >
                {{ t('common.collapseAll') }}
              </button>
            </div>
          </div>

          <div v-for="group in menu" :key="group.title" class="mb-0.5">
            <template v-if="sidebarCollapsed">
              <div class="relative mx-0.5">
                <button
                  type="button"
                  class="w-full flex items-center justify-center py-2.5 rounded text-sm font-semibold transition-colors"
                  :class="
                    group.items.some((it) => isActive(it.path))
                      ? 'bg-slate-700 text-white ring-1 ring-blue-400/60'
                      : 'text-slate-400 hover:bg-slate-700/80 hover:text-slate-200'
                  "
                  :title="group.title"
                  @click="toggleFlyout(group.title)"
                >
                  {{ groupShort[group.title] || group.title[0] }}
                </button>
                <div v-if="flyoutGroup === group.title" class="app-sidebar-flyout">
                  <div class="app-sidebar-flyout-title">
                    {{ group.title }}
                  </div>
                  <router-link
                    v-for="item in group.items"
                    :key="item.path"
                    :to="item.path"
                    class="app-sidebar-flyout-item"
                    :class="isActive(item.path) ? 'text-white bg-slate-700/60' : 'text-slate-300'"
                    @click="closeFlyout"
                  >
                    {{ item.label }}
                  </router-link>
                </div>
              </div>
            </template>

            <template v-else>
              <button
                type="button"
                class="w-full flex items-center gap-2 px-3 py-2.5 text-left hover:bg-slate-700/50 transition-colors"
                @click="toggleGroup(group.title)"
              >
                <svg
                  class="w-3.5 h-3.5 shrink-0 text-slate-500 transition-transform"
                  :class="isGroupExpanded(group.title) ? 'rotate-90' : ''"
                  viewBox="0 0 24 24"
                  fill="currentColor"
                >
                  <path d="M8 5v14l11-7z" />
                </svg>
                <span class="app-sidebar-group-title">
                  {{ group.title }}
                </span>
                <span class="app-sidebar-group-count">{{ group.items.length }}</span>
              </button>
              <div v-show="isGroupExpanded(group.title)" class="pb-1">
                <router-link
                  v-for="item in group.items"
                  :key="item.path"
                  :to="item.path"
                  class="app-sidebar-item"
                  :class="
                    isActive(item.path)
                      ? 'bg-slate-700 text-white border-l-4 border-blue-400 !pl-8'
                      : 'border-l-4 border-transparent text-slate-300'
                  "
                >
                  {{ item.label }}
                </router-link>
              </div>
            </template>
          </div>

          <div
            class="mt-auto border-t border-slate-700/80 pt-2 pb-3"
            :class="sidebarCollapsed ? 'px-1' : 'px-2'"
          >
            <GitHubProjectLink
              v-if="!sidebarCollapsed"
              variant="sidebar"
            />
            <GitHubProjectLink
              v-else
              variant="sidebar"
              icon-only
            />
          </div>
        </nav>
      </aside>

      <div
        v-if="!isApiDocs && mobileOpen"
        class="fixed inset-0 z-30 bg-slate-900/40 lg:hidden"
        aria-hidden="true"
        @click="mobileOpen = false"
      />

      <main
        :class="
          isApiDocs
            ? 'flex-1 min-h-0 min-w-0 flex flex-col overflow-hidden p-2 lg:p-3'
            : 'flex-1 p-3 lg:p-4 overflow-auto min-w-0'
        "
        @click="closeFlyout"
      >
        <router-view />
      </main>
    </div>
  </div>
</template>
