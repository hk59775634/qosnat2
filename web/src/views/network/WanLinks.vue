<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { api } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'
import PageTabs from '@/components/PageTabs.vue'

const { t } = useI18n()
const links = ref([])
const egress = ref([])
const aliases = ref([])
const googleIpv4Url = ref('')
const resolved = ref([])
const cloudflareCIDRs = ref([])
const warpStatusDefaults = {
  installed: false,
  enabled: false,
  service_up: false,
  connected: false,
  netns_healthy: false,
  interface: '',
  root: false,
  status_raw: '',
  exit_info: null,
  warp_license_key: '',
  warp_license_key_set: false,
}
const warpStatus = ref({ ...warpStatusDefaults })
const installingWarp = ref(false)
const warpInstallJob = ref(null)
const warpInstallPoll = ref(null)
const warpInstallPollErrs = ref(0)
const warpStatusPoll = ref(null)
const warpTaskJob = ref(null)
const warpTaskPoll = ref(null)
const warpTaskPollErrs = ref(0)
const warpConnecting = ref(false)
const warpDisconnecting = ref(false)
const warpConnectResult = ref(null)
const warpLicenseKey = ref('')
const warpLicenseSaved = ref('')
const warpLicenseSaving = ref(false)
const warpLicenseDeleting = ref(false)
const warpLicenseApplying = ref(false)
const WARP_ACTION_LOCK_MS = 4000
const warpActionLocked = ref(false)
let warpActionLockTimer = null
const devWan = ref('')
const err = ref('')
const ok = ref('')
const form = ref({
  name: 'WAN2',
  device: '',
  gateway: '',
  metric: 200,
  tier: 2,
  weight: 1,
  enabled: true,
})
const egForm = ref({
  name: 'US exit',
  src_mode: 'cidr',
  src_cidr: '10.250.0.0/24',
  src_alias: '',
  dst_mode: 'none',
  dst_cidr: '',
  dst_alias: '',
  wan_link_id: '',
  snat_ip: '',
  priority: 100,
  enabled: true,
})
const editingId = ref(null)
const editingEgressId = ref(null)
const egEditForm = ref({
  name: '',
  src_mode: 'cidr',
  src_cidr: '',
  src_alias: '',
  dst_mode: 'none',
  dst_cidr: '',
  dst_alias: '',
  wan_link_id: '',
  snat_ip: '',
  priority: 100,
  enabled: true,
})
const editForm = ref({
  name: '',
  device: '',
  gateway: '',
  metric: 200,
  tier: 2,
  weight: 1,
  enabled: true,
})

const linkOptions = computed(() =>
  (links.value || []).filter((w) => w.enabled).map((w) => ({ id: w.id, label: `${w.name} (${w.device})` }))
)
const warpInstallRunning = computed(() => installingWarp.value || warpInstallJob.value?.state === 'running')
const warpTaskRunning = computed(
  () => warpTaskJob.value?.state === 'running' || warpConnecting.value || warpDisconnecting.value
)

const warpUiConnected = computed(() => {
  const s = warpStatus.value
  if (s.connected) return true
  const raw = String(s.status_raw || '').toLowerCase()
  if (raw.includes('status update: connected')) return true
  return !!(
    s.service_up &&
    s.netns_healthy &&
    raw.includes('connected') &&
    !raw.includes('disconnected') &&
    !raw.includes('unable to connect')
  )
})

const warpEnabled = computed(() => !!warpStatus.value.enabled)

const warpExitInfo = computed(() => warpStatus.value?.exit_info || null)

const activeTab = ref('wan')
const wanTabs = computed(() => [
  { id: 'wan', label: t('network.wanLinks.tabWan') },
  { id: 'warp', label: t('network.wanLinks.tabWarp') },
  { id: 'egress', label: t('network.wanLinks.tabEgress') },
])

const warpExitLine = computed(() => {
  const e = warpExitInfo.value
  if (!e) return ''
  if (e.ip) {
    const loc = [e.city, e.region, e.country].filter(Boolean).join(', ')
    return loc ? `${e.ip} · ${loc}` : e.ip
  }
  if (e.error) return e.error
  return ''
})

function warpTierLabel(tier, rawWarp) {
  const key = String(tier || rawWarp || '').toLowerCase()
  if (key === 'off') return t('network.wanLinks.warpTierOff')
  if (key === 'standard' || key === 'on') return t('network.wanLinks.warpTierStandard')
  if (key === 'plus') return t('network.wanLinks.warpTierPlus')
  if (key === '2xc' || key === '2x') return t('network.wanLinks.warpTier2xc')
  if (key) return t('network.wanLinks.warpTierUnknown', { tier: rawWarp || tier })
  return ''
}

const warpServiceLine = computed(() => {
  const e = warpExitInfo.value
  if (!e) return ''
  const tier = warpTierLabel(e.warp_tier, e.warp)
  const parts = []
  if (tier) parts.push(tier)
  if (e.account_type) parts.push(`${t('network.wanLinks.warpAccountType')}: ${e.account_type}`)
  return parts.join(' · ')
})

const warpLicenseKeySet = computed(() => !!warpStatus.value?.warp_license_key_set)

const warpLicenseDirty = computed(() => warpLicenseKey.value.trim() !== '')

const warpLicenseStatusText = computed(() => {
  const typed = warpLicenseKey.value.trim()
  if (typed) return typed
  if (warpLicenseKeySet.value && !warpLicenseDirty.value) return ''
  return ''
})

const warpCanApplyLicense = computed(
  () =>
    warpStatus.value?.root &&
    warpStatus.value?.installed &&
    warpUiConnected.value &&
    warpStatus.value?.netns_healthy &&
    warpLicenseKeySet.value &&
    !warpLicenseDirty.value
)

function formatWarpExitCheckedAt(iso) {
  if (!iso) return ''
  try {
    return new Date(iso).toLocaleString()
  } catch {
    return iso
  }
}

const warpExitCheckedAt = computed(() => formatWarpExitCheckedAt(warpExitInfo.value?.fetched_at))

const warpActiveJob = computed(() => {
  if (warpTaskJob.value?.state === 'running' || warpTaskJob.value?.state === 'failed') {
    return warpTaskJob.value
  }
  if (warpConnecting.value) {
    return { op: 'connect', state: 'running', message: '' }
  }
  if (warpDisconnecting.value) {
    return { op: 'disconnect', state: 'running', message: '' }
  }
  return null
})

const warpTaskPanelVisible = computed(() => {
  const j = warpActiveJob.value
  if (j?.state === 'running' || j?.state === 'failed') return true
  return !!warpConnectResult.value?.diagnostics
})

const warpTaskStatusLine = computed(() => {
  const r = warpConnectResult.value
  if (r?.netns_status) return r.netns_status
  if (r?.diagnostics?.netns_warp_status) return r.diagnostics.netns_warp_status
  const health = warpTaskJob.value?.result?.health
  if (health?.netns_status) return health.netns_status
  return ''
})

const warpTaskDiagnostics = computed(
  () => warpConnectResult.value?.diagnostics || warpTaskJob.value?.result?.diagnostics || null
)

function warpTaskOpLabel(op) {
  if (op === 'connect') return t('network.wanLinks.warpTaskOpConnect')
  if (op === 'disconnect') return t('network.wanLinks.warpTaskOpDisconnect')
  return op || '—'
}

function resolvedRow(policyId) {
  return resolved.value.find((r) => r.policy?.id === policyId)
}

function normalizeWarpTask(job) {
  if (!job || job.state === 'idle') return null
  return job
}

function applyConnectTaskResult(result) {
  if (!result) return
  const health = result.health || null
  warpConnectResult.value = health
  warpStatus.value = {
    ...warpStatus.value,
    installed: true,
    connected: !!health?.connected,
    service_up: !!health?.service_running,
    netns_healthy: true,
    interface: result.interface || warpStatus.value.interface,
    status_raw: health?.netns_status || warpStatus.value.status_raw,
  }
}

function syncWarpLicenseFromStatus(ws) {
  if (!warpLicenseDirty.value) {
    warpLicenseKey.value = ''
  }
  warpLicenseSaved.value = ws?.warp_license_key_set ? 'configured' : ''
}

function applyWarpStatus(ws) {
  if (!ws) return
  const merged = { ...warpStatusDefaults, ...ws }
  if (warpDisconnecting.value && merged.enabled) {
    merged.enabled = false
  }
  warpStatus.value = merged
  syncWarpLicenseFromStatus(merged)
  warpInstallJob.value = normalizeWarpJob(ws.install_job)
  installingWarp.value = ws.install_job?.state === 'running'
  if (installingWarp.value && !warpInstallPoll.value) {
    startWarpInstallPoll()
  }
  const task = normalizeWarpTask(ws.task)
  warpTaskJob.value = task
  if (task?.state === 'running' && !warpTaskPoll.value) {
    if (task.op === 'connect') warpConnecting.value = true
    if (task.op === 'disconnect') warpDisconnecting.value = true
    startWarpTaskPoll()
  }
}

async function refreshWarpStatus() {
  const ws = await api.network.warp.status()
  applyWarpStatus(ws)
  return ws
}

async function load() {
  err.value = ''
  try {
    const [wan, eg, ws] = await Promise.all([
      api.network.wanLinks.list(),
      api.network.egressPolicies.list(),
      api.network.warp.status(),
    ])
    links.value = wan?.wan_links || []
    devWan.value = wan?.dev_wan || ''
    egress.value = eg?.egress_policies || []
    aliases.value = eg?.aliases || []
    googleIpv4Url.value = eg?.google_ipv4_url || 'https://www.gstatic.com/ipranges/goog_ipv4_only.txt'
    resolved.value = eg?.resolved || []
    cloudflareCIDRs.value = eg?.cloudflare_cdn_cidrs_ipv4 || []
    applyWarpStatus(ws)
    if (!form.value.device && devWan.value) form.value.device = devWan.value
    if (!egForm.value.wan_link_id && links.value.length) {
      const pick =
        links.value.find((w) => w.enabled && w.device === devWan.value) ||
        links.value.find((w) => w.enabled) ||
        links.value[0]
      if (pick) egForm.value.wan_link_id = pick.id
    }
  } catch (e) {
    err.value = e?.message || String(e)
  }
}

function normalizeWarpJob(job) {
  if (!job || job.state === 'idle' || job.state === 'ok') return null
  return job
}

function stopWarpInstallPoll() {
  if (warpInstallPoll.value) {
    clearInterval(warpInstallPoll.value)
    warpInstallPoll.value = null
  }
}

function stopWarpStatusPoll() {
  if (warpStatusPoll.value) {
    clearInterval(warpStatusPoll.value)
    warpStatusPoll.value = null
  }
}

function lockWarpButtons() {
  warpActionLocked.value = true
  if (warpActionLockTimer) clearTimeout(warpActionLockTimer)
  warpActionLockTimer = setTimeout(() => {
    warpActionLocked.value = false
    warpActionLockTimer = null
  }, WARP_ACTION_LOCK_MS)
}

function startWarpStatusPoll() {
  stopWarpStatusPoll()
  warpStatusPoll.value = setInterval(async () => {
    try {
      await refreshWarpStatus()
    } catch {
      /* 轮询失败不打断页面 */
    }
  }, 4000)
}

function stopWarpTaskPoll() {
  if (warpTaskPoll.value) {
    clearInterval(warpTaskPoll.value)
    warpTaskPoll.value = null
  }
}

function startWarpTaskPoll() {
  stopWarpTaskPoll()
  warpTaskPollErrs.value = 0
  const pollStart = Date.now()
  const pollMaxMs = 120000
  warpTaskPoll.value = setInterval(async () => {
    try {
      const j = await api.network.warp.taskStatus()
      warpTaskPollErrs.value = 0
      warpTaskJob.value = normalizeWarpTask(j) || j
      if (j.state === 'running' && Date.now() - pollStart > pollMaxMs) {
        stopWarpTaskPoll()
        warpConnecting.value = false
        warpDisconnecting.value = false
        err.value = t('network.wanLinks.warpTaskTimedOut')
        await refreshWarpStatus()
        return
      }
      if (j.state === 'ok') {
        stopWarpTaskPoll()
        warpConnecting.value = false
        warpDisconnecting.value = false
        if (j.op === 'connect') {
          applyConnectTaskResult(j.result)
          ok.value = t('network.wanLinks.warpConnected')
        } else {
          ok.value = t('network.wanLinks.warpDisconnected')
        }
        warpConnectResult.value = null
        warpTaskJob.value = null
        await load()
        if (j.op === 'connect') {
          const warpLink = links.value.find((w) => w.warp_managed)
          if (warpLink) egForm.value.wan_link_id = warpLink.id
        }
      } else if (j.state === 'failed') {
        stopWarpTaskPoll()
        warpConnecting.value = false
        warpDisconnecting.value = false
        err.value = j.message || t('network.wanLinks.warpTaskFailed')
        if (j.op === 'connect' && j.result?.diagnostics) {
          warpConnectResult.value = { diagnostics: j.result.diagnostics }
        }
        warpTaskJob.value = j
      }
    } catch {
      warpTaskPollErrs.value += 1
      if (warpTaskPollErrs.value >= 3) {
        stopWarpTaskPoll()
        warpConnecting.value = false
        warpDisconnecting.value = false
        err.value = t('network.wanLinks.warpTaskStatusLost')
      }
    }
  }, 2000)
}

function startWarpInstallPoll() {
  stopWarpInstallPoll()
  warpInstallPollErrs.value = 0
  warpInstallPoll.value = setInterval(async () => {
    try {
      const j = await api.network.warp.installStatus()
      warpInstallPollErrs.value = 0
      warpInstallJob.value = normalizeWarpJob(j)
      if (j.state === 'ok') {
        stopWarpInstallPoll()
        installingWarp.value = false
        warpInstallJob.value = null
        ok.value = t('network.wanLinks.warpInstalled')
        await load()
      } else if (j.state === 'failed') {
        stopWarpInstallPoll()
        installingWarp.value = false
        err.value = j.message || t('network.wanLinks.warpInstallFailed')
      }
    } catch {
      warpInstallPollErrs.value += 1
      if (warpInstallPollErrs.value >= 3) {
        stopWarpInstallPoll()
        installingWarp.value = false
        err.value = t('network.wanLinks.warpInstallStatusLost')
      }
    }
  }, 3000)
}

async function installWarp() {
  lockWarpButtons()
  err.value = ''
  ok.value = ''
  try {
    installingWarp.value = true
    const r = await api.network.warp.install()
    const state = r?.job?.state || ''
    if (state === 'ok') {
      installingWarp.value = false
      warpInstallJob.value = null
      ok.value = r.message || t('network.wanLinks.warpInstalled')
      await load()
      return
    }
    ok.value = r.message || t('network.wanLinks.warpInstalling')
    warpInstallJob.value = r.job || { state: 'running' }
    startWarpInstallPoll()
  } catch (e) {
    installingWarp.value = false
    err.value = e.message
  }
}

async function deleteWarpLicenseKey() {
  if (!warpLicenseKeySet.value && !warpLicenseKey.value.trim()) return
  if (!window.confirm(t('network.wanLinks.warpLicenseKeyDeleteConfirm'))) return
  err.value = ''
  ok.value = ''
  warpLicenseDeleting.value = true
  try {
    const r = await api.network.warp.deleteLicense()
    warpLicenseKey.value = ''
    warpLicenseSaved.value = ''
    warpStatus.value = {
      ...warpStatus.value,
      enabled: false,
      connected: false,
      warp_license_key: '',
      warp_license_key_set: false,
    }
    ok.value = r.message || t('network.wanLinks.warpLicenseKeyDeleted')
    await refreshWarpStatus()
  } catch (e) {
    err.value = e.message
  } finally {
    warpLicenseDeleting.value = false
  }
}

async function saveWarpLicenseKey() {
  err.value = ''
  ok.value = ''
  warpLicenseSaving.value = true
  try {
    const r = await api.network.warp.saveLicense({ license_key: warpLicenseKey.value.trim() })
    warpLicenseKey.value = ''
    warpLicenseSaved.value = r.warp_license_key_set ? 'configured' : ''
    warpStatus.value = {
      ...warpStatus.value,
      warp_license_key_set: !!r.warp_license_key_set,
    }
    ok.value = t('network.wanLinks.warpLicenseKeySaved')
  } catch (e) {
    err.value = e.message
  } finally {
    warpLicenseSaving.value = false
  }
}

async function applyWarpLicenseKey() {
  if (!warpCanApplyLicense.value) return
  err.value = ''
  ok.value = ''
  warpLicenseApplying.value = true
  try {
    const r = await api.network.warp.applyLicense()
    if (r.exit_info) {
      warpStatus.value = { ...warpStatus.value, exit_info: r.exit_info }
    }
    ok.value = r.message || t('network.wanLinks.warpLicenseKeyApplied')
    await refreshWarpStatus()
  } catch (e) {
    err.value = e.message
  } finally {
    warpLicenseApplying.value = false
  }
}

async function connectWarp() {
  lockWarpButtons()
  err.value = ''
  ok.value = ''
  warpConnectResult.value = null
  if (warpLicenseDirty.value) {
    err.value = t('network.wanLinks.warpLicenseKeyUnsaved')
    return
  }
  warpConnecting.value = true
  warpStatus.value = { ...warpStatus.value, enabled: true }
  try {
    const r = await api.network.warp.connect()
    const job = r?.job || {}
    if (job.state === 'ok' && r?.result?.health) {
      applyConnectTaskResult(r.result)
      ok.value = t('network.wanLinks.warpConnected')
      await load()
      return
    }
    ok.value = r?.message || t('network.wanLinks.warpConnectPending')
    warpTaskJob.value = job.state ? job : { state: 'running', op: 'connect' }
    startWarpTaskPoll()
  } catch (e) {
    warpConnectResult.value = e?.data?.diagnostics ? { diagnostics: e.data.diagnostics } : null
    err.value = e.message
    warpConnecting.value = false
  }
}

async function disconnectWarp() {
  lockWarpButtons()
  err.value = ''
  ok.value = ''
  warpDisconnecting.value = true
  warpStatus.value = { ...warpStatus.value, enabled: false }
  try {
    const r = await api.network.warp.disconnect()
    const job = r?.job || {}
    if (job.state === 'ok') {
      warpConnectResult.value = null
      ok.value = t('network.wanLinks.warpDisconnected')
      await load()
      warpDisconnecting.value = false
      return
    }
    ok.value = r?.message || t('network.wanLinks.warpDisconnectPending')
    warpTaskJob.value = job.state ? job : { state: 'running', op: 'disconnect' }
    startWarpTaskPoll()
  } catch (e) {
    err.value = e.message
    warpDisconnecting.value = false
  }
}

async function add() {
  err.value = ''
  try {
    await api.network.wanLinks.add({ ...form.value })
    ok.value = t('common.saved')
    await load()
  } catch (e) {
    err.value = e.message
  }
}

function startEdit(w) {
  editingId.value = w.id
  editForm.value = {
    name: w.name,
    device: w.device,
    gateway: w.gateway,
    metric: w.metric,
    tier: w.tier,
    weight: w.weight,
    enabled: w.enabled,
  }
}

function cancelEdit() {
  editingId.value = null
}

async function saveEdit() {
  if (!editingId.value) return
  err.value = ''
  try {
    await api.network.wanLinks.put(editingId.value, { ...editForm.value })
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
    await api.network.wanLinks.del(id)
    if (editingId.value === id) editingId.value = null
    await load()
  } catch (e) {
    err.value = e.message
  }
}

async function addEgress() {
  err.value = ''
  try {
    const body = buildEgressBody(egForm.value)
    await api.network.egressPolicies.add(body)
    ok.value = t('common.saved')
    await load()
  } catch (e) {
    err.value = e.message
  }
}

function buildEgressBody(f) {
  const body = {
    name: f.name,
    wan_link_id: f.wan_link_id,
    priority: f.priority,
    enabled: f.enabled,
  }
  if (f.snat_ip) body.snat_ip = f.snat_ip
  if (f.src_mode === 'cidr' && f.src_cidr) body.src_cidr = f.src_cidr.trim()
  if (f.src_mode === 'alias' && f.src_alias) body.src_alias = f.src_alias
  if (f.dst_mode === 'cidr' && f.dst_cidr) body.dst_cidr = f.dst_cidr.trim()
  if (f.dst_mode === 'alias' && f.dst_alias) body.dst_alias = f.dst_alias
  return body
}

function egressEndpointsLabel(p) {
  const parts = []
  if (p.src_alias) parts.push(`${t('network.wanLinks.srcShort')}:@${p.src_alias}`)
  else if (p.src_cidr) parts.push(`${t('network.wanLinks.srcShort')}:${p.src_cidr}`)
  else if (p.cidr && p.match !== 'destination') parts.push(`${t('network.wanLinks.srcShort')}:${p.cidr}`)
  if (p.dst_alias) parts.push(`${t('network.wanLinks.dstShort')}:@${p.dst_alias}`)
  else if (p.dst_cidr) parts.push(`${t('network.wanLinks.dstShort')}:${p.dst_cidr}`)
  else if (p.cidr && (p.match === 'destination' || (!p.src_cidr && !p.src_alias))) {
    parts.push(`${t('network.wanLinks.dstShort')}:${p.cidr}`)
  }
  return parts.join(' · ') || '—'
}

function policyToEditForm(p) {
  let src_mode = 'none'
  let dst_mode = 'none'
  let src_cidr = ''
  let src_alias = ''
  let dst_cidr = ''
  let dst_alias = ''
  if (p.src_alias) {
    src_mode = 'alias'
    src_alias = p.src_alias
  } else if (p.src_cidr) {
    src_mode = 'cidr'
    src_cidr = p.src_cidr
  } else if (p.cidr && p.match !== 'destination') {
    src_mode = 'cidr'
    src_cidr = p.cidr
  }
  if (p.dst_alias) {
    dst_mode = 'alias'
    dst_alias = p.dst_alias
  } else if (p.dst_cidr) {
    dst_mode = 'cidr'
    dst_cidr = p.dst_cidr
  } else if (p.cidr && p.match === 'destination') {
    dst_mode = 'cidr'
    dst_cidr = p.cidr
  }
  return {
    name: p.name || '',
    src_mode,
    src_cidr,
    src_alias,
    dst_mode,
    dst_cidr,
    dst_alias,
    wan_link_id: p.wan_link_id,
    snat_ip: p.snat_ip || '',
    priority: p.priority,
    enabled: p.enabled,
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
      priority: egForm.value.priority || 100,
      enabled: true,
    })
    ok.value = t('network.wanLinks.googlePresetOk')
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
    err.value = 'Cloudflare CDN 列表为空'
    return
  }
  const policies = prefixes.map((cidr) => ({
    name: `Cloudflare CDN ${cidr}`,
    cidr,
    match: 'destination',
    wan_link_id: egForm.value.wan_link_id,
    snat_ip: egForm.value.snat_ip || undefined,
    priority: egForm.value.priority || 100,
    enabled: true,
  }))
  try {
    const res = await api.network.egressPolicies.bulkAdd(policies, true)
    ok.value = `Cloudflare CDN 策略已导入 ${res.added || 0} 条（跳过 ${res.skipped || 0} 条已存在）`
    await load()
  } catch (e) {
    err.value = e.message
  }
}

function startEditEgress(p) {
  editingEgressId.value = p.id
  egEditForm.value = policyToEditForm(p)
}

function cancelEditEgress() {
  editingEgressId.value = null
}

async function saveEditEgress() {
  if (!editingEgressId.value) return
  err.value = ''
  try {
    const body = buildEgressBody(egEditForm.value)
    await api.network.egressPolicies.put(editingEgressId.value, body)
    editingEgressId.value = null
    ok.value = t('common.saved')
    await load()
  } catch (e) {
    err.value = e.message
  }
}

async function removeEgress(id) {
  if (!confirm(t('common.delete') + '?')) return
  err.value = ''
  try {
    await api.network.egressPolicies.del(id)
    if (editingEgressId.value === id) editingEgressId.value = null
    await load()
  } catch (e) {
    err.value = e.message
  }
}

onMounted(async () => {
  await load()
  startWarpStatusPoll()
})
onUnmounted(() => {
  stopWarpInstallPoll()
  stopWarpStatusPoll()
  stopWarpTaskPoll()
  if (warpActionLockTimer) clearTimeout(warpActionLockTimer)
})
</script>

<template>
  <div class="page-stack">
    <PageHeader
      :title="t('network.wanLinks.title')"
      :description="t('network.wanLinks.description')"
      :ok="ok"
      :err="err"
    />
    <PageTabs v-model="activeTab" :tabs="wanTabs" />

    <div v-show="activeTab === 'wan'" class="card card-body mb-0 space-y-3 text-sm">
      <h3 class="font-medium text-slate-800">{{ t('network.wanLinks.tabAddWan') }}</h3>
      <div class="grid sm:grid-cols-2 gap-3">
        <div>
          <label class="text-xs text-slate-500">{{ t('common.name') }}</label>
          <input v-model="form.name" class="input-field mt-1" />
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('network.wanLinks.iface') }}</label>
          <input v-model="form.device" class="input-field mt-1 font-mono" />
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('network.wanLinks.gateway') }}</label>
          <input v-model="form.gateway" class="input-field mt-1 font-mono" />
        </div>
        <div>
          <label class="text-xs text-slate-500">Metric</label>
          <input v-model.number="form.metric" type="number" class="input-field mt-1" />
        </div>
        <div>
          <label class="text-xs text-slate-500">Tier</label>
          <input v-model.number="form.tier" type="number" class="input-field mt-1" />
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('network.wanLinks.weight') }}</label>
          <input v-model.number="form.weight" type="number" class="input-field mt-1" />
        </div>
        <label class="flex items-center gap-2 sm:col-span-2">
          <input v-model="form.enabled" type="checkbox" /> {{ t('common.enabled') }}
        </label>
      </div>
      <button type="button" class="btn-primary" @click="add">{{ t('common.add') }}</button>
    </div>

    <div v-show="activeTab === 'wan'" class="table-wrap card">
      <table class="data w-full text-sm">
        <thead>
          <tr>
            <th>{{ t('common.name') }}</th>
            <th>{{ t('network.wanLinks.iface') }}</th>
            <th>{{ t('network.wanLinks.gateway') }}</th>
            <th>Tier</th>
            <th>Metric</th>
            <th>{{ t('network.wanLinks.weight') }}</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="w in links" :key="w.id" :class="editingId === w.id ? 'bg-slate-50' : ''">
            <template v-if="editingId === w.id">
              <td><input v-model="editForm.name" class="input-field text-xs" /></td>
              <td><input v-model="editForm.device" class="input-field text-xs font-mono" /></td>
              <td><input v-model="editForm.gateway" class="input-field text-xs font-mono" /></td>
              <td><input v-model.number="editForm.tier" type="number" class="input-field text-xs w-16" /></td>
              <td><input v-model.number="editForm.metric" type="number" class="input-field text-xs w-16" /></td>
              <td><input v-model.number="editForm.weight" type="number" class="input-field text-xs w-16" /></td>
              <td class="space-x-2 whitespace-nowrap">
                <label class="inline-flex items-center gap-1 text-xs">
                  <input v-model="editForm.enabled" type="checkbox" /> {{ t('common.enabled') }}
                </label>
                <button type="button" class="text-indigo-600 text-xs" @click="saveEdit">{{ t('common.save') }}</button>
                <button type="button" class="text-slate-500 text-xs" @click="cancelEdit">{{ t('common.cancel') }}</button>
              </td>
            </template>
            <template v-else>
              <td>
                {{ w.name }}
                <span v-if="w.warp_managed" class="ml-1 text-[10px] px-1 py-0.5 rounded bg-violet-100 text-violet-800">WARP</span>
              </td>
              <td class="font-mono">{{ w.device }}</td>
              <td class="font-mono">
                {{ w.gateway }}
                <span v-if="w.policy_only" class="ml-1 text-[10px] px-1 py-0.5 rounded bg-indigo-100 text-indigo-700">
                  policy-only
                </span>
              </td>
              <td>{{ w.tier }}</td>
              <td>{{ w.metric }}</td>
              <td>{{ w.weight }}</td>
              <td class="space-x-2 whitespace-nowrap">
                <button
                  v-if="!w.warp_managed"
                  type="button"
                  class="text-indigo-600 text-xs"
                  @click="startEdit(w)"
                >{{ t('common.edit') }}</button>
                <button
                  v-if="!w.warp_managed"
                  type="button"
                  class="text-red-600 text-xs"
                  @click="remove(w.id)"
                >{{ t('common.delete') }}</button>
                <span v-else class="text-slate-400 text-xs">{{ t('network.wanLinks.warpManagedNoDelete') }}</span>
              </td>
            </template>
          </tr>
          <tr v-if="!links.length">
            <td colspan="7" class="text-center text-slate-400 py-3">{{ t('network.wanLinks.noExtra') }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-show="activeTab === 'warp'" class="card card-body mb-0 space-y-3 text-sm">
      <div>
        <h3 class="font-medium text-slate-800">{{ t('network.wanLinks.warpTitle') }}</h3>
        <p class="text-xs text-slate-500 mt-1">{{ t('network.wanLinks.warpHint') }}</p>
      </div>
      <div class="text-xs text-slate-600 rounded bg-slate-50 p-2 space-y-1">
        <div>
          {{ t('network.wanLinks.warpState') }}:
          {{ warpStatus.installed ? t('network.wanLinks.warpInstalledLabel') : t('network.wanLinks.warpNotInstalledLabel') }}
          · {{ warpEnabled ? t('network.wanLinks.warpEnabledLabel') : t('network.wanLinks.warpDisabledLabel') }}
          · {{ warpUiConnected ? t('network.wanLinks.warpTunnelUp') : t('network.wanLinks.warpTunnelDown') }}
          <span v-if="warpStatus.netns_healthy" class="text-slate-500"> · netns OK</span>
          <span v-if="warpStatus.interface" class="font-mono"> · {{ warpStatus.interface }}</span>
        </div>
        <div v-if="warpEnabled && warpUiConnected && warpServiceLine">
          {{ t('network.wanLinks.warpTierLabel') }}: {{ warpServiceLine }}
        </div>
        <div>
          {{ t('network.wanLinks.warpLicenseKeyStatusLabel') }}:
          <span v-if="warpLicenseStatusText" class="font-mono break-all">{{ warpLicenseStatusText }}</span>
          <span v-else-if="warpLicenseKeySet && !warpLicenseDirty" class="text-emerald-700">{{ t('network.wanLinks.warpLicenseKeyConfigured') }}</span>
          <span v-else class="text-slate-400">{{ t('network.wanLinks.warpLicenseKeyNotSet') }}</span>
          <span v-if="warpLicenseDirty && warpLicenseStatusText" class="text-amber-700 ml-1">
            ({{ t('network.wanLinks.warpLicenseKeyUnsavedShort') }})
          </span>
        </div>
      </div>
      <div class="grid sm:grid-cols-2 gap-3">
        <div class="sm:col-span-2">
          <label class="text-xs text-slate-500">{{ t('network.wanLinks.warpLicenseKey') }}</label>
          <input
            v-model="warpLicenseKey"
            type="text"
            autocomplete="off"
            spellcheck="false"
            class="input-field mt-1 font-mono text-xs"
            :placeholder="t('network.wanLinks.warpLicenseKeyPlaceholder')"
          />
          <p class="text-[11px] text-slate-500 mt-1">{{ t('network.wanLinks.warpLicenseKeyHint') }}</p>
          <p v-if="warpEnabled && warpUiConnected" class="text-[11px] text-amber-700 mt-1">
            {{ t('network.wanLinks.warpLicenseKeyApplyHint') }}
          </p>
          <p v-else-if="warpLicenseKeySet && !warpLicenseDirty" class="text-[11px] text-emerald-700 mt-1">
            {{ t('network.wanLinks.warpLicenseKeyConfigured') }}
          </p>
          <p v-else-if="warpLicenseDirty" class="text-[11px] text-amber-700 mt-1">
            {{ t('network.wanLinks.warpLicenseKeyUnsavedHint') }}
          </p>
          <div class="mt-2 flex flex-wrap gap-2">
            <button
              type="button"
              class="btn-secondary"
              :disabled="warpLicenseSaving || warpLicenseDeleting || warpTaskRunning"
              @click="saveWarpLicenseKey"
            >
              {{ warpLicenseSaving ? t('network.wanLinks.warpLicenseKeySaving') : t('network.wanLinks.warpLicenseKeySave') }}
            </button>
            <button
              type="button"
              class="btn-secondary"
              :disabled="warpLicenseSaving || warpLicenseDeleting || warpLicenseApplying || warpTaskRunning || !warpCanApplyLicense"
              :title="!warpCanApplyLicense ? t('network.wanLinks.warpLicenseKeyApplyDisabledHint') : ''"
              @click="applyWarpLicenseKey"
            >
              {{ warpLicenseApplying ? t('network.wanLinks.warpLicenseKeyApplying') : t('network.wanLinks.warpLicenseKeyApply') }}
            </button>
            <button
              type="button"
              class="btn-secondary text-red-700 border-red-200"
              :disabled="warpLicenseSaving || warpLicenseDeleting || warpLicenseApplying || warpTaskRunning || (!warpLicenseKeySet && !warpLicenseKey.trim())"
              @click="deleteWarpLicenseKey"
            >
              {{ warpLicenseDeleting ? t('network.wanLinks.warpLicenseKeyDeleting') : t('network.wanLinks.warpLicenseKeyDelete') }}
            </button>
          </div>
        </div>
      </div>
      <div class="flex flex-wrap gap-2 items-center">
        <button type="button" class="btn-secondary" :disabled="warpActionLocked || warpTaskRunning || !warpStatus.root || warpStatus.installed || warpInstallRunning" @click="installWarp">
          {{ warpInstallRunning ? t('network.wanLinks.warpInstalling') : t('network.wanLinks.warpInstallBtn') }}
        </button>
        <button type="button" class="btn-secondary" :disabled="warpActionLocked || warpTaskRunning || !warpStatus.root || !warpStatus.installed || warpEnabled || warpUiConnected" @click="connectWarp">
          {{ warpConnecting ? t('network.wanLinks.warpConnecting') : t('network.wanLinks.warpConnectBtn') }}
        </button>
        <button type="button" class="btn-secondary" :disabled="warpActionLocked || (warpTaskRunning && !warpDisconnecting) || !warpStatus.root || !warpStatus.installed || (!warpEnabled && !warpUiConnected) || warpDisconnecting" @click="disconnectWarp">
          {{ warpDisconnecting ? t('network.wanLinks.warpDisconnecting') : t('network.wanLinks.warpDisconnectBtn') }}
        </button>
        <span
          v-if="warpEnabled && warpUiConnected"
          class="text-xs text-slate-600 font-mono pl-1 border-l border-slate-200"
          :title="warpExitInfo?.org || warpServiceLine || ''"
        >
          <span v-if="warpExitLine">
            {{ t('network.wanLinks.warpExitLabel') }}: {{ warpExitLine }}
            <span v-if="warpExitCheckedAt" class="text-slate-500">{{ t('network.wanLinks.warpExitCheckedAt', { time: warpExitCheckedAt }) }}</span>
          </span>
          <span v-else class="text-slate-400">{{ t('network.wanLinks.warpExitLoading') }}</span>
        </span>
      </div>
      <div
        v-if="warpTaskPanelVisible"
        class="mt-1 p-3 rounded border text-xs space-y-2"
        :class="warpActiveJob?.state === 'failed' ? 'border-red-200 bg-red-50' : 'border-slate-200 bg-slate-50'"
      >
        <div class="flex flex-wrap gap-x-3 gap-y-1 text-sm">
          <span>
            {{ t('network.wanLinks.warpTask') }}:
            <strong>{{ warpTaskOpLabel(warpActiveJob?.op) }} / {{ warpActiveJob?.state || 'running' }}</strong>
          </span>
          <span v-if="warpActiveJob?.message" class="text-slate-600">{{ warpActiveJob.message }}</span>
          <span v-if="warpTaskStatusLine" class="text-slate-600 font-mono text-xs">{{ warpTaskStatusLine }}</span>
        </div>
        <pre
          v-if="warpTaskDiagnostics"
          class="max-h-32 overflow-auto whitespace-pre-wrap font-mono text-[11px] text-slate-700"
        >{{ JSON.stringify(warpTaskDiagnostics, null, 2) }}</pre>
      </div>
      <div
        v-if="warpInstallRunning || (warpInstallJob && (warpInstallJob.state === 'running' || warpInstallJob.state === 'failed'))"
        class="mt-1 p-3 rounded border text-xs space-y-2"
        :class="warpInstallJob?.state === 'failed' ? 'border-red-200 bg-red-50' : 'border-slate-200 bg-slate-50'"
      >
        <div class="flex gap-3 text-sm">
          <span>{{ t('network.wanLinks.warpInstallTask') }}: <strong>{{ warpInstallJob?.state || 'running' }}</strong></span>
          <span v-if="warpInstallJob?.message" class="text-slate-600">{{ warpInstallJob.message }}</span>
        </div>
        <pre v-if="warpInstallJob?.log_tail" class="max-h-40 overflow-auto whitespace-pre-wrap font-mono text-[11px] text-slate-700">{{ warpInstallJob.log_tail }}</pre>
      </div>
    </div>

    <div v-show="activeTab === 'egress'" class="card card-body space-y-3 text-sm">
      <div>
        <h3 class="font-medium text-slate-800">{{ t('network.wanLinks.egressTitle') }}</h3>
        <p class="text-xs text-slate-500 mt-1">{{ t('network.wanLinks.egressHint') }}</p>
      </div>
      <div class="grid sm:grid-cols-2 gap-3">
        <div>
          <label class="text-xs text-slate-500">{{ t('common.name') }}</label>
          <input v-model="egForm.name" class="input-field mt-1" />
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('network.wanLinks.wanLink') }}</label>
          <select v-model="egForm.wan_link_id" class="input-field mt-1">
            <option value="">{{ t('network.interfaces.choose') }}</option>
            <option v-for="o in linkOptions" :key="o.id" :value="o.id">{{ o.label }}</option>
          </select>
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('network.wanLinks.srcAddress') }}</label>
          <select v-model="egForm.src_mode" class="input-field mt-1 mb-1">
            <option value="none">{{ t('network.wanLinks.matchAny') }}</option>
            <option value="cidr">{{ t('network.wanLinks.matchCidr') }}</option>
            <option value="alias">{{ t('network.wanLinks.matchAlias') }}</option>
          </select>
          <input
            v-if="egForm.src_mode === 'cidr'"
            v-model="egForm.src_cidr"
            class="input-field font-mono"
            placeholder="10.250.0.0/24"
          />
          <select v-else-if="egForm.src_mode === 'alias'" v-model="egForm.src_alias" class="input-field font-mono">
            <option value="">{{ t('network.interfaces.choose') }}</option>
            <option v-for="a in aliases" :key="a.name" :value="a.name">
              {{ a.name }} ({{ (a.members || []).length }})
            </option>
          </select>
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('network.wanLinks.dstAddress') }}</label>
          <select v-model="egForm.dst_mode" class="input-field mt-1 mb-1">
            <option value="none">{{ t('network.wanLinks.matchAny') }}</option>
            <option value="cidr">{{ t('network.wanLinks.matchCidr') }}</option>
            <option value="alias">{{ t('network.wanLinks.matchAlias') }}</option>
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
          <label class="text-xs text-slate-500">{{ t('network.wanLinks.snatIp') }}</label>
          <input v-model="egForm.snat_ip" class="input-field mt-1 font-mono" :placeholder="t('network.wanLinks.snatAuto')" />
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('network.wanLinks.priority') }}</label>
          <input v-model.number="egForm.priority" type="number" class="input-field mt-1" />
        </div>
        <label class="flex items-center gap-2 sm:col-span-2">
          <input v-model="egForm.enabled" type="checkbox" /> {{ t('common.enabled') }}
        </label>
      </div>
      <div class="flex flex-wrap gap-2">
        <button type="button" class="btn-primary" :disabled="!egForm.wan_link_id" @click="addEgress">{{ t('common.add') }}</button>
        <button
          type="button"
          class="btn-secondary"
          :disabled="!egForm.wan_link_id || !cloudflareCIDRs.length"
          @click="addCloudflarePreset"
        >
          {{ t('network.wanLinks.cloudflarePreset') }}
        </button>
        <button type="button" class="btn-secondary" :disabled="!egForm.wan_link_id" @click="addGooglePreset">
          {{ t('network.wanLinks.googlePreset') }}
        </button>
        <RouterLink to="/firewall/aliases" class="btn-secondary text-xs inline-flex items-center">
          {{ t('network.wanLinks.manageAliases') }}
        </RouterLink>
      </div>
    </div>

    <div v-show="activeTab === 'egress'" class="table-wrap card">
      <table class="data w-full text-sm">
        <thead>
          <tr>
            <th>{{ t('common.name') }}</th>
            <th>{{ t('network.wanLinks.endpoints') }}</th>
            <th>{{ t('network.wanLinks.wanLink') }}</th>
            <th>SNAT</th>
            <th>{{ t('network.wanLinks.routeTable') }}</th>
            <th>{{ t('network.wanLinks.priority') }}</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="p in egress" :key="p.id" :class="editingEgressId === p.id ? 'bg-slate-50' : ''">
            <template v-if="editingEgressId === p.id">
              <td><input v-model="egEditForm.name" class="input-field text-xs" /></td>
              <td class="space-y-1 min-w-[14rem]">
                <select v-model="egEditForm.src_mode" class="input-field text-xs">
                  <option value="none">{{ t('network.wanLinks.srcAddress') }}: {{ t('network.wanLinks.matchAny') }}</option>
                  <option value="cidr">{{ t('network.wanLinks.srcAddress') }}: CIDR</option>
                  <option value="alias">{{ t('network.wanLinks.srcAddress') }}: {{ t('network.wanLinks.matchAlias') }}</option>
                </select>
                <input v-if="egEditForm.src_mode === 'cidr'" v-model="egEditForm.src_cidr" class="input-field text-xs font-mono" />
                <select v-else-if="egEditForm.src_mode === 'alias'" v-model="egEditForm.src_alias" class="input-field text-xs">
                  <option v-for="a in aliases" :key="'es-' + a.name" :value="a.name">{{ a.name }}</option>
                </select>
                <select v-model="egEditForm.dst_mode" class="input-field text-xs">
                  <option value="none">{{ t('network.wanLinks.dstAddress') }}: {{ t('network.wanLinks.matchAny') }}</option>
                  <option value="cidr">{{ t('network.wanLinks.dstAddress') }}: CIDR</option>
                  <option value="alias">{{ t('network.wanLinks.dstAddress') }}: {{ t('network.wanLinks.matchAlias') }}</option>
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
              <td><input v-model="egEditForm.snat_ip" class="input-field text-xs font-mono" /></td>
              <td>{{ resolvedRow(p.id)?.table ?? '—' }}</td>
              <td><input v-model.number="egEditForm.priority" type="number" class="input-field text-xs w-16" /></td>
              <td class="space-x-2 whitespace-nowrap">
                <label class="inline-flex items-center gap-1 text-xs">
                  <input v-model="egEditForm.enabled" type="checkbox" /> {{ t('common.enabled') }}
                </label>
                <button type="button" class="text-indigo-600 text-xs" @click="saveEditEgress">{{ t('common.save') }}</button>
                <button type="button" class="text-slate-500 text-xs" @click="cancelEditEgress">{{ t('common.cancel') }}</button>
              </td>
            </template>
            <template v-else>
              <td>{{ p.name || p.id }}</td>
              <td class="font-mono text-xs">{{ egressEndpointsLabel(p) }}</td>
              <td class="font-mono">{{ links.find((w) => w.id === p.wan_link_id)?.name || p.wan_link_id }}</td>
              <td class="font-mono text-xs">
                {{ resolvedRow(p.id)?.snat_ip || p.snat_ip || t('network.wanLinks.snatAuto') }}
              </td>
              <td>{{ resolvedRow(p.id)?.table ?? '—' }}</td>
              <td>{{ p.priority }}</td>
              <td class="space-x-2 whitespace-nowrap">
                <button type="button" class="text-indigo-600 text-xs" @click="startEditEgress(p)">{{ t('common.edit') }}</button>
                <button type="button" class="text-red-600 text-xs" @click="removeEgress(p.id)">{{ t('common.delete') }}</button>
              </td>
            </template>
          </tr>
          <tr v-if="!egress.length">
            <td colspan="7" class="text-center text-slate-400 py-3">{{ t('network.wanLinks.noEgress') }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
