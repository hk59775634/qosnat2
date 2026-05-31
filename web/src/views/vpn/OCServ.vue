<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { api } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'
import SnmpTrafficChart from '@/components/SnmpTrafficChart.vue'
import { buildVhostPayload, emptyBasicVhost, vhostFormFromGlobal } from '@/lib/ocservVhostForm'
import { clientMbpsFromOcserv, ocservBpsFromClientMbps } from '@/lib/ocservRate'
import { copyText } from '@/lib/clipboard'
import OCServRadiusHelpModal from '@/components/OCServRadiusHelpModal.vue'
import OCServRadiusChallengeHelpModal from '@/components/OCServRadiusChallengeHelpModal.vue'
import CertSelect from '@/components/CertSelect.vue'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()

const cfg = ref(null)
const status = ref(null)
const installScript = ref('')
const users = ref([])
const err = ref('')
const ok = ref('')
const installing = ref(false)
const installJob = ref(null)
const installPollTimer = ref(null)
const adminUser = ref('admin')
const showUninstallModal = ref(false)
const uninstallPassword = ref('')
const uninstallErr = ref('')
const uninstallSubmitting = ref(false)
const restartHints = ref([])
const serviceSubmitting = ref(false)
const rootForServiceOps = ref(true)
const activeTab = ref('overview')
const detail = ref(null)
const sessions = ref([])
const opsErr = ref('')
const sessionsPoll = ref(null)
const sessionSearch = ref('')
const sessionPage = ref(1)
const SESSION_PAGE_SIZE = 20

const editingUser = ref(null)
const userSearch = ref('')
const trafficModal = ref(null)
const trafficData = ref(null)
const trafficPeriod = ref('7d')
const trafficLoading = ref(false)
const trafficErr = ref('')
const trafficPoll = ref(null)
const trafficLastUpdated = ref(null)
const TRAFFIC_POLL_MS = 5000
const trafficLiveEnabled = ref(false)
const trafficLivePoll = ref(null)
const trafficLiveSeries = ref([])
const trafficLiveCounters = ref(null)
const trafficLiveErr = ref('')
const TRAFFIC_LIVE_POLL_MS = 2000
const TRAFFIC_LIVE_WINDOW_SEC = 300
const editingGroup = ref(null)
const groupForm = ref(emptyGroupForm())
const groupFormDns = ref('')
const groupFormRoutes = ref('')
const groupFormNoRoutes = ref('')
const groupSearch = ref('')
const basicVhostForm = ref(emptyBasicVhost())
const vhostSearch = ref('')

const tabs = computed(() => [
  { id: 'overview', label: t('ocserv.tabOverview') },
  { id: 'sessions', label: t('ocserv.tabSessions') },
  { id: 'config', label: t('ocserv.tabServer') },
  { id: 'groups', label: t('ocserv.tabGroups') },
  { id: 'vhosts', label: t('ocserv.tabVhosts') },
  { id: 'users', label: t('ocserv.tabUsers') },
  { id: 'certs', label: t('ocserv.tabCerts') },
  { id: 'advanced', label: t('ocserv.tabAdvanced') },
])

const isSettingsTab = computed(() =>
  ['config', 'groups', 'vhosts', 'users', 'certs', 'advanced'].includes(activeTab.value),
)

const showCustomCertPaths = computed(
  () => cfg.value && !cfg.value.managed_cert_id && !cfg.value.use_qosnat_tls,
)

const groupOptions = computed(() => cfg.value?.groups || [])

const filteredGroups = computed(() => {
  const q = groupSearch.value.trim().toLowerCase()
  const list = cfg.value?.groups || []
  if (!q) return list
  return list.filter((g) =>
    [g.name, g.label, g.comment].join(' ').toLowerCase().includes(q),
  )
})

const filteredVhosts = computed(() => {
  const q = vhostSearch.value.trim().toLowerCase()
  const list = cfg.value?.vhosts || []
  if (!q) return list
  return list.filter((v) =>
    [v.domain, v.comment, v.auth_method].join(' ').toLowerCase().includes(q),
  )
})

function emptyGroupForm() {
  return {
    name: '',
    label: '',
    comment: '',
    ipv4_network: '',
    ipv4_netmask: '',
    mtu: 0,
    tunnel_all_dns: false,
    down_mbps: 0,
    up_mbps: 0,
  }
}

const userForm = ref({ username: '', password: '', comment: '', group: '' })
const radiusSecret = ref('')
const radiusSecretSet = ref(false)
const camouflageSecretSet = ref(false)
const camouflageSecret = ref('')
const dnsText = ref('')
const routesText = ref('')
const noRoutesText = ref('')
const connectionInfo = ref(null)
const vhostsMeta = ref([])
/** 带宽 M = Mbps，保存时换算为 ocserv 的 B/s（×125000） */
const downMbps = ref(0)
const upMbps = ref(0)

const isRadius = computed(() => cfg.value?.auth_method === 'radius')
const useOcctl = computed(() => !!cfg.value?.advanced?.use_occtl)

const needsCiscoSvcUdp443 = computed(() => {
  if (!cfg.value?.advanced?.cisco_svc_client_compat) return false
  return !cfg.value.advanced?.udp || cfg.value.udp_port !== 443
})

function usesCiscoSvcCompatAnywhere() {
  if (cfg.value?.advanced?.cisco_svc_client_compat) return true
  return (cfg.value?.vhosts || []).some((v) => v.cisco_svc_client_compat)
}

const overviewCards = computed(() => {
  const d = detail.value?.detail || {}
  const st = detail.value?.status || status.value || {}
  return [
    { label: t('ocserv.installed'), value: st.installed ? t('common.yes') : t('common.no') },
    { label: t('ocserv.running'), value: st.active ? t('common.yes') : t('common.no') },
    { label: t('ocserv.version'), value: st.version || '—' },
    { label: t('ocserv.serviceStatus'), value: d.Status || '—' },
    { label: t('ocserv.activeSessions'), value: d['Active sessions'] ?? '—' },
    { label: t('ocserv.totalSessions'), value: d['Total sessions'] ?? '—' },
    { label: t('ocserv.periodSessions'), value: d['Sessions handled'] ?? '—' },
    { label: t('common.rx'), value: formatTraffic(d._RX, d.RX) },
    { label: t('common.tx'), value: formatTraffic(d._TX, d.TX) },
    { label: t('ocserv.uptime'), value: d._Up_since || d.uptime != null ? `${d.uptime}s` : '—' },
  ]
})

/** 安装任务进行中（含刷新页面后从 install_job 恢复） */
const installRunning = computed(
  () => installing.value || installJob.value?.state === 'running',
)

/** 仅安装中或失败时展示进度区；成功后隐藏 */
const showInstallProgress = computed(() => {
  if (installRunning.value) return true
  const s = installJob.value?.state
  return s === 'running' || s === 'failed'
})

function normalizeInstallJob(job) {
  if (!job || job.state === 'idle' || job.state === 'ok') return null
  return job
}

const defaultAdvanced = () => ({
  try_mtu_discovery: true,
  isolate_workers: true,
  dtls_legacy: true,
  tcp: true,
  udp: true,
  deny_roaming: false,
  cisco_client_compat: true,
  cisco_svc_client_compat: false,
  client_bypass_protocol: false,
  compression: false,
  keepalive: true,
  dpd: true,
  mobile_dpd: true,
  predictable_ips: false,
  ping_leases: false,
  use_occtl: false,
  rekey: true,
  switch_to_tcp: true,
  camouflage: false,
  max_same_clients: 2,
  keepalive_sec: 32400,
  dpd_sec: 90,
  mobile_dpd_sec: 1800,
  cookie_timeout: 300,
  rekey_time: 172800,
  rekey_method: 'ssl',
  auth_timeout: 240,
  switch_to_tcp_timeout: 25,
  rate_limit_ms: 100,
  log_level: 2,
  max_ban_score: 80,
  ban_time: 300,
  ban_reset_time: 1200,
  server_stats_reset_time: 604800,
  rx_data_per_sec: 0,
  tx_data_per_sec: 0,
  camouflage_realm: '',
  default_domain: '',
  config_per_group: '',
  cert_user_oid: '0.9.2342.19200300.100.1.1',
  tls_priorities: 'NORMAL:%SERVER_PRECEDENCE:%COMPAT:-VERS-SSL3.0:-VERS-TLS1.0:-VERS-TLS1.1',
})

const FEATURE_KEYS = [
  'tcp', 'udp', 'try_mtu_discovery', 'isolate_workers', 'dtls_legacy', 'cisco_client_compat',
  'cisco_svc_client_compat', 'client_bypass_protocol', 'deny_roaming', 'compression', 'keepalive',
  'dpd', 'mobile_dpd', 'switch_to_tcp', 'rekey', 'predictable_ips', 'ping_leases', 'use_occtl', 'camouflage',
]

const NUMERIC_KEYS = [
  { key: 'max_same_clients', min: 1 },
  { key: 'rate_limit_ms', min: 0 },
  { key: 'log_level', min: 0, max: 9 },
  { key: 'max_ban_score', min: 1 },
  { key: 'ban_time', min: 1 },
  { key: 'ban_reset_time', min: 1 },
  { key: 'server_stats_reset_time', min: 60 },
  { key: 'keepalive_sec', min: 60, show: () => cfg.value?.advanced?.keepalive },
  { key: 'dpd_sec', min: 10, show: () => cfg.value?.advanced?.dpd },
  { key: 'mobile_dpd_sec', min: 60, show: () => cfg.value?.advanced?.mobile_dpd },
  { key: 'cookie_timeout', min: 60 },
  { key: 'auth_timeout', min: 30 },
  { key: 'rekey_time', min: 3600, show: () => cfg.value?.advanced?.rekey },
  { key: 'switch_to_tcp_timeout', min: 5, show: () => cfg.value?.advanced?.switch_to_tcp },
]

const featureToggles = computed(() =>
  FEATURE_KEYS.map((key) => ({
    key,
    label: t(`ocserv.feat.${key}.label`),
    desc: t(`ocserv.feat.${key}.desc`),
  })),
)

const numericAdvanced = computed(() =>
  NUMERIC_KEYS.map((n) => ({
    ...n,
    label: t(`ocserv.num.${n.key}.label`),
    desc: t(`ocserv.num.${n.key}.desc`),
  })),
)

function listToText(arr) {
  return (arr || []).join('\n')
}

function textToList(s) {
  return String(s || '')
    .split(/[\n,]+/)
    .map((x) => x.trim())
    .filter(Boolean)
}

function ensureDefaults() {
  if (!cfg.value.auth_method) cfg.value.auth_method = 'plain'
  if (!cfg.value.radius) {
    cfg.value.radius = { auth_port: 1812, acct_port: 1813, groupconfig: true, stats_report_time: 360 }
  }
  cfg.value.advanced = { ...defaultAdvanced(), ...(cfg.value.advanced || {}) }
  if (!cfg.value.server_cert_path) cfg.value.server_cert_path = '/etc/ocserv/certs/server-cert.pem'
  if (!cfg.value.server_key_path) cfg.value.server_key_path = '/etc/ocserv/certs/server-key.pem'
  if (!cfg.value.socket_file) cfg.value.socket_file = '/var/run/ocserv-socket'
  dnsText.value = listToText(cfg.value.dns)
  routesText.value = listToText(cfg.value.routes)
  noRoutesText.value = listToText(cfg.value.no_routes)
  const caps = clientMbpsFromOcserv(
    cfg.value.advanced.rx_data_per_sec,
    cfg.value.advanced.tx_data_per_sec,
  )
  downMbps.value = caps.downMbps
  upMbps.value = caps.upMbps
  if (!cfg.value.groups) cfg.value.groups = []
  if (!cfg.value.vhosts) cfg.value.vhosts = []
  if (!cfg.value.config_per_group) cfg.value.config_per_group = '/etc/ocserv/config-per-group/'
  if (!cfg.value.default_group_config) cfg.value.default_group_config = '/etc/ocserv/defaults/group.conf'
}

function buildBody() {
  const body = { ...cfg.value }
  body.dns = textToList(dnsText.value)
  body.routes = textToList(routesText.value)
  body.no_routes = textToList(noRoutesText.value)
  if (body.advanced?.camouflage && camouflageSecret.value) {
    body.advanced = { ...body.advanced, camouflage_secret: camouflageSecret.value }
  }
  if (body.advanced) {
    body.advanced = {
      ...body.advanced,
      ...ocservBpsFromClientMbps(downMbps.value, upMbps.value),
    }
  }
  return body
}

async function load() {
  const d = await api.get('/api/v1/vpn/ocserv')
  cfg.value = d.config || {}
  ensureDefaults()
  status.value = d.status || {}
  installScript.value = d.install_script || ''
  installJob.value = normalizeInstallJob(d.install_job)
  installing.value = d.install_job?.state === 'running'
  if (installing.value && !installPollTimer.value) {
    startInstallPoll()
  }
  users.value = (d.config?.users || []).map((u) => ({ ...u }))
  radiusSecret.value = ''
  radiusSecretSet.value = !!d.radius_secret_set
  camouflageSecret.value = ''
  camouflageSecretSet.value = !!d.camouflage_secret_set
  connectionInfo.value = d.connection || null
  vhostsMeta.value = d.vhosts_meta || []
  adminUser.value = (d.admin_user && String(d.admin_user).trim()) || 'admin'
  restartHints.value = Array.isArray(d.restart_pending_reasons) ? d.restart_pending_reasons : []
  rootForServiceOps.value = d.root_for_service_ops !== false
}

function restartReasonLabel(code) {
  const c = String(code || '')
  if (c.startsWith('vhost:')) {
    return t('ocserv.restartReasonVhost', { domain: c.slice(6) })
  }
  const key = `ocserv.restartReason.${c}`
  const msg = t(key)
  return msg !== key ? msg : c
}

async function controlOcservService(action) {
  err.value = ''
  ok.value = ''
  serviceSubmitting.value = true
  try {
    await api.post('/api/v1/vpn/ocserv/service', { action })
    if (action === 'start') ok.value = t('ocserv.serviceStarted')
    else if (action === 'stop') ok.value = t('ocserv.serviceStopped')
    else ok.value = t('ocserv.serviceRestarted')
    await load()
  } catch (e) {
    const msg = e.data?.error || e.message
    err.value = msg
    if (e.status === 403) {
      err.value = t('ocserv.serviceRootHint', { msg })
    }
  } finally {
    serviceSubmitting.value = false
  }
}

function vhostConnectUrl(domain) {
  const m = vhostsMeta.value.find((x) => x.domain === domain)
  return m?.connection?.url || ''
}

const connectUrlIssueText = computed(() => {
  const issue = connectionInfo.value?.issue
  if (issue === 'no_cert') return t('ocserv.connectUrlIssueNoCert')
  if (issue === 'no_hostname') return t('ocserv.connectUrlIssueNoHostname')
  if (issue === 'camouflage_secret_missing') return t('ocserv.connectUrlIssueCamoSecret')
  return ''
})

async function copyConnectUrl() {
  const url = connectionInfo.value?.url
  if (!url) return
  if (await copyText(url)) {
    ok.value = t('ocserv.connectUrlCopyOk')
    opsErr.value = ''
  } else {
    opsErr.value = t('common.copyFailed')
  }
}

function openVhostAdvanced(v) {
  router.push({
    name: 'vpn-ocserv-vhost',
    params: { domain: encodeURIComponent(v.domain) },
  })
}

function resetBasicVhostForm() {
  basicVhostForm.value = emptyBasicVhost()
}

function stopInstallPoll() {
  if (installPollTimer.value) {
    clearInterval(installPollTimer.value)
    installPollTimer.value = null
  }
}

function startInstallPoll() {
  stopInstallPoll()
  installPollTimer.value = setInterval(async () => {
    try {
      const j = await api.get('/api/v1/vpn/ocserv/install/status')
      installJob.value = j
      if (j.state === 'ok') {
        stopInstallPoll()
        installing.value = false
        installJob.value = null
        ok.value = t('ocserv.installDone')
        await load()
      } else if (j.state === 'idle' && installing.value) {
        // 兜底：若任务刚结束但状态文件缺失/被清理，避免前端长期停留在“安装中”。
        stopInstallPoll()
        installing.value = false
        installJob.value = null
        ok.value = t('ocserv.installDone')
        await load()
      } else if (j.state === 'failed') {
        stopInstallPoll()
        installing.value = false
        err.value = j.message || t('ocserv.installFailed')
      }
    } catch {
      /* ignore poll errors */
    }
  }, 3000)
}

async function runInstall() {
  if (installRunning.value) return
  err.value = ''
  ok.value = ''
  installing.value = true
  try {
    const r = await api.post('/api/v1/vpn/ocserv/install', {})
    ok.value = t('ocserv.installQueued')
    installJob.value = r.job || { state: 'running' }
    startInstallPoll()
  } catch (e) {
    installing.value = false
    const msg = e.data?.error || e.message
    err.value = msg
    if (e.status === 403) {
      err.value = t('ocserv.installRootHint', { msg })
    } else if (e.status === 409) {
      installing.value = true
      installJob.value = e.data?.job || { state: 'running' }
      startInstallPoll()
    }
  }
}

function openUninstallModal() {
  uninstallErr.value = ''
  uninstallPassword.value = ''
  showUninstallModal.value = true
}

function closeUninstallModal() {
  showUninstallModal.value = false
  uninstallPassword.value = ''
  uninstallErr.value = ''
}

async function confirmUninstallOcserv() {
  uninstallErr.value = ''
  if (!uninstallPassword.value) {
    uninstallErr.value = t('ocserv.uninstallPasswordRequired')
    return
  }
  uninstallSubmitting.value = true
  try {
    await api.post('/api/v1/vpn/ocserv/uninstall', { admin_password: uninstallPassword.value })
    ok.value = t('ocserv.uninstallDone')
    closeUninstallModal()
    await load()
  } catch (e) {
    const msg = e.data?.error || e.message || ''
    const msgStr = String(msg)
    if (e.status === 403) {
      if (/incorrect admin password/i.test(msgStr)) {
        uninstallErr.value = t('ocserv.uninstallWrongPassword')
      } else if (/root|降权|qosnatd/i.test(msgStr)) {
        uninstallErr.value = t('ocserv.uninstallRootHint', { msg: msgStr })
      } else {
        uninstallErr.value = msgStr
      }
    } else {
      uninstallErr.value = msgStr
    }
  } finally {
    uninstallSubmitting.value = false
  }
}

function validateCiscoSvcUdp443() {
  if (!usesCiscoSvcCompatAnywhere()) return true
  if (!cfg.value?.advanced?.udp || cfg.value.udp_port !== 443) {
    err.value = t('ocserv.ciscoSvcUdp443Required')
    return false
  }
  return true
}

async function save() {
  err.value = ''
  ok.value = ''
  try {
    const body = buildBody()
    if (!body.advanced?.tcp && !body.advanced?.udp) {
      err.value = t('ocserv.needTcpUdp')
      return
    }
    if (!validateCiscoSvcUdp443()) return
    if (isRadius.value) {
      body.users = []
      if (radiusSecret.value) body.radius = { ...body.radius, secret: radiusSecret.value }
    } else {
      body.users = users.value
    }
    await api.put('/api/v1/vpn/ocserv', body)
    ok.value = t('ocserv.configSaved')
    await load()
  } catch (e) {
    err.value = e.data?.error || e.message
  }
}

async function apply() {
  err.value = ''
  ok.value = ''
  try {
    await save()
    if (err.value) return
    const r = await api.post('/api/v1/vpn/ocserv/apply', {})
    const mode = r.apply_mode
    restartHints.value = Array.isArray(r.restart_pending_reasons) ? r.restart_pending_reasons : []
    if (mode === 'reload') {
      ok.value =
        restartHints.value.length > 0
          ? t('ocserv.appliedReloadWithPendingRestart')
          : t('ocserv.appliedReload')
    } else if (mode === 'stop') {
      ok.value = t('ocserv.stoppedOcserv')
    } else {
      ok.value = cfg.value.enabled ? t('ocserv.appliedStarted') : t('ocserv.stoppedOcserv')
    }
    await load()
  } catch (e) {
    err.value = e.data?.error || e.message
  }
}

async function addUser() {
  err.value = ''
  try {
    await api.post('/api/v1/vpn/ocserv/users', userForm.value)
    userForm.value = { username: '', password: '', comment: '', group: '' }
    ok.value = t('ocserv.userAdded')
    await load()
  } catch (e) {
    err.value = e.data?.error || e.message
  }
}

function startEditUser(u) {
  editingUser.value = {
    username: u.username,
    password: '',
    comment: u.comment || '',
    group: u.group || '',
  }
}

function cancelEditUser() {
  editingUser.value = null
}

async function saveEditUser() {
  if (!editingUser.value) return
  err.value = ''
  try {
    const body = {
      username: editingUser.value.username,
      comment: editingUser.value.comment,
      group: editingUser.value.group,
    }
    if (editingUser.value.password) body.password = editingUser.value.password
    await api.put('/api/v1/vpn/ocserv/users', body)
    editingUser.value = null
    ok.value = t('ocserv.userUpdated')
    await load()
  } catch (e) {
    err.value = e.data?.error || e.message
  }
}

function buildGroupPayload(form, dnsT, routesT, noRoutesT) {
  return {
    name: form.name.trim(),
    label: form.label,
    comment: form.comment,
    dns: textToList(dnsT),
    routes: textToList(routesT),
    no_routes: textToList(noRoutesT),
    ipv4_network: form.ipv4_network,
    ipv4_netmask: form.ipv4_netmask,
    mtu: form.mtu || 0,
    tunnel_all_dns: !!form.tunnel_all_dns,
    ...ocservBpsFromClientMbps(form.down_mbps, form.up_mbps),
  }
}

function startEditGroup(g) {
  editingGroup.value = g.name
  const caps = clientMbpsFromOcserv(g.rx_data_per_sec, g.tx_data_per_sec)
  groupForm.value = {
    name: g.name,
    label: g.label || '',
    comment: g.comment || '',
    ipv4_network: g.ipv4_network || '',
    ipv4_netmask: g.ipv4_netmask || '',
    mtu: g.mtu || 0,
    tunnel_all_dns: !!g.tunnel_all_dns,
    down_mbps: caps.downMbps,
    up_mbps: caps.upMbps,
  }
  groupFormDns.value = listToText(g.dns)
  groupFormRoutes.value = listToText(g.routes)
  groupFormNoRoutes.value = listToText(g.no_routes)
}

function cancelEditGroup() {
  editingGroup.value = null
  groupForm.value = emptyGroupForm()
  groupFormDns.value = ''
  groupFormRoutes.value = ''
  groupFormNoRoutes.value = ''
}

async function saveGroup() {
  err.value = ''
  if (!groupForm.value.name.trim()) {
    err.value = t('ocserv.groupNameRequired')
    return
  }
  try {
    const body = buildGroupPayload(groupForm.value, groupFormDns.value, groupFormRoutes.value, groupFormNoRoutes.value)
    if (editingGroup.value) {
      await api.put('/api/v1/vpn/ocserv/groups', body)
    } else {
      await api.post('/api/v1/vpn/ocserv/groups', body)
    }
    cancelEditGroup()
    ok.value = t('ocserv.groupSaved')
    await load()
  } catch (e) {
    err.value = e.data?.error || e.message
  }
}

async function delGroup(name) {
  if (!confirm(t('ocserv.deleteGroupConfirm', { name }))) return
  try {
    await api.del(`/api/v1/vpn/ocserv/groups?name=${encodeURIComponent(name)}`)
    await load()
    ok.value = t('ocserv.groupDeleted')
  } catch (e) {
    err.value = e.data?.error || e.message
  }
}

async function addVhost() {
  err.value = ''
  if (!basicVhostForm.value.domain.trim()) {
    err.value = t('ocserv.vhostDomainRequired')
    return
  }
  try {
    const body = buildVhostPayload({
      ...vhostFormFromGlobal(cfg.value),
      ...basicVhostForm.value,
      domain: basicVhostForm.value.domain.trim(),
    })
    await api.post('/api/v1/vpn/ocserv/vhosts', body)
    resetBasicVhostForm()
    ok.value = t('ocserv.vhostAdded')
    await load()
  } catch (e) {
    err.value = e.data?.error || e.message
  }
}

async function delVhost(domain) {
  if (!confirm(t('ocserv.deleteVhostConfirm', { domain }))) return
  try {
    await api.del(`/api/v1/vpn/ocserv/vhosts?domain=${encodeURIComponent(domain)}`)
    await load()
    ok.value = t('ocserv.vhostDeleted')
  } catch (e) {
    err.value = e.data?.error || e.message
  }
}

async function delUser(name) {
  if (!confirm(t('ocserv.deleteUserConfirm', { name }))) return
  try {
    await api.del(`/api/v1/vpn/ocserv/users?username=${encodeURIComponent(name)}`)
    await load()
  } catch (e) {
    err.value = e.data?.error || e.message
  }
}

function applyTrafficToUserList(data) {
  if (!data?.username) return
  const rx = Number(data.summary?.total_rx_bytes) || 0
  const tx = Number(data.summary?.total_tx_bytes) || 0
  const idx = users.value.findIndex((u) => u.username === data.username)
  if (idx < 0) return
  users.value[idx] = {
    ...users.value[idx],
    total_rx_bytes: rx,
    total_tx_bytes: tx,
    total_bytes: rx + tx,
  }
}

async function loadUserTraffic(silent = false) {
  if (!trafficModal.value) return
  if (!silent) {
    trafficLoading.value = true
    trafficErr.value = ''
  }
  try {
    const q = new URLSearchParams({
      username: trafficModal.value,
      period: trafficPeriod.value,
    })
    const data = await api.get(`/api/v1/vpn/ocserv/users/traffic?${q}`)
    trafficData.value = data
    trafficLastUpdated.value = Date.now()
    applyTrafficToUserList(data)
    if (trafficLiveEnabled.value && !data.online) {
      stopTrafficLivePoll()
      trafficLiveEnabled.value = false
      resetTrafficLive()
      trafficLiveErr.value = t('ocserv.trafficLiveOffline')
    }
  } catch (e) {
    if (!silent) {
      trafficErr.value = e.data?.error || e.message
      trafficData.value = null
    }
  } finally {
    if (!silent) trafficLoading.value = false
  }
}

function parseOcctlCounterBytes(v) {
  if (v == null || v === '') return 0
  const n = Number(v)
  if (Number.isFinite(n) && n >= 0) return n
  const s = String(v).trim()
  if (/^\d+$/.test(s)) return Number(s)
  return 0
}

function resetTrafficLive() {
  trafficLiveSeries.value = []
  trafficLiveCounters.value = null
  trafficLiveErr.value = ''
}

function stopTrafficLivePoll() {
  if (trafficLivePoll.value) {
    clearInterval(trafficLivePoll.value)
    trafficLivePoll.value = null
  }
}

function appendTrafficLiveSample(current) {
  const rx = parseOcctlCounterBytes(current?.RX ?? current?.rx)
  const tx = parseOcctlCounterBytes(current?.TX ?? current?.tx)
  const now = Date.now()
  const ts = Math.floor(now / 1000)
  const prev = trafficLiveCounters.value
  if (prev) {
    const dtSec = Math.max(0.5, (now - prev.at) / 1000)
    const drx = rx >= prev.rx ? rx - prev.rx : rx
    const dtx = tx >= prev.tx ? tx - prev.tx : tx
    const rxMbps = (drx * 8) / (dtSec * 1_000_000)
    const txMbps = (dtx * 8) / (dtSec * 1_000_000)
    const cut = ts - TRAFFIC_LIVE_WINDOW_SEC
    trafficLiveSeries.value = [
      ...trafficLiveSeries.value,
      { ts, rx_mbps: rxMbps, tx_mbps: txMbps },
    ].filter((p) => p.ts >= cut)
  }
  trafficLiveCounters.value = { rx, tx, at: now }
}

async function pollTrafficLive() {
  if (!trafficModal.value || !trafficLiveEnabled.value) return
  try {
    const q = new URLSearchParams({
      username: trafficModal.value,
      period: '24h',
    })
    const data = await api.get(`/api/v1/vpn/ocserv/users/traffic?${q}`)
    if (trafficData.value) {
      trafficData.value.online = data.online
      trafficData.value.current = data.current
    }
    if (!data.online || !data.current) {
      trafficLiveErr.value = t('ocserv.trafficLiveOffline')
      stopTrafficLivePoll()
      trafficLiveEnabled.value = false
      resetTrafficLive()
      return
    }
    trafficLiveErr.value = ''
    appendTrafficLiveSample(data.current)
    trafficLastUpdated.value = Date.now()
  } catch (e) {
    trafficLiveErr.value = e.data?.error || e.message || t('ocserv.trafficLiveFailed')
  }
}

function startTrafficLivePoll() {
  stopTrafficLivePoll()
  resetTrafficLive()
  pollTrafficLive()
  trafficLivePoll.value = setInterval(pollTrafficLive, TRAFFIC_LIVE_POLL_MS)
}

function onTrafficLiveToggle() {
  if (trafficLiveEnabled.value) {
    if (!trafficData.value?.online) {
      trafficLiveEnabled.value = false
      trafficLiveErr.value = t('ocserv.trafficLiveOffline')
      return
    }
    startTrafficLivePoll()
  } else {
    stopTrafficLivePoll()
    resetTrafficLive()
  }
}

const trafficChartSeries = computed(() => {
  if (trafficLiveEnabled.value) return trafficLiveSeries.value
  return trafficData.value?.series || []
})

function stopTrafficPoll() {
  if (trafficPoll.value) {
    clearInterval(trafficPoll.value)
    trafficPoll.value = null
  }
}

function startTrafficPoll() {
  stopTrafficPoll()
  trafficPoll.value = setInterval(() => loadUserTraffic(true), TRAFFIC_POLL_MS)
}

function openUserTraffic(username) {
  trafficModal.value = username
  trafficPeriod.value = '7d'
  trafficData.value = null
  trafficErr.value = ''
  trafficLastUpdated.value = null
  trafficLiveEnabled.value = false
  resetTrafficLive()
  stopTrafficLivePoll()
  loadUserTraffic()
  startTrafficPoll()
}

function closeUserTraffic() {
  stopTrafficPoll()
  stopTrafficLivePoll()
  trafficLiveEnabled.value = false
  resetTrafficLive()
  trafficModal.value = null
  trafficData.value = null
  trafficErr.value = ''
  trafficLastUpdated.value = null
}

function formatTrafficUpdatedAt(ts) {
  if (!ts) return ''
  return new Date(ts).toLocaleTimeString()
}

watch(trafficPeriod, () => {
  if (trafficModal.value && !trafficLiveEnabled.value) loadUserTraffic()
})

watch(
  () => cfg.value?.advanced?.cisco_svc_client_compat,
  (on) => {
    if (!on || !cfg.value) return
    cfg.value.advanced.udp = true
    cfg.value.udp_port = 443
  },
)

function sessVal(s, ...keys) {
  for (const k of keys) {
    const v = s[k]
    if (v != null && v !== '') return v
  }
  return '—'
}

/** occtl 会话唯一标识（用户名可能为空或重复，断开必须用 id） */
function sessionId(s) {
  const v = s?.ID ?? s?.id
  if (v == null || v === '') return null
  return String(v)
}

function sessionVhostDomain(s) {
  const d = s?.vhost_domain
  if (d != null && String(d).trim() !== '') return String(d)
  return 'unknown'
}

function sessionVhostTitle(s) {
  const host = s?.vhost_hostname
  const label = sessionVhostDomain(s)
  if (host && label !== host) return `${label} (${host})`
  if (host) return String(host)
  if (s?.vhost_raw) return `occtl: ${s.vhost_raw}`
  return ''
}

/** 字节数自动换算 B / KB / MB / GB */
function userTotalBytes(u) {
  if (u?.total_bytes != null && u.total_bytes !== '') return Number(u.total_bytes)
  return (Number(u?.total_rx_bytes) || 0) + (Number(u?.total_tx_bytes) || 0)
}

function formatBytes(n) {
  const num = Number(n)
  if (!Number.isFinite(num) || num < 0) return '—'
  if (num === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let v = num
  let i = 0
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  const digits = v >= 100 ? 0 : v >= 10 ? 1 : 2
  return `${v.toFixed(digits).replace(/\.?0+$/, '')} ${units[i]}`
}

/** 优先 occtl 已格式化的 _RX/_TX，否则按字节换算 */
function formatTraffic(pretty, raw) {
  if (pretty != null && String(pretty).trim() !== '') return String(pretty).trim()
  if (raw == null || raw === '') return '—'
  if (typeof raw === 'string' && /[kmgt]b/i.test(raw) && !/^\d+$/.test(raw.trim())) return raw.trim()
  return formatBytes(raw)
}

function sessTraffic(s, dir) {
  if (dir === 'rx') return formatTraffic(s._RX, s.RX ?? s.rx)
  return formatTraffic(s._TX, s.TX ?? s.tx)
}

/** 客户端连接 VPN 时的公网/源 IP（occtl Remote IP） */
function sessRemoteIp(s) {
  return sessVal(s, 'Remote IP', 'remote_ip', 'Remote-IP', 'Client IP', 'client_ip')
}

function formatDuration(sec) {
  const s = Math.max(0, Math.floor(Number(sec) || 0))
  if (s < 60) return t('ocserv.durationSec', { n: s })
  if (s < 3600) {
    const m = Math.floor(s / 60)
    const r = s % 60
    return r ? `${t('ocserv.durationMin', { n: m })} ${t('ocserv.durationSec', { n: r })}` : t('ocserv.durationMin', { n: m })
  }
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  return m ? `${t('ocserv.durationHour', { n: h })} ${t('ocserv.durationMin', { n: m })}` : t('ocserv.durationHour', { n: h })
}

function sessionRowText(s) {
  return [
    sessionVhostDomain(s),
    sessVal(s, 'Username', 'username'),
    sessVal(s, 'ID', 'id'),
    sessVal(s, 'IPv4', 'VPN IPv4', 'ip'),
    sessRemoteIp(s),
    sessVal(s, 'Device', 'device'),
    sessVal(s, 'Hostname', 'hostname'),
    sessVal(s, 'Local Device IP'),
    sessVal(s, 'User-Agent', 'user_agent'),
    sessVal(s, 'State', 'state'),
  ]
    .join(' ')
    .toLowerCase()
}

const filteredSessions = computed(() => {
  const q = sessionSearch.value.trim().toLowerCase()
  if (!q) return sessions.value
  return sessions.value.filter((s) => sessionRowText(s).includes(q))
})

const sessionPageCount = computed(() =>
  Math.max(1, Math.ceil(filteredSessions.value.length / SESSION_PAGE_SIZE)),
)

const paginatedSessions = computed(() => {
  const page = Math.min(Math.max(1, sessionPage.value), sessionPageCount.value)
  const start = (page - 1) * SESSION_PAGE_SIZE
  return filteredSessions.value.slice(start, start + SESSION_PAGE_SIZE)
})

const filteredUsers = computed(() => {
  const q = userSearch.value.trim().toLowerCase()
  if (!q) return users.value
  return users.value.filter((u) =>
    [u.username, u.comment, u.group].join(' ').toLowerCase().includes(q),
  )
})

/** 在线时长：优先 occtl 相对时间，否则由 raw_session_started_at 计算 */
function sessOnlineDuration(s) {
  const rel = s['_Session started at']
  if (rel != null && String(rel).trim()) return String(rel).trim()
  const raw = s.raw_session_started_at ?? s.raw_connected_at
  if (raw != null) {
    const ts = Number(raw)
    if (Number.isFinite(ts) && ts > 0) {
      return formatDuration(Math.floor(Date.now() / 1000) - ts)
    }
  }
  return '—'
}

async function loadOverview() {
  opsErr.value = ''
  try {
    await load()
    detail.value = await api.get('/api/v1/vpn/ocserv/status/detail')
  } catch (e) {
    opsErr.value = e.data?.error || e.message
    detail.value = null
  }
}

async function loadSessions() {
  opsErr.value = ''
  try {
    const r = await api.get('/api/v1/vpn/ocserv/sessions')
    sessions.value = r.sessions || []
  } catch (e) {
    opsErr.value = e.data?.error || e.message
    sessions.value = []
  }
}

function stopSessionsPoll() {
  if (sessionsPoll.value) {
    clearInterval(sessionsPoll.value)
    sessionsPoll.value = null
  }
}

function startSessionsPoll() {
  stopSessionsPoll()
  sessionsPoll.value = setInterval(loadSessions, 8000)
}

async function disconnectSession(s) {
  const id = sessionId(s)
  if (id == null) {
    opsErr.value = t('ocserv.disconnectNeedId')
    return
  }
  const user = sessVal(s, 'Username', 'username')
  const label = user !== '—' ? `ID ${id} (${user})` : `ID ${id}`
  if (!confirm(t('ocserv.disconnectConfirm', { label }))) return
  opsErr.value = ''
  try {
    await api.post('/api/v1/vpn/ocserv/sessions/disconnect', { id })
    ok.value = t('ocserv.disconnected')
    await loadSessions()
  } catch (e) {
    opsErr.value = e.data?.error || e.message
  }
}

watch(sessionSearch, () => {
  sessionPage.value = 1
})

watch(filteredSessions, () => {
  if (sessionPage.value > sessionPageCount.value) {
    sessionPage.value = sessionPageCount.value
  }
})

watch(activeTab, async (t) => {
  stopSessionsPoll()
  opsErr.value = ''
  if (t === 'overview') await loadOverview()
  else if (t === 'sessions') {
    await loadSessions()
    startSessionsPoll()
  } else if (t === 'users') {
    editingUser.value = null
    await load()
  }
})

onMounted(async () => {
  const tab = route.query.tab
  const validTabs = ['overview', 'sessions', 'config', 'groups', 'vhosts', 'users', 'certs', 'advanced']
  if (typeof tab === 'string' && validTabs.includes(tab)) {
    activeTab.value = tab
  }
  await load()
})
onUnmounted(() => {
  stopInstallPoll()
  stopSessionsPoll()
  stopTrafficPoll()
  stopTrafficLivePoll()
})
</script>

<template>
  <div>
    <PageHeader :title="t('ocserv.title')" :description="t('ocserv.description')" :ok="ok" :err="err" />

    <div class="card p-4 mb-4 space-y-3">
      <div class="flex flex-wrap gap-4 text-sm">
        <span>{{ t('ocserv.installed') }}: <strong>{{ status?.installed ? t('common.yes') : t('common.no') }}</strong></span>
        <span>{{ t('ocserv.running') }}: <strong>{{ status?.active ? t('common.yes') : t('common.no') }}</strong></span>
        <span>{{ t('ocserv.version') }}: <strong class="font-mono">{{ status?.version || '—' }}</strong></span>
        <span>{{ t('ocserv.radiusLinked') }}: <strong>{{ status?.radius_linked ? t('common.yes') : t('common.no') }}</strong></span>
        <span>occtl: <strong>{{ useOcctl ? t('common.enabled') : t('common.disabled') }}</strong></span>
      </div>

      <div class="flex flex-wrap gap-2">
        <button
          v-if="!status?.installed"
          type="button"
          class="btn-secondary text-sm"
          :disabled="installRunning"
          @click="runInstall"
        >
          {{ installRunning ? t('ocserv.installing') : t('ocserv.installFromSource') }}
        </button>
        <button
          v-else
          type="button"
          class="btn-secondary text-sm border-red-200 text-red-700 hover:bg-red-50"
          :disabled="installRunning || uninstallSubmitting || serviceSubmitting"
          @click="openUninstallModal"
        >
          {{ t('ocserv.uninstallFromSource') }}
        </button>
      </div>

      <div
        v-if="status?.installed"
        class="pt-3 mt-1 border-t border-slate-100 space-y-2"
      >
        <p v-if="!rootForServiceOps" class="text-xs text-amber-800">{{ t('ocserv.serviceRootOnlyHint') }}</p>
        <p class="text-xs text-slate-600">{{ t('ocserv.manualReloadVsRestart') }}</p>
        <div class="flex flex-wrap items-start gap-2">
          <button
            v-if="!status?.active"
            type="button"
            class="btn-secondary text-sm"
            :disabled="installRunning || uninstallSubmitting || serviceSubmitting || !rootForServiceOps"
            :title="!rootForServiceOps ? t('ocserv.serviceRootOnlyHint') : undefined"
            @click="controlOcservService('start')"
          >
            {{ serviceSubmitting ? t('common.loading') : t('ocserv.serviceStart') }}
          </button>
          <template v-else>
            <button
              type="button"
              class="btn-secondary text-sm"
              :disabled="installRunning || uninstallSubmitting || serviceSubmitting || !rootForServiceOps"
              :title="!rootForServiceOps ? t('ocserv.serviceRootOnlyHint') : undefined"
              @click="controlOcservService('stop')"
            >
              {{ t('ocserv.serviceStop') }}
            </button>
            <div class="flex flex-col items-start gap-1">
              <button
                type="button"
                class="btn-secondary text-sm"
                :disabled="installRunning || uninstallSubmitting || serviceSubmitting || !rootForServiceOps"
                :title="!rootForServiceOps ? t('ocserv.serviceRootOnlyHint') : undefined"
                @click="controlOcservService('restart')"
              >
                {{ t('ocserv.serviceRestart') }}
              </button>
              <div v-if="restartHints?.length" class="rounded border border-amber-200 bg-amber-50/80 px-3 py-2 max-w-2xl">
                <p class="text-xs font-medium text-amber-900">{{ t('ocserv.restartPendingTitle') }}</p>
                <ul class="text-xs text-amber-900 list-disc pl-4 mt-1 space-y-0.5">
                  <li v-for="c in restartHints" :key="c">{{ restartReasonLabel(c) }}</li>
                </ul>
              </div>
            </div>
          </template>
        </div>
      </div>

      <div v-if="showInstallProgress" class="mt-3 p-3 rounded border text-xs space-y-2"
        :class="installJob?.state === 'failed' ? 'border-red-200 bg-red-50' : 'border-slate-200 bg-slate-50'">
        <div class="flex gap-3 text-sm">
          <span>{{ t('ocserv.installTask') }}: <strong>{{ installJob?.state || 'running' }}</strong></span>
          <span v-if="installJob?.message" class="text-slate-600">{{ installJob.message }}</span>
        </div>
        <pre v-if="installJob?.log_tail" class="max-h-40 overflow-auto whitespace-pre-wrap font-mono text-[11px] text-slate-700">{{ installJob.log_tail }}</pre>
        <p v-if="installRunning" class="text-slate-500">{{ t('ocserv.installHint') }}</p>
      </div>
    </div>

    <nav class="flex flex-wrap gap-1 mb-4 border-b border-slate-200">
      <button
        v-for="tab in tabs"
        :key="tab.id"
        type="button"
        class="px-4 py-2 text-sm border-b-2 -mb-px transition-colors"
        :class="activeTab === tab.id ? 'border-blue-600 text-blue-700 font-medium' : 'border-transparent text-slate-600 hover:text-slate-900'"
        @click="activeTab = tab.id"
      >
        {{ tab.label }}
      </button>
    </nav>

    <p v-if="opsErr && !isSettingsTab" class="text-sm text-amber-700 mb-2">{{ opsErr }}</p>

    <div v-if="activeTab === 'overview'" class="space-y-4">
      <div
        v-if="connectionInfo"
        class="card p-4 border border-blue-200 bg-blue-50/50 space-y-2"
      >
        <div class="flex flex-wrap items-start justify-between gap-2">
          <h3 class="text-sm font-semibold text-slate-800">{{ t('ocserv.connectUrlTitle') }}</h3>
          <button
            v-if="connectionInfo.url"
            type="button"
            class="btn-secondary text-xs shrink-0"
            @click="copyConnectUrl"
          >
            {{ t('common.copy') }}
          </button>
        </div>
        <p v-if="connectUrlIssueText" class="text-xs text-amber-800">{{ connectUrlIssueText }}</p>
        <p v-if="connectionInfo.url" class="font-mono text-sm break-all text-blue-900 select-all">
          {{ connectionInfo.url }}
        </p>
        <p class="text-xs text-slate-600">{{ t('ocserv.connectUrlHint') }}</p>
        <dl v-if="connectionInfo.cert_hostnames?.length" class="text-xs text-slate-600 grid gap-1">
          <div class="flex flex-wrap gap-x-2">
            <dt class="text-slate-500">{{ t('ocserv.connectUrlCertNames') }}:</dt>
            <dd class="font-mono">{{ connectionInfo.cert_hostnames.join(', ') }}</dd>
          </div>
          <div v-if="connectionInfo.camouflage_enabled && connectionInfo.camouflage_secret" class="flex flex-wrap gap-x-2">
            <dt class="text-slate-500">{{ t('ocserv.connectUrlCamo') }}:</dt>
            <dd class="font-mono break-all">{{ connectionInfo.camouflage_secret }}</dd>
          </div>
        </dl>
      </div>
      <p v-if="!useOcctl" class="text-sm text-slate-600">
        {{ t('ocserv.occtlHint') }}
      </p>
      <div class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-3">
        <div v-for="c in overviewCards" :key="c.label" class="card p-4">
          <div class="text-xs text-slate-500 mb-1">{{ c.label }}</div>
          <div class="text-sm font-semibold break-all">{{ c.value }}</div>
        </div>
      </div>
      <button type="button" class="btn-secondary text-sm" @click="loadOverview">{{ t('common.refresh') }}</button>
    </div>

    <div v-if="activeTab === 'sessions'" class="card p-4">
      <div class="flex flex-wrap justify-between items-center gap-2 mb-3">
        <h3 class="font-medium">{{ t('ocserv.tabSessions') }}</h3>
        <button type="button" class="btn-secondary text-sm" @click="loadSessions">{{ t('common.refresh') }}</button>
      </div>
      <div class="flex flex-wrap gap-2 items-center mb-3">
        <input
          v-model="sessionSearch"
          type="search"
          class="input flex-1 min-w-[12rem] text-sm"
          :placeholder="t('ocserv.sessionsSearch')"
        />
        <span class="text-xs text-slate-500 whitespace-nowrap">
          {{ t('ocserv.sessionsTotal', { n: filteredSessions.length }) }}
          <template v-if="filteredSessions.length > SESSION_PAGE_SIZE">
            · {{ t('common.pageOf', { page: sessionPage, total: sessionPageCount }) }}
          </template>
        </span>
      </div>
      <p v-if="!useOcctl" class="text-sm text-slate-600 mb-3">{{ t('ocserv.occtlHint') }}</p>
      <p v-else-if="isRadius" class="text-sm text-slate-600 mb-3">{{ t('ocserv.sessionsRadiusTraffic') }}</p>
      <p v-if="useOcctl" class="text-xs text-slate-500 mb-3">{{ t('ocserv.colVhostHint') }}</p>
      <div class="table-wrap overflow-x-auto">
        <table class="data w-full text-sm">
          <thead>
            <tr>
              <th>{{ t('ocserv.colVhost') }}</th>
              <th>{{ t('ocserv.colUser') }}</th>
              <th>{{ t('ocserv.colId') }}</th>
              <th>{{ t('ocserv.colIp') }}</th>
              <th>{{ t('ocserv.colRemote') }}</th>
              <th>{{ t('ocserv.colDevice') }}</th>
              <th>{{ t('ocserv.colRx') }}</th>
              <th>{{ t('ocserv.colTx') }}</th>
              <th>Started</th>
              <th>{{ t('ocserv.colDuration') }}</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="s in paginatedSessions" :key="sessionId(s) ?? `row-${sessVal(s, 'VPN IPv4', 'IPv4', 'ip')}`">
              <td
                class="font-mono text-xs"
                :class="sessionVhostDomain(s) === 'unknown' ? 'text-slate-500' : 'text-violet-800'"
                :title="sessionVhostTitle(s) || undefined"
              >
                {{ sessionVhostDomain(s) }}
              </td>
              <td>{{ sessVal(s, 'Username', 'username') }}</td>
              <td class="font-mono">{{ sessVal(s, 'ID', 'id') }}</td>
              <td class="font-mono">{{ sessVal(s, 'VPN IPv4', 'IPv4', 'ip') }}</td>
              <td class="font-mono">{{ sessRemoteIp(s) }}</td>
              <td>{{ sessVal(s, 'Device', 'device') }}</td>
              <td>{{ sessTraffic(s, 'rx') }}</td>
              <td>{{ sessTraffic(s, 'tx') }}</td>
              <td class="whitespace-nowrap">{{ sessVal(s, 'Session started at', 'Last connected at', 'connected_at') }}</td>
              <td class="whitespace-nowrap text-slate-600">{{ sessOnlineDuration(s) }}</td>
              <td class="whitespace-nowrap space-x-2">
                <button
                  v-if="useOcctl && sessVal(s, 'Username', 'username')"
                  type="button"
                  class="text-emerald-700 text-sm"
                  @click="openUserTraffic(String(sessVal(s, 'Username', 'username')))"
                >
                  {{ t('ocserv.traffic') }}
                </button>
                <button
                  type="button"
                  class="text-red-600 text-sm disabled:opacity-40"
                  :disabled="sessionId(s) == null"
                  :title="sessionId(s) == null ? t('ocserv.disconnectNeedId') : undefined"
                  @click="disconnectSession(s)"
                >
                  {{ t('ocserv.disconnect') }}
                </button>
              </td>
            </tr>
            <tr v-if="!paginatedSessions.length">
              <td colspan="11" class="text-center text-slate-400 py-6">
                {{ sessions.length && sessionSearch.trim() ? t('ocserv.noSessionsMatch') : t('ocserv.noSessions') }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div
        v-if="filteredSessions.length > SESSION_PAGE_SIZE"
        class="flex flex-wrap items-center justify-center gap-2 mt-3 text-sm"
      >
        <button
          type="button"
          class="btn-secondary text-xs px-2 py-1"
          :disabled="sessionPage <= 1"
          @click="sessionPage = Math.max(1, sessionPage - 1)"
        >
          {{ t('common.prevPage') }}
        </button>
        <span class="text-slate-600">{{ t('common.pageOf', { page: sessionPage, total: sessionPageCount }) }}</span>
        <button
          type="button"
          class="btn-secondary text-xs px-2 py-1"
          :disabled="sessionPage >= sessionPageCount"
          @click="sessionPage = Math.min(sessionPageCount, sessionPage + 1)"
        >
          {{ t('common.nextPage') }}
        </button>
      </div>
      <p class="text-xs text-slate-500 mt-2">{{ t('ocserv.sessionsPoll', { n: SESSION_PAGE_SIZE }) }}</p>
    </div>

    <div v-if="cfg && activeTab === 'groups'" class="card p-4 space-y-4">
      <h3 class="font-medium">{{ t('ocserv.groupsTitle') }}</h3>
      <p class="text-xs text-slate-500">{{ t('ocserv.groupsHint') }}</p>
      <div v-if="isRadius" class="text-sm text-amber-800 bg-amber-50 border border-amber-100 rounded p-3">
        {{ t('ocserv.groupsRadius') }}
      </div>
      <template v-else>
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3 p-3 bg-slate-50/80 rounded border text-sm">
          <label class="sm:col-span-2">
            {{ t('ocserv.groupDir') }}
            <input v-model="cfg.config_per_group" class="input w-full mt-1 font-mono text-xs" />
            <span class="text-xs text-slate-500">config-per-group</span>
          </label>
          <label class="sm:col-span-2">
            {{ t('ocserv.groupDefault') }}
            <input v-model="cfg.default_group_config" class="input w-full mt-1 font-mono text-xs" />
            <span class="text-xs text-slate-500">default-group-config</span>
          </label>
          <label class="flex items-center gap-2 sm:col-span-2">
            <input v-model="cfg.auto_select_group" type="checkbox" />
            {{ t('ocserv.autoSelectGroup') }}
          </label>
        </div>
        <div class="flex gap-2 mb-2">
          <button type="button" class="btn-primary text-sm" @click="save">{{ t('ocserv.saveGroupGlobals') }}</button>
          <button type="button" class="btn-secondary text-sm" :disabled="!status?.installed" @click="apply">{{ t('common.saveAndApply') }}</button>
        </div>
        <div class="border rounded-lg p-4 space-y-3 bg-blue-50/30">
          <h4 class="text-sm font-medium">{{ editingGroup ? t('ocserv.editGroup', { name: editingGroup }) : t('ocserv.addGroup') }}</h4>
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-2 text-sm">
            <label>
              {{ t('ocserv.groupName') }}
              <input v-model="groupForm.name" class="input w-full mt-1 font-mono" :disabled="!!editingGroup" :placeholder="t('ocserv.groupNamePh')" />
            </label>
            <label>
              {{ t('ocserv.groupLabel') }}
              <input v-model="groupForm.label" class="input w-full mt-1" :placeholder="t('ocserv.groupLabelPh')" />
            </label>
            <label class="sm:col-span-2">
              {{ t('common.comment') }}
              <input v-model="groupForm.comment" class="input w-full mt-1" />
            </label>
            <label>
              {{ t('ocserv.groupNet') }}
              <input v-model="groupForm.ipv4_network" class="input w-full mt-1 font-mono" placeholder="10.10.1.0" />
            </label>
            <label>
              {{ t('ocserv.groupMask') }}
              <input v-model="groupForm.ipv4_netmask" class="input w-full mt-1 font-mono" placeholder="255.255.255.0" />
            </label>
            <label>
              MTU
              <input v-model.number="groupForm.mtu" type="number" class="input w-full mt-1" min="0" />
            </label>
            <label class="flex items-end gap-2 pb-1">
              <input v-model="groupForm.tunnel_all_dns" type="checkbox" />
              tunnel-all-dns
            </label>
            <label>
              {{ t('ocserv.downCapM') }}
              <input v-model.number="groupForm.down_mbps" type="number" class="input w-full mt-1" min="0" step="0.1" />
            </label>
            <label>
              {{ t('ocserv.upCapM') }}
              <input v-model.number="groupForm.up_mbps" type="number" class="input w-full mt-1" min="0" step="0.1" />
            </label>
            <label class="sm:col-span-2">
              {{ t('ocserv.dnsLines') }}
              <textarea v-model="groupFormDns" class="input w-full mt-1 font-mono text-xs" rows="2" />
            </label>
            <label class="sm:col-span-2">
              {{ t('ocserv.routesLines') }}
              <textarea v-model="groupFormRoutes" class="input w-full mt-1 font-mono text-xs" rows="2" />
            </label>
            <label class="sm:col-span-2">
              {{ t('ocserv.noRoutesLines') }}
              <textarea v-model="groupFormNoRoutes" class="input w-full mt-1 font-mono text-xs" rows="2" />
            </label>
          </div>
          <div class="flex gap-2">
            <button type="button" class="btn-primary text-sm" @click="saveGroup">{{ editingGroup ? t('ocserv.updateGroup') : t('ocserv.addGroup') }}</button>
            <button v-if="editingGroup" type="button" class="btn-secondary text-sm" @click="cancelEditGroup">{{ t('common.cancel') }}</button>
          </div>
        </div>
        <input
          v-model="groupSearch"
          type="search"
          class="input w-full max-w-md text-sm"
          :placeholder="t('ocserv.searchGroups')"
        />
        <div class="table-wrap overflow-x-auto">
          <table class="data w-full text-sm">
            <thead>
              <tr>
                <th>{{ t('ocserv.groupName') }}</th>
                <th>{{ t('ocserv.groupLabel') }}</th>
                <th>{{ t('ocserv.groupNet') }}</th>
                <th>{{ t('common.comment') }}</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="g in filteredGroups" :key="g.name">
                <td class="font-mono">{{ g.name }}</td>
                <td>{{ g.label || '—' }}</td>
                <td class="font-mono text-xs">{{ g.ipv4_network ? `${g.ipv4_network}/${g.ipv4_netmask || ''}` : '—' }}</td>
                <td>{{ g.comment || '—' }}</td>
                <td class="whitespace-nowrap space-x-2">
                  <button type="button" class="text-blue-600 text-sm" @click="startEditGroup(g)">{{ t('common.edit') }}</button>
                  <button type="button" class="text-red-600 text-sm" @click="delGroup(g.name)">{{ t('common.delete') }}</button>
                </td>
              </tr>
              <tr v-if="!filteredGroups.length">
                <td colspan="5" class="text-center text-slate-400 py-6">{{ t('ocserv.noGroups') }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </template>
    </div>

    <div v-if="cfg && activeTab === 'vhosts'" class="card p-4 space-y-4">
      <h3 class="font-medium">{{ t('ocserv.vhostsTitle') }}</h3>
      <p class="text-xs text-slate-500">{{ t('ocserv.vhostsListHint') }}</p>
      <div class="border rounded-lg p-4 space-y-3 bg-slate-50/50 max-w-xl">
        <h4 class="text-sm font-medium">{{ t('ocserv.addVhost') }}</h4>
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-2 text-sm">
          <label class="sm:col-span-2">
            {{ t('ocserv.vhostDomain') }}
            <input
              v-model="basicVhostForm.domain"
              class="input w-full mt-1 font-mono"
              placeholder="vpn.example.com"
            />
          </label>
          <label class="sm:col-span-2">
            {{ t('common.comment') }}
            <input v-model="basicVhostForm.comment" class="input w-full mt-1" />
          </label>
        </div>
        <p class="text-xs text-slate-500">{{ t('ocserv.vhostAddHint') }}</p>
        <button type="button" class="btn-primary text-sm" @click="addVhost">{{ t('common.add') }}</button>
      </div>
      <input
        v-model="vhostSearch"
        type="search"
        class="input w-full max-w-md text-sm"
        :placeholder="t('ocserv.searchVhosts')"
      />
      <div class="table-wrap overflow-x-auto">
        <table class="data w-full text-sm">
          <thead>
            <tr>
              <th>{{ t('ocserv.vhostDomain') }}</th>
              <th>{{ t('ocserv.colAuth') }}</th>
              <th>{{ t('ocserv.connectUrlTitle') }}</th>
              <th>{{ t('ocserv.groupNet') }}</th>
              <th>{{ t('common.comment') }}</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="v in filteredVhosts" :key="v.domain">
              <td class="font-mono">{{ v.domain }}</td>
              <td>{{ v.auth_method || t('ocserv.inherit') }}</td>
              <td class="font-mono text-xs max-w-[14rem] truncate" :title="vhostConnectUrl(v.domain)">
                {{ vhostConnectUrl(v.domain) || '—' }}
              </td>
              <td class="font-mono text-xs">{{ v.ipv4_network ? `${v.ipv4_network}/${v.ipv4_netmask || ''}` : '—' }}</td>
              <td>{{ v.comment || '—' }}</td>
              <td class="whitespace-nowrap space-x-2">
                <button type="button" class="text-blue-600 text-sm font-medium" @click="openVhostAdvanced(v)">
                  {{ t('ocserv.vhostAdvanced') }}
                </button>
                <button type="button" class="text-red-600 text-sm" @click="delVhost(v.domain)">{{ t('common.delete') }}</button>
              </td>
            </tr>
            <tr v-if="!filteredVhosts.length">
              <td colspan="6" class="text-center text-slate-400 py-6">{{ t('ocserv.noVhosts') }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <div v-if="activeTab === 'users'" class="card p-4">
      <h3 class="font-medium mb-2">{{ t('ocserv.usersTitle') }}</h3>
      <p v-if="isRadius" class="text-sm text-slate-600 mb-4">
        {{ t('ocserv.usersRadius') }}
      </p>
      <template v-else>
        <div v-if="editingUser" class="border rounded-lg p-4 mb-4 bg-blue-50/50 space-y-3">
          <h4 class="text-sm font-medium">{{ t('ocserv.editUser', { name: editingUser.username }) }}</h4>
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-2 text-sm">
            <label class="text-sm">
              {{ t('ocserv.newPassword') }}
              <input v-model="editingUser.password" type="password" class="input w-full mt-1" :placeholder="t('ocserv.passwordMinPh')" />
            </label>
            <label class="text-sm">
              {{ t('ocserv.colGroup') }}
              <select v-model="editingUser.group" class="input w-full mt-1">
                <option value="">{{ t('ocserv.groupDefaultOpt') }}</option>
                <option v-for="g in groupOptions" :key="g.name" :value="g.name">{{ g.label || g.name }}</option>
              </select>
            </label>
            <label class="text-sm sm:col-span-2">
              {{ t('common.comment') }}
              <input v-model="editingUser.comment" class="input w-full mt-1" />
            </label>
          </div>
          <div class="flex gap-2">
            <button type="button" class="btn-primary text-sm" @click="saveEditUser">{{ t('common.save') }}</button>
            <button type="button" class="btn-secondary text-sm" @click="cancelEditUser">{{ t('common.cancel') }}</button>
          </div>
        </div>
        <div class="grid grid-cols-1 sm:grid-cols-4 gap-2 mb-3 text-sm">
          <input v-model="userForm.username" class="input" :placeholder="t('ocserv.usernamePh')" />
          <input v-model="userForm.password" type="password" class="input" :placeholder="t('ocserv.passwordMinPh')" />
          <input v-model="userForm.comment" class="input" :placeholder="t('common.comment')" />
          <select v-model="userForm.group" class="input">
            <option value="">{{ t('ocserv.groupOptional') }}</option>
            <option v-for="g in groupOptions" :key="g.name" :value="g.name">{{ g.label || g.name }}</option>
          </select>
        </div>
        <button type="button" class="btn-secondary text-sm mb-3" @click="addUser">{{ t('ocserv.addUser') }}</button>
        <input
          v-model="userSearch"
          type="search"
          class="input w-full max-w-md text-sm mb-3"
          :placeholder="t('ocserv.searchUsers')"
        />
        <div class="table-wrap overflow-x-auto">
          <table class="data w-full text-sm">
            <thead>
              <tr>
                <th>{{ t('ocserv.colUser') }}</th>
                <th>{{ t('ocserv.colComment') }}</th>
                <th>{{ t('ocserv.colGroup') }}</th>
                <th>{{ t('ocserv.colTotalTraffic') }}</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="u in filteredUsers" :key="u.username">
                <td class="font-mono">{{ u.username }}</td>
                <td>{{ u.comment || '—' }}</td>
                <td>{{ u.group || '—' }}</td>
                <td class="font-mono text-slate-700">{{ formatBytes(userTotalBytes(u)) }}</td>
                <td class="whitespace-nowrap space-x-2">
                  <button type="button" class="text-blue-600 text-sm" @click="startEditUser(u)">{{ t('common.edit') }}</button>
                  <button type="button" class="text-red-600 text-sm" @click="delUser(u.username)">{{ t('common.delete') }}</button>
                  <button type="button" class="text-emerald-700 text-sm" @click="openUserTraffic(u.username)">{{ t('ocserv.traffic') }}</button>
                </td>
              </tr>
              <tr v-if="!filteredUsers.length">
                <td colspan="5" class="text-center text-slate-400 py-6">
                  {{ users.length && userSearch.trim() ? t('ocserv.noUsersMatch') : t('ocserv.noUsers') }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <p class="text-xs text-slate-500 mt-3">{{ t('ocserv.usersOcpasswd') }}</p>
      </template>
    </div>

    <div v-if="cfg && activeTab === 'config'" class="card p-4 space-y-4">
      <label class="flex items-center gap-2">
        <input v-model="cfg.enabled" type="checkbox" />
        {{ t('ocserv.enable') }}
      </label>

      <div>
        <span class="text-sm font-medium">{{ t('ocserv.authMethod') }}</span>
        <div class="flex gap-4 mt-2 text-sm">
          <label class="flex items-center gap-2"><input v-model="cfg.auth_method" type="radio" value="plain" /> {{ t('ocserv.authPlain') }}</label>
          <label class="flex items-center gap-2"><input v-model="cfg.auth_method" type="radio" value="radius" /> {{ t('ocserv.authRadius') }}</label>
        </div>
      </div>

      <div v-if="isRadius" class="border rounded-lg p-4 space-y-3 bg-slate-50/50">
        <div class="flex flex-wrap items-center gap-2">
          <h3 class="text-sm font-medium">{{ t('ocserv.radiusSection') }}</h3>
          <OCServRadiusHelpModal />
          <OCServRadiusChallengeHelpModal />
        </div>
        <label class="block text-sm">{{ t('ocserv.radiusServer') }} <input v-model="cfg.radius.server" class="input w-full mt-1" /></label>
        <div class="grid grid-cols-2 gap-4">
          <label class="text-sm">{{ t('ocserv.radiusAuthPort') }} <input v-model.number="cfg.radius.auth_port" type="number" class="input w-full mt-1" /></label>
          <label class="text-sm">{{ t('ocserv.radiusAcctPort') }} <input v-model.number="cfg.radius.acct_port" type="number" class="input w-full mt-1" /></label>
        </div>
        <label class="text-sm">
          {{ t('ocserv.radiusSecret') }}
          <input
            v-model="radiusSecret"
            type="password"
            class="input w-full mt-1"
            :placeholder="radiusSecretSet ? t('ocserv.radiusSecretPh') : t('ocserv.radiusSecretRequired')"
            autocomplete="new-password"
          />
          <span v-if="radiusSecretSet && !radiusSecret" class="text-xs text-green-700">{{ t('ocserv.radiusSecretSaved') }}</span>
        </label>
        <label class="text-sm">
          {{ t('ocserv.radiusNas') }}
          <input
            v-model="cfg.radius.nas_identifier"
            class="input w-full mt-1"
            :placeholder="t('ocserv.radiusNasPh')"
          />
          <span class="text-xs text-slate-500">{{ t('ocserv.radiusNasHint') }}</span>
        </label>
        <label class="flex gap-2 text-sm">
          <input v-model="cfg.radius.groupconfig" type="checkbox" />
          <span>
            {{ t('ocserv.radiusGroupconfigEnable') }}
            <span class="text-slate-500">{{ t('ocserv.radiusGroupconfigTag') }}</span>
          </span>
        </label>
        <label class="flex gap-2 text-sm">
          <input v-model="cfg.radius.acct_enabled" type="checkbox" /> {{ t('ocserv.radiusAcctEnable') }}
        </label>
        <label v-if="cfg.radius.acct_enabled" class="text-sm">
          {{ t('ocserv.radiusStats') }}
          <input v-model.number="cfg.radius.stats_report_time" type="number" class="input w-full mt-1 max-w-xs" />
          <span class="text-xs text-slate-500">stats-report-time</span>
        </label>
      </div>

      <div
        v-if="usesCiscoSvcCompatAnywhere()"
        class="text-sm text-amber-800 bg-amber-50 border border-amber-100 rounded p-3"
      >
        {{ t('ocserv.ciscoSvcUdp443Banner') }}
      </div>
      <div class="grid grid-cols-2 gap-4">
        <label class="text-sm">{{ t('ocserv.tcpPort') }} <input v-model.number="cfg.tcp_port" type="number" class="input w-full mt-1" :disabled="!cfg.advanced?.tcp" /></label>
        <label class="text-sm">
          {{ t('ocserv.udpPort') }}
          <input
            v-model.number="cfg.udp_port"
            type="number"
            class="input w-full mt-1"
            :disabled="!cfg.advanced?.udp"
            :class="usesCiscoSvcCompatAnywhere() && cfg.udp_port !== 443 ? 'border-amber-500' : ''"
          />
          <span v-if="usesCiscoSvcCompatAnywhere()" class="text-xs text-amber-700 block mt-1">
            {{ t('ocserv.udpPortCiscoSvcHint') }}
          </span>
        </label>
        <label class="text-sm">{{ t('ocserv.ipv4Net') }} <input v-model="cfg.ipv4_network" class="input w-full mt-1" /></label>
        <label class="text-sm">{{ t('ocserv.ipv4Mask') }} <input v-model="cfg.ipv4_netmask" class="input w-full mt-1" /></label>
        <label class="text-sm">{{ t('ocserv.device') }} <input v-model="cfg.device" class="input w-full mt-1" /></label>
        <label class="text-sm">{{ t('ocserv.maxClients') }} <input v-model.number="cfg.max_clients" type="number" class="input w-full mt-1" /></label>
        <label class="text-sm col-span-2">
          {{ t('ocserv.socketFile') }}
          <input v-model="cfg.socket_file" class="input w-full mt-1 font-mono text-xs" />
          <span class="text-xs text-slate-500">socket-file</span>
        </label>
      </div>

      <div class="grid grid-cols-1 sm:grid-cols-3 gap-4">
        <label class="text-sm">{{ t('ocserv.dnsLines') }}<textarea v-model="dnsText" class="input w-full mt-1 font-mono text-xs" rows="3" /></label>
        <label class="text-sm">{{ t('ocserv.pushRoutes') }}<textarea v-model="routesText" class="input w-full mt-1 font-mono text-xs" rows="3" placeholder="default" /><span class="text-xs text-slate-500">route</span></label>
        <label class="text-sm">{{ t('ocserv.excludeRoutes') }}<textarea v-model="noRoutesText" class="input w-full mt-1 font-mono text-xs" rows="3" /><span class="text-xs text-slate-500">no-route</span></label>
      </div>

      <div class="flex gap-2 pt-2">
        <button type="button" class="btn-primary" @click="save">{{ t('common.save') }}</button>
        <button type="button" class="btn-secondary" :disabled="!status?.installed" @click="apply">{{ t('common.saveAndApply') }}</button>
      </div>
    </div>

    <div v-if="cfg && activeTab === 'certs'" class="card p-4 space-y-4">
      <h3 class="text-sm font-medium">{{ t('ocserv.certsTitle') }}</h3>
      <p class="text-xs text-slate-500">{{ t('ocserv.certsHint') }}</p>
      <CertSelect
        v-model="cfg.managed_cert_id"
        :use-qosnat-tls="!!cfg.use_qosnat_tls"
        allow-qosnat-tls
        @update:use-qosnat-tls="cfg.use_qosnat_tls = $event"
      />
      <p class="text-xs text-slate-500">{{ t('certificates.managedCertHint') }}</p>
      <div v-if="showCustomCertPaths" class="grid grid-cols-2 gap-4">
        <label class="text-sm">{{ t('ocserv.serverCert') }} <input v-model="cfg.server_cert_path" class="input w-full mt-1 font-mono text-xs" /><span class="text-xs text-slate-500">server-cert</span></label>
        <label class="text-sm">{{ t('ocserv.serverKey') }} <input v-model="cfg.server_key_path" class="input w-full mt-1 font-mono text-xs" /><span class="text-xs text-slate-500">server-key</span></label>
        <label class="text-sm col-span-2">{{ t('ocserv.caCert') }}<input v-model="cfg.ca_cert_path" class="input w-full mt-1 font-mono text-xs" /><span class="text-xs text-slate-500">ca-cert</span></label>
      </div>
      <label class="text-sm">
        {{ t('ocserv.certUserOidLabel') }}
        <input v-model="cfg.advanced.cert_user_oid" class="input w-full mt-1 font-mono text-xs" />
        <span class="text-xs text-slate-500">{{ t('ocserv.certUserOidHint') }}</span>
      </label>
      <label class="text-sm">
        {{ t('ocserv.tlsPrioritiesLabel') }}
        <input v-model="cfg.advanced.tls_priorities" class="input w-full mt-1 font-mono text-xs" />
        <span class="text-xs text-slate-500">{{ t('ocserv.tlsPrioritiesHint') }}</span>
      </label>
      <div class="flex gap-2 pt-2">
        <button type="button" class="btn-primary" @click="save">{{ t('common.save') }}</button>
        <button type="button" class="btn-secondary" :disabled="!status?.installed" @click="apply">
          {{ t('common.saveAndApply') }}
        </button>
      </div>
    </div>

    <div v-if="cfg && activeTab === 'advanced'" class="card p-4 space-y-4">
      <h3 class="text-sm font-medium">{{ t('ocserv.advancedTitle') }}</h3>
      <p class="text-xs text-slate-500">{{ t('ocserv.advancedHint') }}</p>
      <p
        v-if="needsCiscoSvcUdp443"
        class="text-sm text-amber-800 bg-amber-50 border border-amber-100 rounded p-3"
      >
        {{ t('ocserv.ciscoSvcUdp443Banner') }}
      </p>
      <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
        <label
          v-for="f in featureToggles"
          :key="f.key"
          class="flex gap-2 text-sm p-2 border rounded cursor-pointer"
          :class="f.key === 'cisco_svc_client_compat' && cfg.advanced.cisco_svc_client_compat ? 'border-amber-300 bg-amber-50/40' : ''"
        >
          <input v-model="cfg.advanced[f.key]" type="checkbox" class="mt-0.5" />
          <span><span class="font-medium">{{ f.label }}</span><span class="block text-xs text-slate-500">{{ f.desc }}</span></span>
        </label>
      </div>
      <div v-if="cfg.advanced.camouflage" class="grid grid-cols-2 gap-4 p-3 bg-amber-50/50 rounded border border-amber-100">
        <label class="text-sm col-span-2">
          {{ t('ocserv.camoSecret') }}
          <input
            v-model="camouflageSecret"
            type="password"
            class="input w-full mt-1"
            :placeholder="camouflageSecretSet ? t('ocserv.camoSecretPhKeep') : t('ocserv.camoSecretPhRequired')"
            autocomplete="new-password"
          />
          <span class="text-xs text-slate-500">camouflage-secret</span>
        </label>
        <label class="text-sm col-span-2">
          {{ t('ocserv.camoRealm') }}
          <input v-model="cfg.advanced.camouflage_realm" class="input w-full mt-1" placeholder="Restricted Content" />
          <span class="text-xs text-slate-500">{{ t('ocserv.camoRealmHint') }}</span>
        </label>
      </div>
      <div class="grid grid-cols-2 gap-4 p-3 bg-slate-50/80 rounded border">
        <label class="text-sm">
          {{ t('ocserv.downCapM') }}
          <input v-model.number="downMbps" type="number" class="input w-full mt-1" min="0" step="0.1" />
          <span class="text-xs text-slate-500">{{ t('ocserv.capHint0') }} · tx-data-per-sec</span>
        </label>
        <label class="text-sm">
          {{ t('ocserv.upCapM') }}
          <input v-model.number="upMbps" type="number" class="input w-full mt-1" min="0" step="0.1" />
          <span class="text-xs text-slate-500">{{ t('ocserv.capHint0') }} · rx-data-per-sec</span>
        </label>
        <p class="text-xs text-slate-500 col-span-2">{{ t('ocserv.capDirectionHint') }}</p>
      </div>
      <div class="grid grid-cols-2 sm:grid-cols-3 gap-4">
        <label v-for="n in numericAdvanced" v-show="!n.show || n.show()" :key="n.key" class="text-sm">
          {{ n.label }}
          <input v-model.number="cfg.advanced[n.key]" type="number" class="input w-full mt-1" :min="n.min" :max="n.max" />
          <span v-if="n.desc" class="text-xs text-slate-500">{{ n.desc }}</span>
        </label>
      </div>
      <div class="grid grid-cols-2 gap-4">
        <label class="text-sm">
          {{ t('ocserv.rekeyMethod') }}
          <select v-model="cfg.advanced.rekey_method" class="input w-full mt-1">
            <option value="ssl">{{ t('ocserv.rekeySsl') }}</option>
            <option value="new-tunnel">{{ t('ocserv.rekeyTunnel') }}</option>
          </select>
          <span class="text-xs text-slate-500">rekey-method</span>
        </label>
        <label class="text-sm">
          {{ t('ocserv.defaultDomain') }}
          <input v-model="cfg.advanced.default_domain" class="input w-full mt-1" />
          <span class="text-xs text-slate-500">default-domain</span>
        </label>
      </div>
      <div class="flex gap-2 pt-2">
        <button type="button" class="btn-primary" @click="save">{{ t('common.save') }}</button>
        <button type="button" class="btn-secondary" :disabled="!status?.installed" @click="apply">
          {{ t('common.saveAndApply') }}
        </button>
      </div>
    </div>

    <!-- uninstall ocserv modal -->
    <Teleport to="body">
      <div
        v-if="showUninstallModal"
        class="fixed inset-0 z-[60] flex items-center justify-center p-4 bg-black/40"
        role="presentation"
        @click.self="closeUninstallModal"
      >
        <div
          class="bg-white rounded-xl shadow-xl w-full max-w-md border border-slate-200"
          role="dialog"
          aria-modal="true"
          aria-labelledby="ocserv-uninstall-title"
        >
          <div class="flex items-center justify-between px-4 py-3 border-b border-slate-100">
            <h3 id="ocserv-uninstall-title" class="font-medium text-slate-900">
              {{ t('ocserv.uninstallModalTitle') }}
            </h3>
            <button
              type="button"
              class="text-slate-500 hover:text-slate-800 text-xl leading-none px-2"
              :disabled="uninstallSubmitting"
              @click="closeUninstallModal"
            >
              ×
            </button>
          </div>
          <div class="p-4 space-y-3">
            <p class="text-sm text-slate-600">{{ t('ocserv.uninstallModalBody') }}</p>
            <label class="block text-sm text-slate-700">
              {{ t('ocserv.uninstallPasswordLabel', { user: adminUser }) }}
              <input
                v-model="uninstallPassword"
                type="password"
                autocomplete="current-password"
                class="input mt-1 w-full"
                :placeholder="t('ocserv.uninstallPasswordPh', { user: adminUser })"
                :disabled="uninstallSubmitting"
                @keydown.enter.prevent="confirmUninstallOcserv"
              >
            </label>
            <p v-if="uninstallErr" class="text-sm text-red-600">{{ uninstallErr }}</p>
            <div class="flex justify-end gap-2 pt-1">
              <button type="button" class="btn-secondary" :disabled="uninstallSubmitting" @click="closeUninstallModal">
                {{ t('common.cancel') }}
              </button>
              <button
                type="button"
                class="btn-primary bg-red-600 hover:bg-red-700 border-red-600"
                :disabled="uninstallSubmitting"
                @click="confirmUninstallOcserv"
              >
                {{ uninstallSubmitting ? t('common.loading') : t('ocserv.uninstallConfirm') }}
              </button>
            </div>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- traffic modal -->
    <Teleport to="body">
      <div
        v-if="trafficModal"
        class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/40"
        @click.self="closeUserTraffic"
      >
        <div
          class="bg-white rounded-xl shadow-xl w-full max-w-2xl max-h-[90vh] overflow-y-auto border border-slate-200"
          role="dialog"
          aria-labelledby="traffic-modal-title"
        >
          <div class="flex items-center justify-between px-4 py-3 border-b border-slate-100 sticky top-0 bg-white z-10">
            <h3 id="traffic-modal-title" class="font-medium">
              {{ t('ocserv.trafficTitle', { user: trafficModal }) }}
            </h3>
            <button type="button" class="text-slate-500 hover:text-slate-800 text-xl leading-none px-2" @click="closeUserTraffic">×</button>
          </div>
          <div class="p-4 space-y-4">
            <div class="flex flex-wrap items-center gap-2 text-sm">
              <span class="text-slate-600">{{ t('ocserv.trafficRange') }}</span>
              <select v-model="trafficPeriod" class="input text-sm py-1 w-auto" :disabled="trafficLiveEnabled">
                <option value="24h">{{ t('ocserv.period24h') }}</option>
                <option value="7d">{{ t('ocserv.period7d') }}</option>
                <option value="30d">{{ t('ocserv.period30d') }}</option>
                <option value="365d">{{ t('ocserv.period365d') }}</option>
              </select>
              <span
                v-if="trafficData"
                class="text-xs px-2 py-0.5 rounded"
                :class="trafficData.online ? 'bg-emerald-100 text-emerald-800' : 'bg-slate-100 text-slate-600'"
              >
                {{ trafficData.online ? t('ocserv.onlineNow') : t('ocserv.offlineNow') }}
              </span>
              <span
                v-if="trafficData && trafficLastUpdated"
                class="text-xs text-slate-500 ml-auto"
              >
                {{ t('ocserv.trafficLastUpdated', { time: formatTrafficUpdatedAt(trafficLastUpdated) }) }}
              </span>
            </div>
            <p v-if="trafficErr" class="text-sm text-red-600">{{ trafficErr }}</p>
            <p v-else-if="trafficLoading" class="text-sm text-slate-500">{{ t('common.loading') }}</p>
            <template v-else-if="trafficData">
              <div class="grid grid-cols-2 sm:grid-cols-4 gap-2 text-sm">
                <div class="rounded-lg border p-2 bg-slate-50">
                  <p class="text-xs text-slate-500">{{ t('ocserv.todayRx') }}</p>
                  <p class="font-mono font-medium">{{ formatBytes(trafficData.summary?.today_rx_bytes) }}</p>
                </div>
                <div class="rounded-lg border p-2 bg-slate-50">
                  <p class="text-xs text-slate-500">{{ t('ocserv.todayTx') }}</p>
                  <p class="font-mono font-medium">{{ formatBytes(trafficData.summary?.today_tx_bytes) }}</p>
                </div>
                <div class="rounded-lg border p-2 bg-slate-50">
                  <p class="text-xs text-slate-500">{{ t('ocserv.periodRx') }}</p>
                  <p class="font-mono font-medium">{{ formatBytes(trafficData.summary?.period_rx_bytes) }}</p>
                </div>
                <div class="rounded-lg border p-2 bg-slate-50">
                  <p class="text-xs text-slate-500">{{ t('ocserv.periodTx') }}</p>
                  <p class="font-mono font-medium">{{ formatBytes(trafficData.summary?.period_tx_bytes) }}</p>
                </div>
                <div class="rounded-lg border p-2 bg-emerald-50/50 sm:col-span-2">
                  <p class="text-xs text-slate-500">{{ t('ocserv.totalRx') }}</p>
                  <p class="font-mono font-medium text-emerald-800">{{ formatBytes(trafficData.summary?.total_rx_bytes) }}</p>
                </div>
                <div class="rounded-lg border p-2 bg-sky-50/50 sm:col-span-2">
                  <p class="text-xs text-slate-500">{{ t('ocserv.totalTx') }}</p>
                  <p class="font-mono font-medium text-sky-800">{{ formatBytes(trafficData.summary?.total_tx_bytes) }}</p>
                </div>
              </div>
              <div v-if="trafficData.online && trafficData.current" class="text-sm rounded border border-emerald-100 bg-emerald-50/40 p-3">
                <p class="text-xs text-slate-600 mb-1">{{ t('ocserv.currentSession') }}</p>
                <p>
                  RX {{ formatTraffic(trafficData.current._RX, trafficData.current.RX ?? trafficData.current.rx) }}
                  · TX {{ formatTraffic(trafficData.current._TX, trafficData.current.TX ?? trafficData.current.tx) }}
                </p>
              </div>
              <div class="space-y-2">
                <div class="flex flex-wrap items-center gap-3 text-sm rounded-lg border border-slate-200 bg-slate-50/80 px-3 py-2">
                  <label class="flex items-center gap-2 cursor-pointer select-none">
                    <input
                      v-model="trafficLiveEnabled"
                      type="checkbox"
                      class="rounded"
                      :disabled="!useOcctl || !trafficData.online"
                      @change="onTrafficLiveToggle"
                    />
                    <span class="font-medium text-slate-700">{{ t('ocserv.trafficLive') }}</span>
                  </label>
                  <span v-if="trafficLiveEnabled" class="text-xs text-emerald-700">
                    {{ t('ocserv.trafficLiveActive') }}
                  </span>
                  <span v-else-if="!trafficData.online" class="text-xs text-slate-500">
                    {{ t('ocserv.trafficLiveNeedOnline') }}
                  </span>
                  <span v-else-if="!useOcctl" class="text-xs text-slate-500">
                    {{ t('ocserv.occtlHint') }}
                  </span>
                </div>
                <p v-if="trafficLiveErr" class="text-xs text-amber-700">{{ trafficLiveErr }}</p>
                <p v-else-if="trafficLiveEnabled && trafficLiveSeries.length < 2" class="text-xs text-slate-500">
                  {{ t('ocserv.trafficLiveWarming') }}
                </p>
                <SnmpTrafficChart
                  :series="trafficChartSeries"
                  :empty-label="trafficLiveEnabled ? t('ocserv.trafficLiveWarming') : undefined"
                  :footer-label="trafficLiveEnabled ? t('ocserv.trafficLiveFooter') : undefined"
                />
              </div>
            </template>
            <p class="text-xs text-slate-500">
              {{ t('ocserv.trafficFoot') }} · {{ t('ocserv.trafficAutoRefresh') }}
            </p>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>
