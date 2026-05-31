<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { api } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'
import {
  actionMeta,
  autoRulesForChain,
  builtinRulesForChain,
  formatDestination,
  formatIface,
  formatPort,
  formatProto,
  formatSource,
  isRuleAutoManaged,
  mergeChainReorder,
  ruleDetailLines,
  ruleMatchesSearch,
  builtinMatchesSearch,
  userRulesForChain,
} from '@/lib/firewallRuleDisplay'
import {
  filterBuiltinByIface,
  filterRulesByIface,
  IFACE_ALL,
  IFACE_FLOATING,
  ifaceFormDefaults,
  ifaceTabLabel,
  mergeChainReorderForIface,
  ruleTouchesIface,
  wanDeviceNames,
} from '@/lib/firewallIface'
import {
  emptyRuleForm,
  formToPayload,
  isRuleMutable,
  PROTO_OPTIONS,
  ruleToForm,
  validateRuleForm,
} from '@/lib/firewallRuleForm'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
let syncingFromRoute = false

const rules = ref([])
const devLan = ref('')
const devWan = ref('')
const adminPort = ref('')
const aliasNames = ref([])
const vpnMeta = ref({})
const activeChain = ref('forward')
const showRendered = ref(false)
const showForm = ref(false)
const rendered = ref('')
const err = ref('')
const ok = ref('')
const dragIdx = ref(null)
const savingOrder = ref(false)
const editing = ref(null)
const formPanel = ref(null)
const searchQuery = ref('')
const acmeTempAllow = ref(false)
const selectedRule = ref(null)
const nftLines = ref({})
const ifaceList = ref([])
const activeIface = ref(IFACE_ALL)
const formHint = ref('')
const pendingRuleId = ref('')
const previewLine = ref('')
const previewLoading = ref(false)
const hasPendingChanges = ref(false)
const canApplyChanges = ref(false)
const changeIssues = ref([])
const changeDiff = ref({ added: [], modified: [], removed: [] })
const applyBusy = ref(false)

const chains = [
  { id: 'forward', labelKey: 'security.firewall.tabForward' },
  { id: 'input', labelKey: 'security.firewall.tabInput' },
]

const form = ref(emptyRuleForm())

const extraWanDevices = computed(() => wanDeviceNames(ifaceList.value, devWan.value))

function filterUserByIface(list) {
  return filterRulesByIface(list, activeIface.value, activeChain.value)
}

function filterAutoByIface(list) {
  return filterRulesByIface(list, activeIface.value, activeChain.value)
}

const userRulesInChainRaw = computed(() =>
  filterUserByIface(userRulesForChain(rules.value, activeChain.value)),
)

const userRulesInChain = computed(() => {
  const q = searchQuery.value.trim()
  if (!q) return userRulesInChainRaw.value
  return userRulesInChainRaw.value.filter((r) =>
    ruleMatchesSearch(r, q, devLan.value, devWan.value),
  )
})

const autoRulesInChainRaw = computed(() =>
  filterAutoByIface(autoRulesForChain(rules.value, activeChain.value)),
)

const autoRulesInChain = computed(() => {
  const q = searchQuery.value.trim()
  if (!q) return autoRulesInChainRaw.value
  return autoRulesInChainRaw.value.filter((r) =>
    ruleMatchesSearch(r, q, devLan.value, devWan.value),
  )
})

const showOutCol = computed(() => activeChain.value === 'forward')

const tableColspan = computed(() => (showOutCol.value ? 13 : 12))

const builtinCtx = computed(() => ({
  adminPort: adminPort.value,
  vpn: vpnMeta.value,
  acmeTempAllow: acmeTempAllow.value,
  ifaceList: ifaceList.value,
}))

const builtinRowsRaw = computed(() => {
  const rows = builtinRulesForChain(activeChain.value, devLan.value, devWan.value, builtinCtx.value, t)
  return filterBuiltinByIface(rows, activeIface.value, devLan.value, devWan.value, extraWanDevices.value)
})

const builtinRows = computed(() => {
  const q = searchQuery.value.trim()
  if (!q) return builtinRowsRaw.value
  return builtinRowsRaw.value.filter((br) => builtinMatchesSearch(br, q))
})

const activeIfaceLabel = computed(() => {
  if (!activeIface.value || activeIface.value === IFACE_ALL) return t('security.firewall.tabAll')
  if (activeIface.value === IFACE_FLOATING) return t('security.firewall.tabFloating')
  const hit = ifaceList.value.find((x) => x.name === activeIface.value)
  return hit ? ifaceTabLabel(hit, t) : activeIface.value
})

function ruleCountOnIface(iface, chainId = activeChain.value) {
  const ch = chainId
  const u = filterRulesByIface(userRulesForChain(rules.value, ch), iface, ch).length
  const a = filterRulesByIface(autoRulesForChain(rules.value, ch), iface, ch).length
  return u + a
}

const customCount = computed(() => userRulesInChainRaw.value.length)
const autoCount = computed(() => autoRulesInChain.value.length)

const hasAutoForwardRules = computed(() =>
  (rules.value || []).some((r) => String(r.id || '').startsWith('auto-fwd-')),
)

const isInputChain = computed(() => form.value.chain === 'input')

const formChainTabMismatch = computed(() => showForm.value && form.value.chain !== activeChain.value)

const formResolvedIif = computed(() => {
  if (!showForm.value) return ''
  return formToPayload(form.value, devLan.value, devWan.value).iif || ''
})

const formWanDropWarning = computed(() => {
  const f = form.value
  if (!showForm.value || f.chain !== 'input' || f.action !== 'drop') return false
  const iif = formResolvedIif.value
  if (!iif) return false
  if (iif === devWan.value) return true
  return wanDeviceNames(ifaceList.value, devWan.value).includes(iif)
})

const ifaceReorderHint = computed(
  () =>
    activeIface.value &&
    activeIface.value !== IFACE_ALL &&
    activeIface.value !== IFACE_FLOATING,
)

const detailLines = computed(() => {
  if (!selectedRule.value) return []
  return ruleDetailLines(selectedRule.value, devLan.value, devWan.value, t)
})

const selectedNftLine = computed(() => {
  const id = selectedRule.value?.id
  if (!id) return ''
  return nftLines.value[id] || ''
})

const selectedNftHint = computed(() => {
  const r = selectedRule.value
  if (!r || selectedNftLine.value) return ''
  if (r.enabled === false) return t('security.firewall.detailNftDisabled')
  if (r.system && !r.iif && !r.oif && !r.proto) return t('security.firewall.detailNftBuiltin')
  return ''
})

watch(activeChain, () => {
  selectedRule.value = null
  syncFirewallRoute()
})

watch(activeIface, () => {
  selectedRule.value = null
  dragIdx.value = null
  syncFirewallRoute()
})

watch(selectedRule, () => {
  syncFirewallRoute()
})

function applyIfaceToForm(f) {
  const d = ifaceFormDefaults(activeIface.value, devLan.value, devWan.value, ifaceList.value)
  f.iif_mode = d.iif_mode
  f.oif_mode = d.oif_mode
  if (d.iif_mode === 'custom') {
    f.iif_custom = activeIface.value
  } else {
    f.iif_custom = ''
  }
  if (d.oif_mode === 'custom') {
    f.oif_custom = activeIface.value
  }
}

function applyRouteQuery() {
  const q = route.query
  if (q.chain === 'forward' || q.chain === 'input') {
    activeChain.value = q.chain
  } else if (!q.chain && !syncingFromRoute) {
    activeChain.value = 'forward'
  }
  const iface = typeof q.iface === 'string' ? q.iface.trim() : ''
  if (iface) {
    activeIface.value = iface
  } else if (!q.iface && !syncingFromRoute) {
    activeIface.value = IFACE_ALL
  }
  const ruleId = typeof q.rule === 'string' ? q.rule.trim() : ''
  if (ruleId) {
    pendingRuleId.value = ruleId
  } else if (!q.rule && !syncingFromRoute) {
    selectedRule.value = null
  }
}

function firewallQueryEqual(a, b) {
  const keys = new Set([...Object.keys(a || {}), ...Object.keys(b || {})])
  for (const k of keys) {
    if (String(a?.[k] ?? '') !== String(b?.[k] ?? '')) return false
  }
  return true
}

function buildFirewallQuery() {
  const q = {}
  if (activeChain.value === 'input') q.chain = 'input'
  if (activeIface.value && activeIface.value !== IFACE_ALL) q.iface = activeIface.value
  if (selectedRule.value?.id) q.rule = selectedRule.value.id
  return q
}

function syncFirewallRoute() {
  if (syncingFromRoute) return
  const next = buildFirewallQuery()
  if (firewallQueryEqual(route.query, next)) return
  syncingFromRoute = true
  router
    .replace({ name: 'firewall-rules', query: next })
    .catch(() => {})
    .finally(() => {
      syncingFromRoute = false
    })
}

function focusPendingRule() {
  const id = pendingRuleId.value
  if (!id) return
  const hit = (rules.value || []).find((r) => r.id === id)
  pendingRuleId.value = ''
  if (!hit) return
  if (hit.chain) activeChain.value = hit.chain
  activeIface.value = IFACE_ALL
  openView(hit)
}

async function load() {
  const d = await api.firewall.rules.list()
  rules.value = d.rules || []
  syncChangesFromPayload(d.changes)
  devLan.value = d.dev_lan || ''
  devWan.value = d.dev_wan || ''
  adminPort.value = d.admin_port || ''
  aliasNames.value = d.alias_names || []
  vpnMeta.value = d.vpn || {}
  acmeTempAllow.value = !!d.acme_temp_allow_http01
  rendered.value = d.rendered || ''
  ifaceList.value = d.interfaces || []
  nftLines.value = d.nft_lines || {}
  focusPendingRule()
}

function openAdd() {
  editing.value = null
  previewLine.value = ''
  const f = emptyRuleForm(activeChain.value)
  applyIfaceToForm(f)
  form.value = f
  showForm.value = true
}

function syncChangesFromPayload(changes) {
  if (!changes) {
    hasPendingChanges.value = false
    canApplyChanges.value = false
    changeIssues.value = []
    changeDiff.value = { added: [], modified: [], removed: [] }
    return
  }
  hasPendingChanges.value = !!changes.has_pending_changes
  canApplyChanges.value = !!changes.can_apply
  changeIssues.value = changes.issues || []
  changeDiff.value = changes.diff || { added: [], modified: [], removed: [] }
}

function applyStageResponse(res) {
  if (res?.rules) rules.value = res.rules
  syncChangesFromPayload(res?.changes)
  if (res?.warning) warn.value = res.warning
}

async function applyPendingChanges() {
  if (!hasPendingChanges.value) return
  if (!canApplyChanges.value) {
    err.value = t('security.firewall.cannotApply')
    return
  }
  if (!confirm(t('security.firewall.applyBarHint'))) return
  applyBusy.value = true
  err.value = ''
  ok.value = ''
  try {
    const res = await api.firewall.apply()
    if (res?.rules) rules.value = res.rules
    syncChangesFromPayload(res?.changes)
    ok.value = t('security.firewall.appliedOk')
  } catch (e) {
    err.value = apiError(e)
    if (e?.data?.changes) syncChangesFromPayload(e.data.changes)
  } finally {
    applyBusy.value = false
  }
}

async function discardPendingChanges() {
  if (!hasPendingChanges.value) return
  if (!confirm(t('security.firewall.discardChanges') + '?')) return
  applyBusy.value = true
  err.value = ''
  ok.value = ''
  try {
    const res = await api.firewall.discard()
    if (res?.rules) rules.value = res.rules
    syncChangesFromPayload(res?.changes)
    ok.value = t('security.firewall.discardedOk')
  } catch (e) {
    err.value = apiError(e)
  } finally {
    applyBusy.value = false
  }
}

const warn = ref('')

function apiError(e) {
  return e?.data?.error || e?.message || String(e)
}

function startEdit(r) {
  if (!isRuleMutable(r)) {
    err.value = t('security.firewall.ruleLocked')
    return
  }
  editing.value = r.id
  previewLine.value = ''
  form.value = ruleToForm(r, devLan.value, devWan.value)
  showForm.value = true
  requestAnimationFrame(() => {
    formPanel.value?.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
  })
}

function cancelEdit() {
  editing.value = null
  form.value = emptyRuleForm(activeChain.value)
  formHint.value = ''
  previewLine.value = ''
  showForm.value = false
}

function buildPayload() {
  return formToPayload(form.value, devLan.value, devWan.value)
}

async function previewNft() {
  if (!runFormValidation()) return
  err.value = ''
  ok.value = ''
  previewLine.value = ''
  previewLoading.value = true
  try {
    const payload = buildPayload()
    const res = editing.value
      ? await api.firewall.rules.put(editing.value, { ...payload, id: editing.value }, { dryRun: true })
      : await api.firewall.rules.add(payload, { dryRun: true })
    previewLine.value = res.nft_line || ''
    ok.value = t('security.firewall.previewNftOk')
  } catch (e) {
    err.value = apiError(e)
  } finally {
    previewLoading.value = false
  }
}

function runFormValidation() {
  const problems = validateRuleForm(form.value, t, aliasNames.value)
  if (problems.length) {
    err.value = `${t('security.firewall.formErrors')} ${problems.join('；')}`
    return false
  }
  return true
}

function openView(r) {
  selectedRule.value = r
}

function duplicateRule(r) {
  if (!isRuleMutable(r)) return
  editing.value = null
  if (r.chain) activeChain.value = r.chain
  form.value = ruleToForm(r, devLan.value, devWan.value)
  showForm.value = true
}

function applyPreset(kind) {
  editing.value = null
  formHint.value = ''
  const f = emptyRuleForm(activeChain.value)
  const wanDev =
    activeIface.value && activeIface.value !== IFACE_ALL && activeIface.value !== IFACE_FLOATING
      ? activeIface.value
      : devWan.value
  if (kind === 'lan-in' && (devLan.value || activeIface.value === devLan.value)) {
    f.chain = 'input'
    activeChain.value = 'input'
    f.action = 'accept'
    f.iif_mode = devLan.value ? 'lan' : 'custom'
    if (f.iif_mode === 'custom') f.iif_custom = activeIface.value || devLan.value
    f.comment = t('security.firewall.presetAllowLan')
  } else if (kind === 'wan-block' && wanDev) {
    f.chain = 'input'
    activeChain.value = 'input'
    f.action = 'drop'
    if (wanDev === devWan.value) f.iif_mode = 'wan'
    else {
      f.iif_mode = 'custom'
      f.iif_custom = wanDev
    }
    f.comment = t('security.firewall.presetBlockWan')
    formHint.value = t('security.firewall.presetBlockWanHint', {
      port: adminPort.value || '8080',
    })
  } else if (kind === 'tcp-allow') {
    f.action = 'accept'
    f.proto = 'tcp'
    f.dst_port_mode = 'custom'
    f.dst_port_custom = '443'
    f.comment = t('security.firewall.presetAllowTcp')
    applyIfaceToForm(f)
  }
  form.value = f
  showForm.value = true
}

async function add() {
  if (!runFormValidation()) return
  err.value = ''
  warn.value = ''
  try {
    const res = await api.firewall.rules.add(buildPayload())
    applyStageResponse(res)
    ok.value = t('security.firewall.stagedOk')
    cancelEdit()
  } catch (e) {
    err.value = apiError(e)
  }
}

async function saveEdit() {
  if (!editing.value) return
  if (!runFormValidation()) return
  err.value = ''
  ok.value = ''
  warn.value = ''
  try {
    const res = await api.firewall.rules.put(editing.value, { ...buildPayload(), id: editing.value })
    applyStageResponse(res)
    ok.value = t('security.firewall.stagedOk')
    cancelEdit()
  } catch (e) {
    err.value = apiError(e)
  }
}

async function toggleEnabled(r) {
  if (!isRuleMutable(r)) return
  if (r.enabled && !confirm(t('security.firewall.confirmDisable'))) return
  err.value = ''
  ok.value = ''
  warn.value = ''
  try {
    const res = await api.firewall.rules.put(r.id, { ...r, enabled: !r.enabled })
    applyStageResponse(res)
    ok.value = t('security.firewall.stagedOk')
  } catch (e) {
    err.value = apiError(e)
  }
}

async function remove(r) {
  if (!isRuleMutable(r)) {
    err.value = t('security.firewall.ruleLocked')
    return
  }
  if (!confirm(t('security.firewall.confirmDelete'))) return
  err.value = ''
  ok.value = ''
  warn.value = ''
  try {
    const res = await api.firewall.rules.del(r.id)
    applyStageResponse(res)
    if (editing.value === r.id) cancelEdit()
    ok.value = t('security.firewall.stagedOk')
  } catch (e) {
    err.value = apiError(e)
  }
}

function onDragStart(idx) {
  dragIdx.value = idx
}

function onDragOver(e) {
  e.preventDefault()
}

async function persistOrder(reorderedSubset) {
  savingOrder.value = true
  err.value = ''
  try {
    const merged =
      activeIface.value && activeIface.value !== IFACE_ALL
        ? mergeChainReorderForIface(rules.value, activeChain.value, activeIface.value, reorderedSubset)
        : mergeChainReorder(rules.value, activeChain.value, reorderedSubset)
    const orderIds = merged.filter((r) => isRuleMutable(r)).map((r) => r.id)
    if (!orderIds.length) {
      rules.value = merged
      ok.value = t('security.firewall.orderSaved')
      return
    }
    const res = await api.firewall.rules.reorder(orderIds)
    applyStageResponse(res)
    ok.value = t('security.firewall.stagedOk')
  } catch (e) {
    err.value = apiError(e)
  } finally {
    savingOrder.value = false
  }
}

async function onDrop(targetIdx) {
  if (dragIdx.value === null || dragIdx.value === targetIdx) {
    dragIdx.value = null
    return
  }
  const arr = [...userRulesInChainRaw.value]
  const [item] = arr.splice(dragIdx.value, 1)
  arr.splice(targetIdx, 0, item)
  dragIdx.value = null
  await persistOrder(arr)
}

async function moveRule(idx, dir) {
  const displayed = userRulesInChain.value
  const raw = userRulesInChainRaw.value
  if (searchQuery.value.trim()) {
    const id = displayed[idx]?.id
    const rawIdx = raw.findIndex((r) => r.id === id)
    if (rawIdx < 0) return
    idx = rawIdx
  }
  const j = idx + dir
  if (j < 0 || j >= raw.length) return
  const arr = [...raw]
  ;[arr[idx], arr[j]] = [arr[j], arr[idx]]
  await persistOrder(arr)
}

function rowActionLabel(action) {
  const m = actionMeta(action)
  const key = { pass: 'actionPass', block: 'actionBlock', reject: 'actionReject' }[m.badge]
  return t(`security.firewall.${key}`)
}

watch(
  () => route.query,
  () => {
    if (syncingFromRoute) return
    applyRouteQuery()
    focusPendingRule()
  },
)

onMounted(() => {
  applyRouteQuery()
  load().then(() => syncFirewallRoute())
})
</script>

<template>
  <div class="page-stack fw-page">
    <PageHeader :title="t('security.firewall.title')" :description="t('security.firewall.description')" :ok="ok" :err="err" />

    <div
      v-if="hasPendingChanges"
      class="rounded-lg border border-amber-300 bg-amber-50 text-amber-950 p-4 mb-3 space-y-3"
      role="status"
    >
      <div class="flex flex-wrap items-start gap-3 justify-between">
        <div>
          <p class="font-semibold">{{ t('security.firewall.applyBarTitle') }}</p>
          <p class="text-sm mt-1 text-amber-900">{{ t('security.firewall.applyBarHint') }}</p>
          <p v-if="warn" class="text-sm mt-2 text-red-700">{{ warn }}</p>
        </div>
        <div class="flex flex-wrap gap-2">
          <button
            type="button"
            class="btn-primary text-sm"
            :disabled="applyBusy || !canApplyChanges"
            @click="applyPendingChanges"
          >
            {{ applyBusy ? t('common.processing') : t('security.firewall.applyChanges') }}
          </button>
          <button type="button" class="btn-secondary text-sm" :disabled="applyBusy" @click="discardPendingChanges">
            {{ t('security.firewall.discardChanges') }}
          </button>
        </div>
      </div>
      <div v-if="changeDiff.added?.length || changeDiff.modified?.length || changeDiff.removed?.length" class="text-xs">
        <span v-if="changeDiff.added?.length" class="mr-3">{{ t('security.firewall.diffAdded') }}: {{ changeDiff.added.join(', ') }}</span>
        <span v-if="changeDiff.modified?.length" class="mr-3">{{ t('security.firewall.diffModified') }}: {{ changeDiff.modified.join(', ') }}</span>
        <span v-if="changeDiff.removed?.length">{{ t('security.firewall.diffRemoved') }}: {{ changeDiff.removed.join(', ') }}</span>
      </div>
      <div v-if="changeIssues.length" class="border-t border-amber-200 pt-3 space-y-2">
        <p class="text-sm font-medium">{{ t('security.firewall.changesReview') }}</p>
        <ul class="text-sm space-y-2">
          <li
            v-for="(iss, idx) in changeIssues"
            :key="idx"
            class="rounded border p-2"
            :class="iss.severity === 'error' ? 'border-red-300 bg-red-50' : 'border-amber-200 bg-white'"
          >
            <span class="font-mono text-xs mr-2">{{ iss.rule_id || '—' }}</span>
            <span class="font-semibold">{{
              iss.severity === 'error' ? t('security.firewall.issueSeverityError') : t('security.firewall.issueSeverityWarn')
            }}</span>
            : {{ iss.message }}
            <p v-if="iss.hint" class="text-xs mt-1 text-slate-600">{{ iss.hint }}</p>
          </li>
        </ul>
      </div>
    </div>

    <!-- 网卡切换（pfSense 风格） -->
    <div class="card overflow-hidden">
      <div class="flex flex-wrap items-center gap-2 px-3 py-2 border-b bg-slate-50 text-sm">
        <span class="text-slate-600 font-medium">{{ t('security.firewall.filterByIface') }}</span>
        <span class="text-xs text-slate-500 hidden sm:inline">{{ t('security.firewall.ifaceViewHint') }}</span>
        <span class="ml-auto flex flex-wrap gap-2 items-center">
          <RouterLink to="/firewall/aliases" class="text-sm text-blue-600 hover:underline">
            {{ t('security.firewall.manageAliases') }}
          </RouterLink>
          <button type="button" class="btn-primary text-sm" @click="openAdd">
            {{ t('security.firewall.addRule') }}
          </button>
          <button type="button" class="btn-secondary text-sm" @click="showForm = !showForm">
            {{ showForm ? t('security.firewall.hideForm') : t('security.firewall.showForm') }}
          </button>
        </span>
      </div>
      <nav class="fw-iface-tabs flex gap-0.5 overflow-x-auto px-2 py-2 border-b border-slate-100">
        <button
          type="button"
          class="fw-iface-tab shrink-0"
          :class="{ 'fw-iface-tab-active': activeIface === IFACE_ALL }"
          @click="activeIface = IFACE_ALL"
        >
          {{ t('security.firewall.tabAll') }}
          <span class="fw-iface-tab-count">{{ ruleCountOnIface(IFACE_ALL) }}</span>
        </button>
        <button
          v-for="item in ifaceList"
          :key="item.name"
          type="button"
          class="fw-iface-tab shrink-0"
          :class="{
            'fw-iface-tab-active': activeIface === item.name,
            'fw-iface-tab-lan': item.role === 'LAN',
            'fw-iface-tab-wan': item.role === 'WAN',
          }"
          @click="activeIface = item.name"
        >
          <span class="font-mono text-xs">{{ item.name }}</span>
          <span v-if="item.label" class="opacity-80 text-[10px] ml-1">{{ item.label }}</span>
          <span v-if="item.role" class="fw-iface-tab-role">{{ item.role }}</span>
          <span class="fw-iface-tab-count">{{ ruleCountOnIface(item.name) }}</span>
        </button>
        <button
          type="button"
          class="fw-iface-tab shrink-0"
          :class="{ 'fw-iface-tab-active': activeIface === IFACE_FLOATING }"
          @click="activeIface = IFACE_FLOATING"
        >
          {{ t('security.firewall.tabFloating') }}
          <span class="fw-iface-tab-count">{{ ruleCountOnIface(IFACE_FLOATING) }}</span>
        </button>
      </nav>
      <p class="text-xs text-slate-500 px-3 py-1.5 bg-white">
        {{ t('security.firewall.viewingIface', { iface: activeIfaceLabel }) }}
        <span v-if="activeChain === 'forward'" class="ml-2">{{ t('security.firewall.forwardIfaceHint') }}</span>
        <span v-else class="ml-2">{{ t('security.firewall.inputIfaceHint') }}</span>
      </p>
    </div>

    <div class="flex flex-wrap gap-2 items-center">
      <input
        v-model="searchQuery"
        type="search"
        class="input-field text-sm max-w-md flex-1 min-w-[12rem]"
        :placeholder="t('security.firewall.searchPh')"
      />
      <button type="button" class="btn-secondary text-xs" @click="applyPreset('lan-in')">
        {{ t('security.firewall.presetAllowLan') }}
      </button>
      <button type="button" class="btn-secondary text-xs" @click="applyPreset('wan-block')">
        {{ t('security.firewall.presetBlockWan') }}
      </button>
      <button type="button" class="btn-secondary text-xs" @click="applyPreset('tcp-allow')">
        {{ t('security.firewall.presetAllowTcp') }}
      </button>
    </div>

    <!-- 链 Tab -->
    <nav class="fw-tabs flex gap-1 border-b border-slate-200">
      <button
        v-for="c in chains"
        :key="c.id"
        type="button"
        class="fw-tab px-4 py-2 text-sm font-medium rounded-t transition-colors"
        :class="
          activeChain === c.id
            ? 'bg-white border border-b-white border-slate-200 text-pfsense-nav -mb-px'
            : 'text-slate-500 hover:text-slate-800 hover:bg-slate-50'
        "
        @click="activeChain = c.id"
      >
        {{ t(c.labelKey) }}
        <span class="ml-1 text-xs opacity-70">({{ ruleCountOnIface(activeIface, c.id) }})</span>
      </button>
    </nav>

    <!-- 规则表（pfSense 风格） -->
    <div class="card overflow-hidden border-t-0 rounded-t-none">
      <p v-if="savingOrder" class="text-xs text-slate-500 px-3 py-2 bg-slate-50 border-b">
        {{ t('security.firewall.savingOrder') }}
      </p>

      <div class="table-wrap">
        <table class="data fw-rules-table w-full text-sm">
          <thead>
            <tr>
              <th class="fw-col-drag w-8"></th>
              <th class="w-10 text-center">#</th>
              <th class="w-24">{{ t('security.firewall.colAction') }}</th>
              <th class="w-28">{{ t('security.firewall.colIn') }}</th>
              <th v-if="showOutCol" class="w-28">{{ t('security.firewall.colOut') }}</th>
              <th class="w-16">{{ t('security.firewall.colProto') }}</th>
              <th>{{ t('security.firewall.colSource') }}</th>
              <th class="w-16 text-center">{{ t('security.firewall.colSPort') }}</th>
              <th>{{ t('security.firewall.colDest') }}</th>
              <th class="w-16 text-center">{{ t('security.firewall.colDPort') }}</th>
              <th class="w-16 text-center">{{ t('security.firewall.colStatus') }}</th>
              <th>{{ t('security.firewall.colDescription') }}</th>
              <th class="w-32 text-right">{{ t('security.firewall.colActions') }}</th>
            </tr>
          </thead>
          <tbody>
            <!-- 系统默认规则 -->
            <tr class="fw-section-row">
              <td :colspan="tableColspan" class="!py-1.5 !bg-slate-100 !text-xs !font-semibold !text-slate-600">
                {{ t('security.firewall.sectionSystem') }}
              </td>
            </tr>
            <tr
              v-for="(br, bi) in builtinRows"
              :key="br.id"
              class="fw-row-system"
            >
              <td class="text-center text-slate-300">—</td>
              <td class="text-center text-xs text-slate-400">{{ bi + 1 }}</td>
              <td>
                <span
                  class="fw-action-badge"
                  :class="actionMeta(br.action).class"
                >
                  {{ rowActionLabel(br.action) }}
                </span>
              </td>
              <td :colspan="showOutCol ? 2 : 1" class="text-xs text-slate-600">{{ br.summary }}</td>
              <td class="text-center text-slate-400">*</td>
              <td class="text-slate-400">*</td>
              <td class="text-center text-slate-400">*</td>
              <td class="text-slate-400">*</td>
              <td class="text-center text-slate-400">*</td>
              <td class="text-center">
                <span class="fw-status-on text-[10px]">{{ t('security.firewall.statusOn') }}</span>
              </td>
              <td class="text-xs text-slate-500">
                {{ br.detail || t('security.firewall.systemRule') }}
              </td>
              <td></td>
            </tr>

            <!-- 自动同步规则 -->
            <tr class="fw-section-row">
              <td :colspan="tableColspan" class="!py-1.5 !bg-amber-50 !text-xs !font-semibold !text-amber-900">
                {{ t('security.firewall.sectionAuto') }}
                <span class="font-normal text-amber-800">({{ autoCount }})</span>
              </td>
            </tr>
            <tr v-if="hasAutoForwardRules && activeChain === 'forward'" class="fw-row-hint">
              <td :colspan="tableColspan" class="!py-2 !bg-amber-50/80 !text-xs !text-amber-950 leading-relaxed">
                {{ t('security.firewall.portForwardSyncHint') }}
                <RouterLink to="/nat/forwards" class="text-blue-700 hover:underline font-medium ml-1">
                  {{ t('security.firewall.portForwardLink') }} →
                </RouterLink>
              </td>
            </tr>
            <tr v-if="autoCount === 0" class="fw-row-empty">
              <td :colspan="tableColspan" class="text-center text-slate-400 py-4 text-sm">
                {{ t('security.firewall.noAuto') }}
              </td>
            </tr>
            <tr
              v-for="(r, aidx) in autoRulesInChain"
              :key="r.id"
              class="fw-row-auto"
              :class="{ 'bg-amber-50/50 ring-1 ring-amber-200': selectedRule?.id === r.id }"
            >
              <td class="text-center text-amber-400 text-xs" :title="t('security.firewall.ruleLocked')">🔒</td>
              <td class="text-center text-xs text-slate-500 font-mono">A{{ aidx + 1 }}</td>
              <td>
                <span class="fw-action-badge" :class="actionMeta(r.action).class">
                  {{ rowActionLabel(r.action) }}
                </span>
              </td>
              <td>
                <span v-if="formatIface(r.iif, devLan, devWan).name !== '—'" class="fw-iface-cell">
                  <span v-if="formatIface(r.iif, devLan, devWan).roleKey" class="fw-mini-tag">{{
                    formatIface(r.iif, devLan, devWan).roleKey === 'lan'
                      ? t('security.firewall.roleLan')
                      : t('security.firewall.roleWan')
                  }}</span>
                  <span class="font-mono text-xs">{{ formatIface(r.iif, devLan, devWan).name }}</span>
                </span>
                <span v-else class="text-slate-400">*</span>
              </td>
              <td v-if="showOutCol">
                <span class="text-slate-400">*</span>
              </td>
              <td class="font-mono text-xs text-center">{{ formatProto(r.proto) }}</td>
              <td>
                <span class="font-mono text-xs">{{ formatSource(r).label }}</span>
              </td>
              <td class="text-center font-mono text-xs">{{ formatPort(r.src_port) }}</td>
              <td>
                <span class="font-mono text-xs">{{ formatDestination(r).label }}</span>
              </td>
              <td class="text-center font-mono text-xs">{{ formatPort(r.dst_port) }}</td>
              <td class="text-center">
                <span class="fw-status-on text-[10px]">{{ t('security.firewall.statusOn') }}</span>
              </td>
              <td class="text-xs text-slate-700 max-w-[12rem] truncate" :title="r.comment">
                <span class="fw-auto-badge">{{ t('security.firewall.autoRule') }}</span>
                {{ r.comment || '—' }}
              </td>
              <td class="text-right whitespace-nowrap">
                <button type="button" class="fw-icon-btn text-slate-600" @click.stop="openView(r)">
                  {{ t('security.firewall.viewRule') }}
                </button>
              </td>
            </tr>

            <!-- 自定义规则 -->
            <tr class="fw-section-row">
              <td :colspan="tableColspan" class="!py-1.5 !bg-blue-50 !text-xs !font-semibold !text-blue-900">
                {{ t('security.firewall.sectionCustom') }}
                <span class="font-normal text-blue-700">({{ customCount }})</span>
              </td>
            </tr>
            <tr v-if="customCount === 0" class="fw-row-empty">
              <td :colspan="tableColspan" class="text-center text-slate-400 py-6 text-sm">
                {{ t('security.firewall.noCustom') }}
              </td>
            </tr>
            <tr
              v-for="(r, idx) in userRulesInChain"
              :key="r.id || `row-${idx}`"
              class="fw-row-custom"
              :class="{
                'opacity-40': !r.enabled,
                'opacity-50': dragIdx === idx,
                'bg-blue-50/80 ring-1 ring-blue-200': editing === r.id,
              }"
              @dragover="onDragOver"
              @drop="onDrop(idx)"
            >
              <td
                class="text-slate-400 text-center select-none text-xs cursor-grab active:cursor-grabbing"
                :draggable="!searchQuery.trim()"
                :title="
                  searchQuery.trim()
                    ? t('security.firewall.searchPh')
                    : ifaceReorderHint
                      ? t('security.firewall.dragHintIface')
                      : t('security.firewall.dragHint')
                "
                @dragstart="onDragStart(idx)"
              >
                ⋮⋮
              </td>
              <td class="text-center text-xs text-slate-500 font-mono">{{ idx + 1 }}</td>
              <td>
                <span class="fw-action-badge" :class="actionMeta(r.action).class">
                  {{ rowActionLabel(r.action) }}
                </span>
              </td>
              <td>
                <span v-if="formatIface(r.iif, devLan, devWan).name !== '—'" class="fw-iface-cell">
                  <span v-if="formatIface(r.iif, devLan, devWan).roleKey" class="fw-mini-tag">{{
                    formatIface(r.iif, devLan, devWan).roleKey === 'lan'
                      ? t('security.firewall.roleLan')
                      : t('security.firewall.roleWan')
                  }}</span>
                  <span class="font-mono text-xs">{{ formatIface(r.iif, devLan, devWan).name }}</span>
                </span>
                <span v-else class="text-slate-400">*</span>
              </td>
              <td v-if="showOutCol">
                <span v-if="formatIface(r.oif, devLan, devWan).name !== '—'" class="fw-iface-cell">
                  <span v-if="formatIface(r.oif, devLan, devWan).roleKey" class="fw-mini-tag">{{
                    formatIface(r.oif, devLan, devWan).roleKey === 'lan'
                      ? t('security.firewall.roleLan')
                      : t('security.firewall.roleWan')
                  }}</span>
                  <span class="font-mono text-xs">{{ formatIface(r.oif, devLan, devWan).name }}</span>
                </span>
                <span v-else class="text-slate-400">*</span>
              </td>
              <td class="font-mono text-xs text-center">{{ formatProto(r.proto) }}</td>
              <td>
                <span
                  class="font-mono text-xs"
                  :class="formatSource(r).kind === 'alias' ? 'text-violet-700' : ''"
                >
                  <template v-if="formatSource(r).kind === 'alias'">@{{ formatSource(r).label }}</template>
                  <template v-else>{{ formatSource(r).label }}</template>
                </span>
              </td>
              <td class="text-center font-mono text-xs">{{ formatPort(r.src_port) }}</td>
              <td>
                <span
                  class="font-mono text-xs"
                  :class="formatDestination(r).kind === 'alias' ? 'text-violet-700' : ''"
                >
                  <template v-if="formatDestination(r).kind === 'alias'"
                    >@{{ formatDestination(r).label }}</template
                  >
                  <template v-else>{{ formatDestination(r).label }}</template>
                </span>
              </td>
              <td class="text-center font-mono text-xs">{{ formatPort(r.dst_port) }}</td>
              <td class="text-center">
                <span
                  class="text-[10px] font-medium px-1.5 py-0.5 rounded"
                  :class="r.enabled !== false ? 'fw-status-on' : 'fw-status-off'"
                >
                  {{ r.enabled !== false ? t('security.firewall.statusOn') : t('security.firewall.statusOff') }}
                </span>
              </td>
              <td class="text-xs text-slate-700 max-w-[12rem] truncate" :title="r.comment || r.id">
                {{ r.comment || '—' }}
              </td>
              <td class="text-right whitespace-nowrap fw-col-actions">
                <button type="button" class="fw-icon-btn text-slate-500" @click.stop="openView(r)">
                  {{ t('security.firewall.viewRule') }}
                </button>
                <template v-if="isRuleMutable(r)">
                  <button
                    type="button"
                    class="fw-icon-btn"
                    :title="t('security.firewall.moveUp')"
                    @click.stop="moveRule(idx, -1)"
                  >
                    ↑
                  </button>
                  <button
                    type="button"
                    class="fw-icon-btn"
                    :title="t('security.firewall.moveDown')"
                    @click.stop="moveRule(idx, 1)"
                  >
                    ↓
                  </button>
                  <button
                    type="button"
                    class="fw-icon-btn"
                    :class="r.enabled ? 'text-slate-600' : 'text-green-700'"
                    :title="r.enabled ? t('security.firewall.disableRule') : t('security.firewall.enableRule')"
                    @click.stop="toggleEnabled(r)"
                  >
                    {{ r.enabled ? '○' : '●' }}
                  </button>
                  <button type="button" class="fw-icon-btn text-blue-600" @click.stop="startEdit(r)">
                    {{ t('common.edit') }}
                  </button>
                  <button type="button" class="fw-icon-btn text-violet-600" @click.stop="duplicateRule(r)">
                    {{ t('security.firewall.duplicateRule') }}
                  </button>
                  <button type="button" class="fw-icon-btn text-red-600" @click.stop="remove(r)">
                    {{ t('common.delete') }}
                  </button>
                </template>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <p v-if="ifaceReorderHint" class="text-xs text-amber-800 bg-amber-50 border-t border-amber-100 px-3 py-2">
        {{ t('security.firewall.ifaceReorderHint', { iface: activeIfaceLabel }) }}
      </p>
      <p class="text-xs text-slate-500 px-3 py-2 border-t bg-slate-50">
        {{ t('security.firewall.orderHint') }}
      </p>
    </div>

    <div v-if="selectedRule" class="card card-body text-sm border-l-4 border-l-slate-400">
      <div class="flex items-start justify-between gap-2">
        <h3 class="font-medium text-pfsense-nav">
          {{ t('security.firewall.ruleDetail') }}
          <span v-if="isRuleAutoManaged(selectedRule)" class="fw-auto-badge ml-2">{{
            t('security.firewall.autoRule')
          }}</span>
        </h3>
        <button type="button" class="text-slate-400 hover:text-slate-700 text-lg leading-none" @click="selectedRule = null">
          ×
        </button>
      </div>
      <dl class="mt-2 grid sm:grid-cols-2 gap-x-4 gap-y-1 text-xs">
        <template v-for="(line, li) in detailLines" :key="li">
          <dt class="text-slate-500">{{ line.k }}</dt>
          <dd class="font-mono text-slate-800 break-all">{{ line.v }}</dd>
        </template>
      </dl>
      <div v-if="selectedNftLine" class="mt-3 pt-3 border-t border-slate-200">
        <p class="text-xs text-slate-500 mb-1">{{ t('security.firewall.detailNftLine') }}</p>
        <pre class="text-xs font-mono bg-slate-900 text-slate-100 p-2 rounded overflow-x-auto whitespace-pre-wrap">{{ selectedNftLine }}</pre>
      </div>
      <p v-else-if="selectedNftHint" class="mt-3 pt-3 border-t border-slate-200 text-xs text-slate-500">
        {{ selectedNftHint }}
      </p>
      <p v-if="isRuleAutoManaged(selectedRule)" class="mt-2 text-xs text-amber-800">
        {{ t('security.firewall.ruleLocked') }}
      </p>
    </div>

    <!-- 添加/编辑表单 -->
    <div
      v-show="showForm"
      ref="formPanel"
      class="card card-body space-y-3 text-sm border-l-4 border-l-blue-500"
    >
      <h3 class="font-medium text-pfsense-nav">
        {{ editing ? t('security.firewall.editRule') : t('security.firewall.addRule') }}
        <span class="text-slate-400 font-normal">({{ form.chain }} · {{ activeIfaceLabel }})</span>
      </h3>
      <p v-if="formChainTabMismatch" class="text-xs text-amber-800 bg-amber-50 border border-amber-200 rounded px-2 py-1.5">
        {{
          t('security.firewall.formChainTabMismatch', {
            formChain: form.chain,
            tabChain: activeChain,
          })
        }}
      </p>
      <p v-if="formHint" class="text-xs text-amber-800 bg-amber-50 border border-amber-200 rounded px-2 py-1.5">
        {{ formHint }}
      </p>
      <p
        v-else-if="formWanDropWarning"
        class="text-xs text-amber-800 bg-amber-50 border border-amber-200 rounded px-2 py-1.5"
      >
        {{
          t('security.firewall.formWanDropHint', {
            iface: formResolvedIif,
            port: adminPort || '8080',
          })
        }}
      </p>
      <div class="grid sm:grid-cols-2 lg:grid-cols-3 gap-3">
        <div>
          <label class="text-xs text-slate-500">{{ t('security.firewall.chain') }}</label>
          <select v-model="form.chain" class="input-field mt-1">
            <option value="forward">{{ t('security.firewall.chainForward') }}</option>
            <option value="input">{{ t('security.firewall.chainInput') }}</option>
          </select>
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('security.firewall.action') }}</label>
          <select v-model="form.action" class="input-field mt-1">
            <option value="accept">{{ t('security.firewall.actionPass') }}</option>
            <option value="drop">{{ t('security.firewall.actionBlock') }}</option>
            <option value="reject">{{ t('security.firewall.actionReject') }}</option>
          </select>
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('security.firewall.inIface') }}</label>
          <select v-model="form.iif_mode" class="input-field mt-1">
            <option value="any">{{ t('security.firewall.optAny') }}</option>
            <option v-if="devLan" value="lan">{{ t('security.firewall.roleLan') }} ({{ devLan }})</option>
            <option v-if="devWan" value="wan">{{ t('security.firewall.roleWan') }} ({{ devWan }})</option>
            <option
              v-for="item in ifaceList.filter((x) => x.name !== devLan && x.name !== devWan)"
              :key="'iif-' + item.name"
              :value="'dev:' + item.name"
            >
              {{ item.name }}{{ item.label ? ` (${item.label})` : '' }} · {{ item.role }}
            </option>
            <option value="custom">{{ t('security.firewall.optCustomIface') }}</option>
          </select>
          <input
            v-if="form.iif_mode === 'custom'"
            v-model="form.iif_custom"
            class="input-field mt-1 font-mono text-xs"
            :placeholder="t('security.firewall.customIfacePh')"
          />
        </div>
        <div v-if="!isInputChain">
          <label class="text-xs text-slate-500">{{ t('security.firewall.outIface') }}</label>
          <select v-model="form.oif_mode" class="input-field mt-1">
            <option value="any">{{ t('security.firewall.optAny') }}</option>
            <option v-if="devLan" value="lan">{{ t('security.firewall.roleLan') }} ({{ devLan }})</option>
            <option v-if="devWan" value="wan">{{ t('security.firewall.roleWan') }} ({{ devWan }})</option>
            <option
              v-for="item in ifaceList.filter((x) => x.name !== devLan && x.name !== devWan)"
              :key="'oif-' + item.name"
              :value="'dev:' + item.name"
            >
              {{ item.name }}{{ item.label ? ` (${item.label})` : '' }} · {{ item.role }}
            </option>
            <option value="custom">{{ t('security.firewall.optCustomIface') }}</option>
          </select>
          <input
            v-if="form.oif_mode === 'custom'"
            v-model="form.oif_custom"
            class="input-field mt-1 font-mono text-xs"
            :placeholder="t('security.firewall.customIfacePh')"
          />
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('security.firewall.protocol') }}</label>
          <select v-model="form.proto" class="input-field mt-1">
            <option v-for="p in PROTO_OPTIONS" :key="p.value || 'any'" :value="p.value">
              {{ t(`security.firewall.${p.labelKey}`) }}
            </option>
          </select>
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('security.firewall.srcPort') }}</label>
          <select v-model="form.src_port_mode" class="input-field mt-1">
            <option value="any">{{ t('security.firewall.optAny') }}</option>
            <option value="custom">{{ t('security.firewall.optCustomPort') }}</option>
          </select>
          <input
            v-if="form.src_port_mode === 'custom'"
            v-model="form.src_port_custom"
            type="number"
            min="1"
            max="65535"
            class="input-field mt-1 font-mono"
          />
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('security.firewall.dstPort') }}</label>
          <select v-model="form.dst_port_mode" class="input-field mt-1">
            <option value="any">{{ t('security.firewall.optAny') }}</option>
            <option value="custom">{{ t('security.firewall.optCustomPort') }}</option>
          </select>
          <input
            v-if="form.dst_port_mode === 'custom'"
            v-model="form.dst_port_custom"
            type="number"
            min="1"
            max="65535"
            class="input-field mt-1 font-mono"
          />
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('security.firewall.srcAddr') }}</label>
          <select v-model="form.src_mode" class="input-field mt-1">
            <option value="any">{{ t('security.firewall.optAny') }}</option>
            <option value="cidr">{{ t('security.firewall.optCustomCidr') }}</option>
            <option value="alias">{{ t('security.firewall.optAlias') }}</option>
          </select>
          <input
            v-if="form.src_mode === 'cidr'"
            v-model="form.src_cidr"
            class="input-field mt-1 font-mono text-xs"
            placeholder="10.0.0.0/8"
          />
          <select
            v-if="form.src_mode === 'alias'"
            v-model="form.src_alias"
            class="input-field mt-1 font-mono text-xs"
          >
            <option value="">{{ t('security.firewall.pickAlias') }}</option>
            <option v-for="a in aliasNames" :key="'s-' + a" :value="a">{{ a }}</option>
          </select>
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('security.firewall.dstAddr') }}</label>
          <select v-model="form.dst_mode" class="input-field mt-1">
            <option value="any">{{ t('security.firewall.optAny') }}</option>
            <option value="cidr">{{ t('security.firewall.optCustomCidr') }}</option>
            <option value="alias">{{ t('security.firewall.optAlias') }}</option>
          </select>
          <input
            v-if="form.dst_mode === 'cidr'"
            v-model="form.dst_cidr"
            class="input-field mt-1 font-mono text-xs"
          />
          <select
            v-if="form.dst_mode === 'alias'"
            v-model="form.dst_alias"
            class="input-field mt-1 font-mono text-xs"
          >
            <option value="">{{ t('security.firewall.pickAlias') }}</option>
            <option v-for="a in aliasNames" :key="'d-' + a" :value="a">{{ a }}</option>
          </select>
        </div>
        <div class="sm:col-span-2 lg:col-span-3">
          <label class="text-xs text-slate-500">{{ t('security.firewall.colDescription') }}</label>
          <input v-model="form.comment" class="input-field mt-1" />
        </div>
      </div>
      <label class="flex items-center gap-2">
        <input v-model="form.enabled" type="checkbox" />
        {{ t('security.firewall.enabled') }}
      </label>
      <div class="flex flex-wrap gap-2">
        <button v-if="!editing" type="button" class="btn-primary" @click="add">
          {{ t('security.firewall.addApply') }}
        </button>
        <template v-else>
          <button type="button" class="btn-primary" @click="saveEdit">{{ t('security.firewall.saveEdit') }}</button>
          <button type="button" class="btn-secondary" @click="cancelEdit">{{ t('common.cancel') }}</button>
        </template>
        <button
          type="button"
          class="btn-secondary"
          :disabled="previewLoading"
          @click="previewNft"
        >
          {{ previewLoading ? t('security.firewall.previewLoading') : t('security.firewall.previewNft') }}
        </button>
      </div>
      <pre
        v-if="previewLine"
        class="text-xs bg-slate-900 text-emerald-200 p-3 rounded-lg overflow-auto"
      >{{ previewLine }}</pre>
    </div>

    <button type="button" class="text-sm text-slate-600 hover:text-slate-900" @click="showRendered = !showRendered">
      {{ showRendered ? t('security.firewall.hideNft') : t('security.firewall.showNft') }}
    </button>
    <pre v-if="showRendered" class="text-xs bg-slate-900 text-slate-100 p-4 rounded-lg overflow-auto max-h-96">{{
      rendered
    }}</pre>
  </div>
</template>

<style scoped>
.fw-iface-tabs {
  @apply overflow-x-auto;
}

.fw-iface-tab {
  @apply inline-flex items-center gap-1 px-3 py-1.5 rounded-md text-xs font-medium text-slate-600
    border border-transparent hover:bg-slate-100 transition-colors;
}

.fw-iface-tab-active {
  @apply bg-white border-slate-300 text-pfsense-nav shadow-sm;
}

.fw-iface-tab-lan.fw-iface-tab-active {
  @apply border-emerald-300 bg-emerald-50 text-emerald-900;
}

.fw-iface-tab-wan.fw-iface-tab-active {
  @apply border-sky-300 bg-sky-50 text-sky-900;
}

.fw-iface-tab-role {
  @apply text-[9px] font-bold uppercase px-1 rounded bg-slate-200 text-slate-600;
}

.fw-iface-tab-count {
  @apply ml-0.5 text-[10px] opacity-70 tabular-nums;
}

.fw-rules-table thead th {
  @apply sticky top-0 z-10 whitespace-nowrap;
}

.fw-action-badge {
  @apply inline-block px-2 py-0.5 rounded text-xs font-semibold uppercase tracking-wide;
}

.fw-action-pass {
  @apply bg-green-100 text-green-800 border border-green-200;
}

.fw-action-block {
  @apply bg-red-100 text-red-800 border border-red-200;
}

.fw-action-reject {
  @apply bg-amber-100 text-amber-900 border border-amber-200;
}

.fw-row-system {
  @apply bg-slate-50/80;
}

.fw-row-system td {
  @apply border-b border-slate-100;
}

.fw-row-custom:hover {
  @apply bg-slate-50;
}

.fw-iface-cell {
  @apply inline-flex items-center gap-1;
}

.fw-mini-tag {
  @apply text-[10px] font-bold px-1 rounded bg-slate-200 text-slate-700;
}

.fw-icon-btn {
  @apply text-xs px-1 py-0.5 rounded hover:bg-slate-100;
}

.fw-col-actions {
  @apply sticky right-0 z-[5] bg-white shadow-[-4px_0_6px_-4px_rgba(0,0,0,0.08)];
}

.fw-row-custom:hover .fw-col-actions {
  @apply bg-slate-50;
}

.fw-section-row td {
  @apply border-b border-slate-200;
}

.fw-row-auto {
  @apply bg-amber-50/30;
}

.fw-auto-badge {
  @apply inline-block mr-1 px-1 py-0 rounded text-[10px] font-bold uppercase bg-amber-200 text-amber-900;
}

.fw-status-on {
  @apply bg-green-100 text-green-800;
}

.fw-status-off {
  @apply bg-slate-200 text-slate-600;
}
</style>
