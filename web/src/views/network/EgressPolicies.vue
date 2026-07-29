<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { api } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'
import {
  WAN_TAB_ALL,
  buildEgressBody,
  emptyEgressForm,
  egressEndpointsLabel as endpointsLabel,
  isAutoManagedEgress,
  optimalNoSnatForWan,
  pickOptimalWanLinkId,
  policyToEditForm,
  wanTabLabel,
} from '@/lib/egressPolicyForm'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
let syncingFromRoute = false

const links = ref([])
const egress = ref([])
const aliases = ref([])
const resolved = ref([])
const cloudflareCIDRs = ref([])
const googleIpv4Url = ref('')
const ifaces = ref([])
const devWan = ref('')
const err = ref('')
const ok = ref('')
const showForm = ref(true)
const activeWan = ref(WAN_TAB_ALL)
const editingId = ref(null)

const egForm = ref(emptyEgressForm())
const egEditForm = ref(emptyEgressForm())

const linkOptions = computed(() =>
  (links.value || []).filter((w) => w.enabled).map((w) => ({
    id: w.id,
    label: `${w.name || w.id}${w.device ? ` (${w.device})` : ''}`,
  })),
)

const ifaceOptions = computed(() => (ifaces.value || []).map((i) => i.name).filter(Boolean))

const wanTabs = computed(() => links.value || [])

const filteredEgress = computed(() => {
  const list = egress.value || []
  if (!activeWan.value || activeWan.value === WAN_TAB_ALL) return list
  return list.filter((p) => p.wan_link_id === activeWan.value)
})

const activeWanLink = computed(() => {
  if (!activeWan.value || activeWan.value === WAN_TAB_ALL) return null
  return (links.value || []).find((w) => w.id === activeWan.value) || null
})

const activeWanLabel = computed(() => {
  if (!activeWan.value || activeWan.value === WAN_TAB_ALL) return t('network.egressPolicies.tabAll')
  return wanTabLabel(activeWanLink.value) || activeWan.value
})

function countForWan(wanId) {
  const list = egress.value || []
  if (!wanId || wanId === WAN_TAB_ALL) return list.length
  return list.filter((p) => p.wan_link_id === wanId).length
}

function resolvedRow(policyId) {
  return resolved.value.find((r) => r.policy?.id === policyId)
}

function linkName(wanLinkId) {
  return (links.value || []).find((w) => w.id === wanLinkId)?.name || wanLinkId
}

function endpointsOf(p) {
  return endpointsLabel(p, {
    iif: t('network.egressPolicies.iifShort'),
    src: t('network.egressPolicies.srcShort'),
    dst: t('network.egressPolicies.dstShort'),
  })
}

function applyOptimalDefaults(preferWanId) {
  const wanId =
    preferWanId ||
    (activeWan.value && activeWan.value !== WAN_TAB_ALL
      ? activeWan.value
      : pickOptimalWanLinkId(links.value, devWan.value))
  const link = (links.value || []).find((w) => w.id === wanId)
  egForm.value = emptyEgressForm({
    name: link ? `${link.name || link.id} egress` : t('network.egressPolicies.defaultName'),
    wan_link_id: wanId || '',
    no_snat: optimalNoSnatForWan(link),
    dst_mode: 'none',
    src_mode: 'none',
    src_cidr: '',
    priority: 100,
    enabled: true,
    snat_ip: '',
  })
}

function setActiveWan(wanId) {
  activeWan.value = wanId || WAN_TAB_ALL
  editingId.value = null
  applyOptimalDefaults(wanId && wanId !== WAN_TAB_ALL ? wanId : '')
}

function syncRouteQuery() {
  if (syncingFromRoute) return
  const q = { ...route.query }
  if (activeWan.value && activeWan.value !== WAN_TAB_ALL) q.wan = activeWan.value
  else delete q.wan
  const same =
    String(route.query.wan || '') === String(q.wan || '')
  if (!same) {
    router.replace({ query: q }).catch(() => {})
  }
}

function applyRouteQuery() {
  syncingFromRoute = true
  try {
    const wan = String(route.query.wan || '')
    if (wan && (links.value || []).some((w) => w.id === wan)) {
      activeWan.value = wan
    } else if (!wan) {
      activeWan.value = WAN_TAB_ALL
    }
  } finally {
    syncingFromRoute = false
  }
}

watch(activeWan, () => {
  syncRouteQuery()
})

watch(
  () => route.query.wan,
  () => {
    applyRouteQuery()
    applyOptimalDefaults(
      activeWan.value && activeWan.value !== WAN_TAB_ALL ? activeWan.value : '',
    )
  },
)

async function load() {
  err.value = ''
  try {
    const [wan, eg, ifs] = await Promise.all([
      api.network.wanLinks.list(),
      api.network.egressPolicies.list(),
      api.interfaces.list(),
    ])
    links.value = wan?.wan_links || []
    devWan.value = wan?.dev_wan || ''
    egress.value = eg?.egress_policies || []
    aliases.value = eg?.aliases || []
    googleIpv4Url.value = eg?.google_ipv4_url || 'https://www.gstatic.com/ipranges/goog_ipv4_only.txt'
    resolved.value = eg?.resolved || []
    cloudflareCIDRs.value = eg?.cloudflare_cdn_cidrs_ipv4 || []
    ifaces.value = ifs?.interfaces || []
    applyRouteQuery()
    if (!egForm.value.wan_link_id) {
      applyOptimalDefaults(
        activeWan.value && activeWan.value !== WAN_TAB_ALL ? activeWan.value : '',
      )
    } else if (activeWan.value && activeWan.value !== WAN_TAB_ALL) {
      // Keep form WAN aligned with tab when switching after load.
      if (egForm.value.wan_link_id !== activeWan.value && !editingId.value) {
        applyOptimalDefaults(activeWan.value)
      }
    }
  } catch (e) {
    err.value = e?.message || String(e)
  }
}

async function addEgress() {
  err.value = ''
  ok.value = ''
  try {
    const body = buildEgressBody(egForm.value)
    await api.network.egressPolicies.add(body)
    ok.value = t('common.saved')
    await load()
    applyOptimalDefaults(
      activeWan.value && activeWan.value !== WAN_TAB_ALL ? activeWan.value : '',
    )
  } catch (e) {
    err.value = e.message
  }
}

async function addGooglePreset() {
  if (!egForm.value.wan_link_id) return
  err.value = ''
  ok.value = ''
  const url = googleIpv4Url.value
  try {
    await api.firewall.aliases.add({
      name: 'google_ipv4',
      type: 'ipv4_addr',
      url,
      comment: 'Google IPv4-only ranges',
    })
    await api.network.egressPolicies.add({
      name: 'Google IPv4',
      dst_alias: 'google_ipv4',
      wan_link_id: egForm.value.wan_link_id,
      snat_ip: egForm.value.snat_ip || undefined,
      no_snat: !!egForm.value.no_snat || undefined,
      priority: egForm.value.priority || 100,
      enabled: true,
    })
    ok.value = t('network.egressPolicies.googlePresetOk')
    await load()
  } catch (e) {
    err.value = e.message
  }
}

async function addCloudflarePreset() {
  if (!egForm.value.wan_link_id) return
  err.value = ''
  ok.value = ''
  const prefixes = cloudflareCIDRs.value || []
  if (!prefixes.length) {
    err.value = t('network.egressPolicies.cloudflareEmpty')
    return
  }
  const policies = prefixes.map((cidr) => ({
    name: `Cloudflare CDN ${cidr}`,
    cidr,
    match: 'destination',
    wan_link_id: egForm.value.wan_link_id,
    snat_ip: egForm.value.snat_ip || undefined,
    no_snat: !!egForm.value.no_snat || undefined,
    priority: egForm.value.priority || 100,
    enabled: true,
  }))
  try {
    const res = await api.network.egressPolicies.bulkAdd(policies, true)
    ok.value = t('network.egressPolicies.cloudflarePresetOk', {
      added: res.added || 0,
      skipped: res.skipped || 0,
    })
    await load()
  } catch (e) {
    err.value = e.message
  }
}

function startEdit(p) {
  if (isAutoManagedEgress(p)) return
  editingId.value = p.id
  egEditForm.value = policyToEditForm(p)
  showForm.value = false
}

function cancelEdit() {
  editingId.value = null
}

async function saveEdit() {
  if (!editingId.value) return
  err.value = ''
  try {
    const body = buildEgressBody(egEditForm.value)
    await api.network.egressPolicies.put(editingId.value, body)
    editingId.value = null
    ok.value = t('common.saved')
    await load()
  } catch (e) {
    err.value = e.message
  }
}

async function remove(id) {
  if (!confirm(t('common.delete') + '?')) return
  err.value = ''
  try {
    await api.network.egressPolicies.del(id)
    if (editingId.value === id) editingId.value = null
    await load()
  } catch (e) {
    err.value = e.message
  }
}

watch(
  () => egForm.value.wan_link_id,
  (id) => {
    const link = (links.value || []).find((w) => w.id === id)
    if (link && optimalNoSnatForWan(link)) egForm.value.no_snat = true
  },
)

onMounted(load)
</script>

<template>
  <div class="page-stack">
    <PageHeader
      :title="t('network.egressPolicies.title')"
      :description="t('network.egressPolicies.description')"
      :ok="ok"
      :err="err"
    />

    <div class="card overflow-hidden">
      <div class="flex flex-wrap items-center gap-2 px-3 py-2 border-b bg-slate-50 text-sm">
        <span class="text-slate-600 font-medium">{{ t('network.egressPolicies.filterByWan') }}</span>
        <span class="text-xs text-slate-500 hidden sm:inline">{{ t('network.egressPolicies.wanViewHint') }}</span>
        <span class="ml-auto flex flex-wrap gap-2 items-center">
          <RouterLink to="/network/wan-links" class="text-sm text-blue-600 hover:underline">
            {{ t('network.egressPolicies.manageWanLinks') }}
          </RouterLink>
          <RouterLink to="/firewall/aliases" class="text-sm text-blue-600 hover:underline">
            {{ t('network.egressPolicies.manageAliases') }}
          </RouterLink>
          <button type="button" class="btn-secondary text-sm" @click="showForm = !showForm">
            {{ showForm ? t('network.egressPolicies.hideForm') : t('network.egressPolicies.showForm') }}
          </button>
        </span>
      </div>

      <nav class="eg-wan-tabs flex gap-0.5 overflow-x-auto px-2 py-2 border-b border-slate-100">
        <button
          type="button"
          class="eg-wan-tab shrink-0"
          :class="{ 'eg-wan-tab-active': activeWan === WAN_TAB_ALL }"
          @click="setActiveWan(WAN_TAB_ALL)"
        >
          {{ t('network.egressPolicies.tabAll') }}
          <span class="eg-wan-tab-count">{{ countForWan(WAN_TAB_ALL) }}</span>
        </button>
        <button
          v-for="w in wanTabs"
          :key="w.id"
          type="button"
          class="eg-wan-tab shrink-0"
          :class="{
            'eg-wan-tab-active': activeWan === w.id,
            'eg-wan-tab-policy': w.policy_only || w.warp_managed || w.proxy_managed,
          }"
          @click="setActiveWan(w.id)"
        >
          {{ w.name || w.id }}
          <span v-if="w.device" class="eg-wan-tab-dev">{{ w.device }}</span>
          <span class="eg-wan-tab-count">{{ countForWan(w.id) }}</span>
        </button>
      </nav>

      <div class="px-3 py-1.5 text-xs text-slate-500 border-b bg-white">
        {{ t('network.egressPolicies.viewingWan', { wan: activeWanLabel }) }}
      </div>
    </div>

    <div v-show="showForm" class="card card-body space-y-3 text-sm">
      <div>
        <h3 class="font-medium text-slate-800">{{ t('network.egressPolicies.addTitle') }}</h3>
        <p class="text-xs text-slate-500 mt-1">{{ t('network.egressPolicies.hint') }}</p>
      </div>
      <div class="grid sm:grid-cols-2 gap-3">
        <div>
          <label class="text-xs text-slate-500">{{ t('common.name') }}</label>
          <input v-model="egForm.name" class="input-field mt-1" />
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('network.egressPolicies.wanLink') }}</label>
          <select v-model="egForm.wan_link_id" class="input-field mt-1">
            <option value="">{{ t('network.interfaces.choose') }}</option>
            <option v-for="o in linkOptions" :key="o.id" :value="o.id">{{ o.label }}</option>
          </select>
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('network.egressPolicies.srcIface') }}</label>
          <select v-model="egForm.src_iface" class="input-field mt-1 mb-1">
            <option value="">{{ t('network.egressPolicies.matchAny') }}</option>
            <option v-for="name in ifaceOptions" :key="'eg-iif-' + name" :value="name">{{ name }}</option>
          </select>
          <label class="text-xs text-slate-500">{{ t('network.egressPolicies.srcAddress') }}</label>
          <select v-model="egForm.src_mode" class="input-field mt-1 mb-1">
            <option value="none">{{ t('network.egressPolicies.matchAny') }}</option>
            <option value="cidr">{{ t('network.egressPolicies.matchCidr') }}</option>
            <option value="alias">{{ t('network.egressPolicies.matchAlias') }}</option>
          </select>
          <input
            v-if="egForm.src_mode === 'cidr'"
            v-model="egForm.src_cidr"
            class="input-field font-mono"
            placeholder="198.18.250.0/24"
          />
          <select v-else-if="egForm.src_mode === 'alias'" v-model="egForm.src_alias" class="input-field font-mono">
            <option value="">{{ t('network.interfaces.choose') }}</option>
            <option v-for="a in aliases" :key="a.name" :value="a.name">
              {{ a.name }} ({{ (a.members || []).length }})
            </option>
          </select>
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('network.egressPolicies.dstAddress') }}</label>
          <select v-model="egForm.dst_mode" class="input-field mt-1 mb-1">
            <option value="none">{{ t('network.egressPolicies.matchAny') }}</option>
            <option value="cidr">{{ t('network.egressPolicies.matchCidr') }}</option>
            <option value="alias">{{ t('network.egressPolicies.matchAlias') }}</option>
          </select>
          <input
            v-if="egForm.dst_mode === 'cidr'"
            v-model="egForm.dst_cidr"
            class="input-field font-mono"
            placeholder="173.245.48.0/20"
          />
          <select v-else-if="egForm.dst_mode === 'alias'" v-model="egForm.dst_alias" class="input-field font-mono">
            <option value="">{{ t('network.interfaces.choose') }}</option>
            <option v-for="a in aliases" :key="'d-' + a.name" :value="a.name">
              {{ a.name }} ({{ (a.members || []).length }})
            </option>
          </select>
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('network.egressPolicies.snatIp') }}</label>
          <input
            v-model="egForm.snat_ip"
            class="input-field mt-1 font-mono"
            :disabled="egForm.no_snat"
            :placeholder="egForm.no_snat ? t('network.egressPolicies.snatDisabled') : t('network.egressPolicies.snatAuto')"
          />
          <label class="mt-2 flex items-center gap-2 text-xs text-slate-600">
            <input v-model="egForm.no_snat" type="checkbox" />
            {{ t('network.egressPolicies.noSnat') }}
          </label>
          <p class="mt-1 text-[11px] text-slate-400 leading-snug">{{ t('network.egressPolicies.noSnatHint') }}</p>
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('network.egressPolicies.priority') }}</label>
          <input v-model.number="egForm.priority" type="number" class="input-field mt-1" />
        </div>
        <label class="flex items-center gap-2 sm:col-span-2">
          <input v-model="egForm.enabled" type="checkbox" /> {{ t('common.enabled') }}
        </label>
      </div>
      <div class="flex flex-wrap gap-2">
        <button type="button" class="btn-primary" :disabled="!egForm.wan_link_id" @click="addEgress">
          {{ t('common.add') }}
        </button>
        <button
          type="button"
          class="btn-secondary"
          :disabled="!egForm.wan_link_id || !cloudflareCIDRs.length"
          @click="addCloudflarePreset"
        >
          {{ t('network.egressPolicies.cloudflarePreset') }}
        </button>
        <button type="button" class="btn-secondary" :disabled="!egForm.wan_link_id" @click="addGooglePreset">
          {{ t('network.egressPolicies.googlePreset') }}
        </button>
      </div>
    </div>

    <div class="table-wrap card">
      <table class="data w-full text-sm">
        <thead>
          <tr>
            <th>{{ t('common.name') }}</th>
            <th>{{ t('network.egressPolicies.endpoints') }}</th>
            <th>{{ t('network.egressPolicies.wanLink') }}</th>
            <th>SNAT</th>
            <th>{{ t('network.egressPolicies.routeTable') }}</th>
            <th>{{ t('network.egressPolicies.priority') }}</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="p in filteredEgress" :key="p.id" :class="editingId === p.id ? 'bg-slate-50' : ''">
            <template v-if="editingId === p.id">
              <td><input v-model="egEditForm.name" class="input-field text-xs" /></td>
              <td class="space-y-1 min-w-[14rem]">
                <select v-model="egEditForm.src_iface" class="input-field text-xs">
                  <option value="">{{ t('network.egressPolicies.srcIface') }}: {{ t('network.egressPolicies.matchAny') }}</option>
                  <option v-for="name in ifaceOptions" :key="'eg-eiif-' + name" :value="name">
                    {{ t('network.egressPolicies.srcIface') }}: {{ name }}
                  </option>
                </select>
                <select v-model="egEditForm.src_mode" class="input-field text-xs">
                  <option value="none">{{ t('network.egressPolicies.srcAddress') }}: {{ t('network.egressPolicies.matchAny') }}</option>
                  <option value="cidr">{{ t('network.egressPolicies.srcAddress') }}: CIDR</option>
                  <option value="alias">{{ t('network.egressPolicies.srcAddress') }}: {{ t('network.egressPolicies.matchAlias') }}</option>
                </select>
                <input v-if="egEditForm.src_mode === 'cidr'" v-model="egEditForm.src_cidr" class="input-field text-xs font-mono" />
                <select v-else-if="egEditForm.src_mode === 'alias'" v-model="egEditForm.src_alias" class="input-field text-xs">
                  <option v-for="a in aliases" :key="'es-' + a.name" :value="a.name">{{ a.name }}</option>
                </select>
                <select v-model="egEditForm.dst_mode" class="input-field text-xs">
                  <option value="none">{{ t('network.egressPolicies.dstAddress') }}: {{ t('network.egressPolicies.matchAny') }}</option>
                  <option value="cidr">{{ t('network.egressPolicies.dstAddress') }}: CIDR</option>
                  <option value="alias">{{ t('network.egressPolicies.dstAddress') }}: {{ t('network.egressPolicies.matchAlias') }}</option>
                </select>
                <input v-if="egEditForm.dst_mode === 'cidr'" v-model="egEditForm.dst_cidr" class="input-field text-xs font-mono" />
                <select v-else-if="egEditForm.dst_mode === 'alias'" v-model="egEditForm.dst_alias" class="input-field text-xs">
                  <option v-for="a in aliases" :key="'ed-' + a.name" :value="a.name">{{ a.name }}</option>
                </select>
              </td>
              <td>
                <select v-model="egEditForm.wan_link_id" class="input-field text-xs">
                  <option v-for="o in linkOptions" :key="o.id" :value="o.id">{{ o.label }}</option>
                </select>
              </td>
              <td>
                <div class="space-y-1">
                  <input
                    v-model="egEditForm.snat_ip"
                    class="input-field text-xs font-mono"
                    :disabled="egEditForm.no_snat"
                    :placeholder="egEditForm.no_snat ? t('network.egressPolicies.snatDisabled') : t('network.egressPolicies.snatAuto')"
                  />
                  <label class="inline-flex items-center gap-1 text-xs text-slate-600">
                    <input v-model="egEditForm.no_snat" type="checkbox" />
                    {{ t('network.egressPolicies.noSnatShort') }}
                  </label>
                </div>
              </td>
              <td>{{ resolvedRow(p.id)?.table ?? '—' }}</td>
              <td><input v-model.number="egEditForm.priority" type="number" class="input-field text-xs w-16" /></td>
              <td class="space-x-2 whitespace-nowrap">
                <label class="inline-flex items-center gap-1 text-xs">
                  <input v-model="egEditForm.enabled" type="checkbox" /> {{ t('common.enabled') }}
                </label>
                <button type="button" class="text-indigo-600 text-xs" @click="saveEdit">{{ t('common.save') }}</button>
                <button type="button" class="text-slate-500 text-xs" @click="cancelEdit">{{ t('common.cancel') }}</button>
              </td>
            </template>
            <template v-else>
              <td>
                {{ p.name || p.id }}
                <span
                  v-if="isAutoManagedEgress(p)"
                  class="ml-1 text-[10px] uppercase tracking-wide text-slate-400"
                >{{ t('network.egressPolicies.autoBadge') }}</span>
              </td>
              <td class="font-mono text-xs">{{ endpointsOf(p) }}</td>
              <td class="font-mono">{{ linkName(p.wan_link_id) }}</td>
              <td class="font-mono text-xs">
                <template v-if="p.no_snat || resolvedRow(p.id)?.no_snat">
                  {{ t('network.egressPolicies.noSnatShort') }}
                </template>
                <template v-else>
                  {{ resolvedRow(p.id)?.snat_ip || p.snat_ip || t('network.egressPolicies.snatAuto') }}
                </template>
              </td>
              <td>{{ resolvedRow(p.id)?.table ?? '—' }}</td>
              <td>{{ p.priority }}</td>
              <td class="space-x-2 whitespace-nowrap">
                <template v-if="isAutoManagedEgress(p)">
                  <span class="text-xs text-slate-400">{{ t('network.egressPolicies.autoManaged') }}</span>
                </template>
                <template v-else>
                  <button type="button" class="text-indigo-600 text-xs" @click="startEdit(p)">{{ t('common.edit') }}</button>
                  <button type="button" class="text-red-600 text-xs" @click="remove(p.id)">{{ t('common.delete') }}</button>
                </template>
              </td>
            </template>
          </tr>
          <tr v-if="!filteredEgress.length">
            <td colspan="7" class="text-center text-slate-400 py-3">
              {{ t('network.egressPolicies.empty') }}
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<style scoped>
.eg-wan-tabs {
  @apply overflow-x-auto;
}

.eg-wan-tab {
  @apply inline-flex items-center gap-1 px-3 py-1.5 rounded-md text-xs font-medium text-slate-600
    border border-transparent hover:bg-slate-100 transition-colors;
}

.eg-wan-tab-active {
  @apply bg-white border-slate-300 text-pfsense-nav shadow-sm;
}

.eg-wan-tab-policy.eg-wan-tab-active {
  @apply border-violet-300 bg-violet-50 text-violet-900;
}

.eg-wan-tab-dev {
  @apply text-[10px] font-mono opacity-70;
}

.eg-wan-tab-count {
  @apply ml-0.5 text-[10px] opacity-70 tabular-nums;
}
</style>
