<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { api } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'
import PageTabs from '@/components/PageTabs.vue'

const { t } = useI18n()
const links = ref([])
const wanHealth = ref({})
const wanHealthPoll = ref(null)
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
const ifaces = ref([])
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
  policy_only: false,
  monitor_enabled: false,
  monitor_addr: '',
  monitor_interval_sec: 5,
  monitor_loss_threshold: 3,
})
const editingId = ref(null)
const editForm = ref({
  name: '',
  device: '',
  gateway: '',
  metric: 200,
  tier: 2,
  weight: 1,
  enabled: true,
  policy_only: false,
})

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
  { id: 'proxy', label: t('network.wanLinks.tabProxy') },
])

const proxyItems = ref([])
const proxyInstalled = ref(false)
const proxyVersion = ref('')
const proxyRoot = ref(false)
const proxyInstallJob = ref(null)
const proxyInstallPoll = ref(null)
const installingProxy = ref(false)
const uninstallingProxy = ref(false)
const proxyTaskJob = ref(null)
const proxyTaskPoll = ref(null)
const proxyBusyId = ref('')
const proxyForm = ref({
  name: '',
  type: 'socks5',
  server: '',
  port: 1080,
  username: '',
  password: '',
})
const proxyInstallRunning = computed(
  () => installingProxy.value || proxyInstallJob.value?.state === 'running'
)
const proxyTaskRunning = computed(() => proxyTaskJob.value?.state === 'running' || !!proxyBusyId.value)

function formatProxyExitCheckedAt(iso) {
  if (!iso) return ''
  try {
    return new Date(iso).toLocaleString()
  } catch {
    return iso
  }
}

function proxyExitLine(p) {
  const e = p?.exit_info
  if (!e) {
    if (p?.egress_ip) return p.egress_ip
    return ''
  }
  if (e.ip) {
    const loc = [e.city, e.region, e.country].filter(Boolean).join(', ')
    return loc ? `${e.ip} · ${loc}` : e.ip
  }
  if (e.error) return e.error
  return ''
}

function proxyExitOrg(p) {
  return p?.exit_info?.org || ''
}

async function refreshProxyStatus() {
  const st = await api.network.proxyEgress.status()
  proxyInstalled.value = !!st?.installed
  proxyVersion.value = st?.version || ''
  proxyRoot.value = !!st?.root
  proxyItems.value = st?.items || []
  if (st?.install_job && st.install_job.state !== 'idle' && st.install_job.state !== 'ok') {
    proxyInstallJob.value = st.install_job
    installingProxy.value = st.install_job.state === 'running'
  } else if (st?.install_job?.state === 'ok') {
    proxyInstallJob.value = null
    installingProxy.value = false
  }
  if (st?.task && st.task.state !== 'idle') {
    proxyTaskJob.value = st.task
    if (st.task.state === 'running' && !proxyTaskPoll.value) {
      proxyBusyId.value = st.task.proxy_id || ''
      startProxyTaskPoll()
    }
    if (st.task.state === 'ok' || st.task.state === 'failed') {
      proxyBusyId.value = ''
    }
  }
  return st
}

async function installProxy() {
  err.value = ''
  ok.value = ''
  try {
    installingProxy.value = true
    await api.network.proxyEgress.install()
    startProxyInstallPoll()
  } catch (e) {
    installingProxy.value = false
    err.value = e?.message || String(e)
  }
}

async function uninstallProxy() {
  if (!confirm(t('network.wanLinks.proxyUninstallConfirm'))) return
  err.value = ''
  ok.value = ''
  uninstallingProxy.value = true
  try {
    await api.network.proxyEgress.uninstall()
    ok.value = t('network.wanLinks.proxyUninstalled')
    proxyInstallJob.value = null
    await load()
  } catch (e) {
    err.value = e?.message || t('network.wanLinks.proxyUninstallFailed')
  } finally {
    uninstallingProxy.value = false
  }
}

function startProxyInstallPoll() {
  stopProxyInstallPoll()
  proxyInstallPoll.value = setInterval(async () => {
    try {
      const job = await api.network.proxyEgress.installStatus()
      proxyInstallJob.value = job
      if (job?.state === 'ok') {
        stopProxyInstallPoll()
        installingProxy.value = false
        ok.value = t('network.wanLinks.proxyInstalled')
        await refreshProxyStatus()
      } else if (job?.state === 'failed') {
        stopProxyInstallPoll()
        installingProxy.value = false
        err.value = job.message || t('network.wanLinks.proxyInstallFailed')
      }
    } catch {
      /* ignore */
    }
  }, 2000)
}

function stopProxyInstallPoll() {
  if (proxyInstallPoll.value) {
    clearInterval(proxyInstallPoll.value)
    proxyInstallPoll.value = null
  }
}

function startProxyTaskPoll() {
  stopProxyTaskPoll()
  proxyTaskPoll.value = setInterval(async () => {
    try {
      const job = await api.network.proxyEgress.taskStatus()
      proxyTaskJob.value = job
      if (job?.state === 'ok') {
        stopProxyTaskPoll()
        proxyBusyId.value = ''
        if (job.exit_info?.ip) {
          ok.value = t('network.wanLinks.proxyTestOk') + ': ' + proxyExitLine({ exit_info: job.exit_info })
        } else {
          ok.value = job.message || t('common.saved')
        }
        await load()
      } else if (job?.state === 'failed') {
        stopProxyTaskPoll()
        proxyBusyId.value = ''
        const line = proxyExitLine({ exit_info: job.exit_info })
        err.value = line || job.message || t('network.wanLinks.proxyTestFailed')
        await refreshProxyStatus()
      }
    } catch {
      /* ignore */
    }
  }, 1500)
}

function stopProxyTaskPoll() {
  if (proxyTaskPoll.value) {
    clearInterval(proxyTaskPoll.value)
    proxyTaskPoll.value = null
  }
}

async function addProxy() {
  err.value = ''
  ok.value = ''
  try {
    const body = {
      name: proxyForm.value.name,
      type: proxyForm.value.type,
      server: proxyForm.value.server,
      port: Number(proxyForm.value.port) || 0,
      username: proxyForm.value.username || undefined,
      password: proxyForm.value.password || undefined,
    }
    const res = await api.network.proxyEgress.add(body)
    proxyForm.value = { name: '', type: 'socks5', server: '', port: 1080, username: '', password: '' }
    const id = res?.proxy?.id || res?.id
    if (res?.auto_test && id) {
      ok.value = t('network.wanLinks.proxyTesting')
      proxyBusyId.value = id
      startProxyTaskPoll()
      await load()
      return
    }
    ok.value = t('common.saved')
    await load()
    if (proxyInstalled.value && id) {
      await connectProxy(id)
    }
  } catch (e) {
    err.value = e?.message || String(e)
  }
}

async function connectProxy(id) {
  err.value = ''
  ok.value = ''
  proxyBusyId.value = id
  try {
    await api.network.proxyEgress.connect(id)
    startProxyTaskPoll()
  } catch (e) {
    proxyBusyId.value = ''
    err.value = e?.message || String(e)
  }
}

async function disconnectProxy(id) {
  err.value = ''
  ok.value = ''
  proxyBusyId.value = id
  try {
    await api.network.proxyEgress.disconnect(id)
    startProxyTaskPoll()
  } catch (e) {
    proxyBusyId.value = ''
    err.value = e?.message || String(e)
  }
}

async function removeProxy(id) {
  if (!confirm(t('network.wanLinks.proxyDeleteConfirm'))) return
  err.value = ''
  try {
    await api.network.proxyEgress.del(id)
    await load()
  } catch (e) {
    err.value = e?.message || String(e)
  }
}
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

async function loadWanHealth() {
  try {
    const d = await api.network.wanHealth()
    const map = {}
    for (const h of d.health || []) {
      if (h?.id) map[h.id] = h
    }
    wanHealth.value = map
  } catch {
    /* optional while tab idle */
  }
}

function healthFor(w) {
  return wanHealth.value?.[w.id] || null
}

function healthLabel(w) {
  if (!w.monitor_enabled) return '—'
  const h = healthFor(w)
  if (!h) return t('network.wanLinks.healthPending')
  if (h.unhealthy) return t('network.wanLinks.healthDown')
  if (h.ok) {
    const ms = h.latency_ms != null ? ` ${h.latency_ms}ms` : ''
    return t('network.wanLinks.healthOK') + ms
  }
  return t('network.wanLinks.healthFailing', { n: h.fail_count || 0 })
}

function healthClass(w) {
  if (!w.monitor_enabled) return 'text-slate-400'
  const h = healthFor(w)
  if (!h) return 'text-slate-400'
  if (h.unhealthy) return 'text-red-600 font-medium'
  if (h.ok) return 'text-emerald-700'
  return 'text-amber-700'
}

function startWanHealthPoll() {
  stopWanHealthPoll()
  wanHealthPoll.value = setInterval(() => {
    loadWanHealth()
  }, 5000)
}

function stopWanHealthPoll() {
  if (wanHealthPoll.value) {
    clearInterval(wanHealthPoll.value)
    wanHealthPoll.value = null
  }
}

async function load() {
  err.value = ''
  try {
    const [wan, ws, ifs] = await Promise.all([
      api.network.wanLinks.list(),
      api.network.warp.status(),
      api.interfaces.list(),
    ])
    links.value = wan?.wan_links || []
    devWan.value = wan?.dev_wan || ''
    ifaces.value = ifs?.interfaces || []
    applyWarpStatus(ws)
    await loadWanHealth()
    try {
      await refreshProxyStatus()
    } catch {
      /* optional */
    }
    if (!form.value.device && devWan.value) form.value.device = devWan.value
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
    policy_only: !!w.policy_only,
    monitor_enabled: !!w.monitor_enabled,
    monitor_addr: w.monitor_addr || '',
    monitor_interval_sec: w.monitor_interval_sec || 5,
    monitor_loss_threshold: w.monitor_loss_threshold || 3,
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

onMounted(async () => {
  await load()
  startWarpStatusPoll()
  startWanHealthPoll()
})
onUnmounted(() => {
  stopWarpInstallPoll()
  stopWarpStatusPoll()
  stopWarpTaskPoll()
  stopProxyInstallPoll()
  stopProxyTaskPoll()
  stopWanHealthPoll()
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
    <p class="text-xs text-slate-500 -mt-1">
      {{ t('network.wanLinks.egressPoliciesHint') }}
      <RouterLink to="/network/egress-policies" class="text-blue-600 hover:underline">{{ t('network.wanLinks.egressPoliciesLink') }}</RouterLink>
    </p>

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
        <div class="sm:col-span-2 space-y-1">
          <label class="flex items-center gap-2">
            <input v-model="form.policy_only" type="checkbox" /> {{ t('network.wanLinks.policyOnly') }}
          </label>
          <p class="text-[11px] text-slate-400 leading-snug">{{ t('network.wanLinks.policyOnlyHint') }}</p>
        </div>
        <label class="flex items-center gap-2 sm:col-span-2">
          <input v-model="form.monitor_enabled" type="checkbox" /> {{ t('network.wanLinks.monitorEnabled') }}
        </label>
        <div v-if="form.monitor_enabled">
          <label class="text-xs text-slate-500">{{ t('network.wanLinks.monitorAddr') }}</label>
          <input v-model="form.monitor_addr" class="input-field mt-1 font-mono" />
        </div>
        <div v-if="form.monitor_enabled">
          <label class="text-xs text-slate-500">{{ t('network.wanLinks.monitorInterval') }}</label>
          <input v-model.number="form.monitor_interval_sec" type="number" class="input-field mt-1" />
        </div>
        <div v-if="form.monitor_enabled">
          <label class="text-xs text-slate-500">{{ t('network.wanLinks.monitorLoss') }}</label>
          <input v-model.number="form.monitor_loss_threshold" type="number" class="input-field mt-1" />
        </div>
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
            <th>{{ t('network.wanLinks.healthStatus') }}</th>
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
              <td class="text-xs text-slate-400">—</td>
              <td class="space-x-2 whitespace-nowrap">
                <label class="inline-flex items-center gap-1 text-xs">
                  <input v-model="editForm.enabled" type="checkbox" /> {{ t('common.enabled') }}
                </label>
                <label class="inline-flex items-center gap-1 text-xs" :title="t('network.wanLinks.policyOnlyHint')">
                  <input v-model="editForm.policy_only" type="checkbox" /> {{ t('network.wanLinks.policyOnlyShort') }}
                </label>
                <label class="inline-flex items-center gap-1 text-xs">
                  <input v-model="editForm.monitor_enabled" type="checkbox" /> {{ t('network.wanLinks.monitorEnabled') }}
                </label>
                <button type="button" class="text-indigo-600 text-xs" @click="saveEdit">{{ t('common.save') }}</button>
                <button type="button" class="text-slate-500 text-xs" @click="cancelEdit">{{ t('common.cancel') }}</button>
              </td>
            </template>
            <template v-else>
              <td>
                {{ w.name }}
                <span v-if="w.warp_managed" class="ml-1 text-[10px] px-1 py-0.5 rounded bg-violet-100 text-violet-800">WARP</span>
                <span v-else-if="w.proxy_managed" class="ml-1 text-[10px] px-1 py-0.5 rounded bg-emerald-100 text-emerald-800">Proxy</span>
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
              <td class="text-xs whitespace-nowrap" :class="healthClass(w)">{{ healthLabel(w) }}</td>
              <td class="space-x-2 whitespace-nowrap">
                <button
                  v-if="!w.warp_managed && !w.proxy_managed"
                  type="button"
                  class="text-indigo-600 text-xs"
                  @click="startEdit(w)"
                >{{ t('common.edit') }}</button>
                <button
                  v-if="!w.warp_managed && !w.proxy_managed"
                  type="button"
                  class="text-red-600 text-xs"
                  @click="remove(w.id)"
                >{{ t('common.delete') }}</button>
                <span v-else-if="w.warp_managed" class="text-slate-400 text-xs">{{ t('network.wanLinks.warpManagedNoDelete') }}</span>
                <span v-else class="text-slate-400 text-xs">{{ t('network.wanLinks.proxyManagedNoDelete') }}</span>
              </td>
            </template>
          </tr>
          <tr v-if="!links.length">
            <td colspan="8" class="text-center text-slate-400 py-3">{{ t('network.wanLinks.noExtra') }}</td>
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

    <div v-show="activeTab === 'proxy'" class="card card-body mb-0 space-y-3 text-sm">
      <div>
        <h3 class="font-medium text-slate-800">{{ t('network.wanLinks.proxyTitle') }}</h3>
        <p class="text-xs text-slate-500 mt-1">{{ t('network.wanLinks.proxyHint') }}</p>
      </div>
      <div class="text-xs text-slate-600 rounded bg-slate-50 p-2 space-y-1">
        <div>
          {{ proxyInstalled ? t('network.wanLinks.proxyInstalled') : t('network.wanLinks.proxyNotInstalled') }}
          <span v-if="proxyVersion" class="font-mono text-slate-500"> · {{ proxyVersion }}</span>
        </div>
      </div>
      <div class="flex flex-wrap gap-2 items-center">
        <button
          v-if="!proxyInstalled"
          type="button"
          class="btn-secondary"
          :disabled="!proxyRoot || proxyInstallRunning || uninstallingProxy"
          @click="installProxy"
        >
          {{ proxyInstallRunning ? t('network.wanLinks.proxyInstalling') : t('network.wanLinks.proxyInstallBtn') }}
        </button>
        <button
          v-else
          type="button"
          class="btn-secondary text-red-700 border-red-200"
          :disabled="!proxyRoot || proxyInstallRunning || uninstallingProxy || proxyTaskRunning"
          @click="uninstallProxy"
        >
          {{ uninstallingProxy ? t('network.wanLinks.proxyUninstalling') : t('network.wanLinks.proxyUninstallBtn') }}
        </button>
      </div>
      <div
        v-if="proxyInstallRunning || (proxyInstallJob && (proxyInstallJob.state === 'running' || proxyInstallJob.state === 'failed'))"
        class="p-3 rounded border text-xs space-y-2"
        :class="proxyInstallJob?.state === 'failed' ? 'border-red-200 bg-red-50' : 'border-slate-200 bg-slate-50'"
      >
        <div>
          <strong>{{ proxyInstallJob?.state || 'running' }}</strong>
          <span v-if="proxyInstallJob?.message" class="text-slate-600 ml-2">{{ proxyInstallJob.message }}</span>
        </div>
        <pre v-if="proxyInstallJob?.log_tail" class="max-h-32 overflow-auto whitespace-pre-wrap font-mono text-[11px]">{{ proxyInstallJob.log_tail }}</pre>
      </div>

      <div class="border-t border-slate-100 pt-3 grid sm:grid-cols-2 gap-3">
        <div>
          <label class="text-xs text-slate-500">{{ t('network.wanLinks.proxyName') }}</label>
          <input v-model="proxyForm.name" class="input-field mt-1" placeholder="US residential" />
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('network.wanLinks.proxyType') }}</label>
          <select v-model="proxyForm.type" class="input-field mt-1">
            <option value="socks5">SOCKS5</option>
            <option value="http">HTTP</option>
            <option value="https">HTTPS</option>
          </select>
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('network.wanLinks.proxyServer') }}</label>
          <input v-model="proxyForm.server" class="input-field mt-1 font-mono" placeholder="proxy.example.com" />
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('network.wanLinks.proxyPort') }}</label>
          <input v-model.number="proxyForm.port" type="number" min="1" max="65535" class="input-field mt-1 font-mono" />
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('network.wanLinks.proxyUsername') }}</label>
          <input v-model="proxyForm.username" class="input-field mt-1 font-mono" autocomplete="off" />
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('network.wanLinks.proxyPassword') }}</label>
          <input v-model="proxyForm.password" type="password" class="input-field mt-1 font-mono" autocomplete="new-password" />
        </div>
      </div>
      <div>
        <button type="button" class="btn-primary" :disabled="!proxyForm.server || !proxyForm.port" @click="addProxy">
          {{ t('network.wanLinks.proxyAdd') }}
        </button>
      </div>
    </div>

    <div v-show="activeTab === 'proxy'" class="table-wrap card">
      <table class="data-table">
        <thead>
          <tr>
            <th>{{ t('network.wanLinks.proxyName') }}</th>
            <th>{{ t('network.wanLinks.proxyType') }}</th>
            <th>{{ t('network.wanLinks.proxyServer') }}</th>
            <th>{{ t('network.wanLinks.proxyDevice') }}</th>
            <th>{{ t('network.wanLinks.proxyEgressIP') }}</th>
            <th>{{ t('common.status') }}</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="!proxyItems.length">
            <td colspan="7" class="text-slate-400 text-sm">{{ t('network.wanLinks.proxyEmpty') }}</td>
          </tr>
          <tr v-for="p in proxyItems" :key="p.id">
            <td>
              {{ p.name }}
              <div class="text-[10px] text-slate-400 font-mono">{{ p.wan_link_id }}</div>
            </td>
            <td class="font-mono uppercase text-xs">{{ p.type }}</td>
            <td class="font-mono text-xs">{{ p.server }}:{{ p.port }}</td>
            <td class="font-mono">{{ p.device || '—' }}</td>
            <td class="font-mono text-xs">
              <div v-if="proxyExitLine(p)">{{ proxyExitLine(p) }}</div>
              <div v-else-if="proxyBusyId === p.id && proxyTaskRunning" class="text-slate-400">
                {{ t('network.wanLinks.proxyExitLoading') }}
              </div>
              <span v-else class="text-slate-400">—</span>
              <div
                v-if="p.exit_info?.fetched_at"
                class="text-[10px] text-slate-400 font-sans"
              >
                {{ t('network.wanLinks.proxyExitCheckedAt', { time: formatProxyExitCheckedAt(p.exit_info.fetched_at) }) }}
              </div>
              <div v-if="proxyExitOrg(p)" class="text-[10px] text-slate-500 font-sans" :title="proxyExitOrg(p)">
                {{ proxyExitOrg(p) }}
              </div>
            </td>
            <td class="text-xs">
              <span :class="p.running ? 'text-emerald-700' : 'text-slate-500'">
                {{ p.running ? t('network.wanLinks.proxyRunning') : t('network.wanLinks.proxyStopped') }}
              </span>
              ·
              {{ p.enabled ? t('network.wanLinks.proxyEnabled') : t('network.wanLinks.proxyDisabled') }}
              <span v-if="p.test_ok" class="text-emerald-700"> · {{ t('network.wanLinks.proxyTestOk') }}</span>
              <span v-else-if="p.exit_info?.error" class="text-red-600"> · {{ t('network.wanLinks.proxyTestFailed') }}</span>
            </td>
            <td class="space-x-2 whitespace-nowrap">
              <button
                type="button"
                class="text-indigo-600 text-xs"
                :disabled="!proxyInstalled || proxyTaskRunning || p.running"
                @click="connectProxy(p.id)"
              >
                {{ proxyBusyId === p.id && !p.running ? t('network.wanLinks.proxyConnecting') : t('network.wanLinks.proxyConnect') }}
              </button>
              <button
                type="button"
                class="text-amber-700 text-xs"
                :disabled="proxyTaskRunning || (!p.running && !p.enabled)"
                @click="disconnectProxy(p.id)"
              >
                {{ proxyBusyId === p.id && (p.running || p.enabled) ? t('network.wanLinks.proxyDisconnecting') : t('network.wanLinks.proxyDisconnect') }}
              </button>
              <button
                type="button"
                class="text-red-600 text-xs"
                :disabled="proxyTaskRunning || p.running || p.enabled"
                @click="removeProxy(p.id)"
              >{{ t('common.delete') }}</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

  </div>
</template>
