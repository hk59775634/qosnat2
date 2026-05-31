<script setup>
import { computed, nextTick, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/api/client'
import { setDisplayName } from '@/composables/useBranding'
import PageTabs from '@/components/PageTabs.vue'
import CertSelect from '@/components/CertSelect.vue'

const { t } = useI18n()
const cfg = ref(null)
const form = ref({
  hostname: '',
  display_name: '',
  admin_port: '',
  new_password: '',
  current_password: '',
  tls_enabled: false,
  tls_cert: '',
  tls_key: '',
  tls_domain: '',
  tls_acme_enabled: false,
  tls_acme_email: '',
  tls_acme_staging: false,
  tls_acme_renew_days: 30,
  tls_managed_cert_id: '',
})
const err = ref('')
const ok = ref('')
const warn = ref('')
const acmeBusy = ref(false)
const versionInfo = ref(null)
const switchTag = ref('')
const versionSwitchJob = ref(null)
const versionSwitchPoll = ref(null)
const versionSwitchPollErrs = ref(0)
const versionSwitchModalOpen = ref(false)
const versionSwitchPassword = ref('')
const versionSwitchModalErr = ref('')
const versionSwitchSubmitting = ref(false)
const versionSwitchPasswordRef = ref(null)
const importFile = ref(null)
const importConfirm = ref(false)
const backupBusy = ref(false)

const activeTab = ref('basic')
const generalTabs = computed(() => [
  { id: 'basic', label: t('system.general.tabBasic') },
  { id: 'version', label: t('system.general.tabVersion') },
  { id: 'tls', label: t('system.general.tabTls') },
])

const versionSwitchRunning = computed(() => versionSwitchJob.value?.state === 'running')

const versionSwitchPanelVisible = computed(() => {
  const j = versionSwitchJob.value
  if (!j) return false
  return j.state === 'running' || j.state === 'failed' || j.state === 'ok'
})

function normalizeVersionSwitchJob(job) {
  if (!job || job.state === 'idle') return null
  return job
}

function applyVersionSwitchJob(job) {
  const j = normalizeVersionSwitchJob(job)
  versionSwitchJob.value = j
  if (!j) return
  if (j.state === 'running' && !versionSwitchPoll.value) {
    startVersionSwitchPoll()
    return
  }
  if (j.state === 'failed') {
    err.value = j.message || t('system.general.versionSwitchFailed')
    ok.value = ''
  } else if (j.state === 'ok') {
    const msg = j.result?.message || j.message || t('system.general.versionSwitchSuccess')
    ok.value = `${t('system.general.versionSwitchSuccess')} (${j.target_tag || j.result?.tag || ''}) — ${msg}`
    err.value = ''
  }
}

async function loadVersionInfo() {
  try {
    const v = await api.system.version.get()
    versionInfo.value = v
    applyVersionSwitchJob(v?.switch_task)
    return v
  } catch {
    versionInfo.value = null
    return null
  }
}

async function load() {
  cfg.value = await api.system.general.get()
  const v = await loadVersionInfo()
  const tlsCfg = cfg.value.tls || {}
  form.value.hostname = cfg.value.hostname || ''
  form.value.display_name = cfg.value.display_name || ''
  form.value.admin_port = cfg.value.admin_port || ''
  form.value.tls_enabled = tlsCfg.tls_enabled ?? false
  form.value.tls_domain = tlsCfg.domain || ''
  form.value.tls_acme_enabled = tlsCfg.acme_enabled ?? false
  form.value.tls_acme_email = tlsCfg.acme_email || ''
  form.value.tls_acme_staging = tlsCfg.acme_staging ?? false
  form.value.tls_acme_renew_days = tlsCfg.acme_renew_days || 30
  form.value.tls_managed_cert_id = tlsCfg.managed_cert_id || ''
  form.value.tls_cert = ''
  form.value.tls_key = ''
  if (!switchTag.value) {
    switchTag.value = v?.current_tag || v?.releases?.[0]?.tag || ''
  }
}

function stopVersionSwitchPoll() {
  if (versionSwitchPoll.value) {
    clearInterval(versionSwitchPoll.value)
    versionSwitchPoll.value = null
  }
}

function startVersionSwitchPoll() {
  stopVersionSwitchPoll()
  versionSwitchPollErrs.value = 0
  versionSwitchPoll.value = setInterval(async () => {
    try {
      const j = await api.system.version.switchStatus()
      versionSwitchPollErrs.value = 0
      versionSwitchJob.value = normalizeVersionSwitchJob(j) || j
      if (j.state === 'idle') {
        stopVersionSwitchPoll()
        versionSwitchJob.value = null
        await loadVersionInfo()
        return
      }
      if (j.state === 'ok') {
        stopVersionSwitchPoll()
        const msg = j.result?.message || j.message || t('system.general.versionSwitchSuccess')
        ok.value = `${t('system.general.versionSwitchSuccess')} (${j.target_tag || j.result?.tag || ''}) — ${msg}`
        err.value = ''
        versionSwitchJob.value = null
        await loadVersionInfo()
      } else if (j.state === 'failed') {
        stopVersionSwitchPoll()
        err.value = j.message || t('system.general.versionSwitchFailed')
        ok.value = ''
      }
    } catch {
      versionSwitchPollErrs.value += 1
      if (versionSwitchPollErrs.value >= 15) {
        stopVersionSwitchPoll()
        err.value = t('system.general.versionSwitchStatusLost')
      }
    }
  }, 2000)
}

const useLibraryCert = computed(() => !!form.value.tls_managed_cert_id)

function readFile(e, field) {
  const f = e.target?.files?.[0]
  if (!f) return
  const reader = new FileReader()
  reader.onload = () => {
    form.value[field] = String(reader.result || '')
  }
  reader.readAsText(f)
  e.target.value = ''
}

function buildBasicPutBody() {
  return {
    hostname: form.value.hostname,
    display_name: form.value.display_name,
    admin_port: form.value.admin_port || undefined,
    new_password: form.value.new_password || undefined,
    current_password: form.value.current_password || undefined,
  }
}

function buildTlsPutBody() {
  return {
    current_password: form.value.current_password || undefined,
    tls_enabled: form.value.tls_enabled,
    tls_domain: form.value.tls_domain,
    tls_acme_enabled: form.value.tls_acme_enabled,
    tls_acme_email: form.value.tls_acme_email,
    tls_acme_staging: form.value.tls_acme_staging,
    tls_acme_renew_days: form.value.tls_acme_renew_days,
    tls_managed_cert_id: form.value.tls_managed_cert_id || '',
    tls_cert: form.value.tls_cert.trim() || undefined,
    tls_key: form.value.tls_key.trim() || undefined,
  }
}

function adminPortTouched() {
  const cur = String(cfg.value?.admin_port || '')
  return String(form.value.admin_port || '') !== cur && String(form.value.admin_port || '') !== ''
}

function tlsSettingsTouched() {
  const tlsCfg = cfg.value?.tls || {}
  return (
    form.value.tls_enabled !== (tlsCfg.tls_enabled ?? false) ||
    form.value.tls_domain !== (tlsCfg.domain || '') ||
    form.value.tls_acme_enabled !== (tlsCfg.acme_enabled ?? false) ||
    form.value.tls_acme_email !== (tlsCfg.acme_email || '') ||
    form.value.tls_acme_staging !== (tlsCfg.acme_staging ?? false) ||
    form.value.tls_acme_renew_days !== (tlsCfg.acme_renew_days || 30) ||
    form.value.tls_managed_cert_id !== (tlsCfg.managed_cert_id || '') ||
    form.value.tls_cert.trim() !== '' ||
    form.value.tls_key.trim() !== ''
  )
}

async function save() {
  err.value = ''
  ok.value = ''
  warn.value = ''
  try {
    const needPw =
      activeTab.value === 'tls'
        ? tlsSettingsTouched() &&
          (form.value.tls_enabled || form.value.tls_acme_enabled || form.value.tls_cert)
        : form.value.new_password || adminPortTouched()
    if (needPw && !form.value.current_password) {
      err.value = t('system.general.needPasswordForTls')
      return
    }
    const res = await api.system.general.put(activeTab.value === 'tls' ? buildTlsPutBody() : buildBasicPutBody())
    ok.value = t('system.general.saved')
    if (res.warning) warn.value = res.warning
    if (res.admin_port) form.value.admin_port = res.admin_port
    form.value.new_password = ''
    form.value.current_password = ''
    form.value.tls_cert = ''
    form.value.tls_key = ''
    setDisplayName(form.value.display_name)
    await load()
  } catch (e) {
    err.value = e.data?.error || e.message
  }
}

async function exportState() {
  err.value = ''
  ok.value = ''
  backupBusy.value = true
  try {
    const res = await fetch(api.system.state.exportUrl(), { credentials: 'include' })
    if (!res.ok) {
      let msg = res.statusText
      try {
        const j = await res.json()
        msg = j.error || msg
      } catch {
        /* ignore */
      }
      throw new Error(msg)
    }
    const blob = await res.blob()
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `qosnat2-state-${new Date().toISOString().slice(0, 10)}.json`
    document.body.appendChild(a)
    a.click()
    a.remove()
    URL.revokeObjectURL(url)
    ok.value = t('system.general.stateExported')
  } catch (e) {
    err.value = e.message
  } finally {
    backupBusy.value = false
  }
}

async function importState() {
  err.value = ''
  ok.value = ''
  warn.value = ''
  if (!importConfirm.value) {
    err.value = t('system.general.importNeedConfirm')
    return
  }
  if (!form.value.current_password) {
    err.value = t('system.general.importNeedPassword')
    return
  }
  const file = importFile.value?.files?.[0]
  if (!file) {
    err.value = t('system.general.importNeedFile')
    return
  }
  backupBusy.value = true
  try {
    const text = await file.text()
    const state = JSON.parse(text)
    const res = await api.system.state.import({
      current_password: form.value.current_password,
      state,
    })
    ok.value = t('system.general.stateImported')
    if (res.warning) warn.value = res.warning
    form.value.current_password = ''
    importConfirm.value = false
    if (importFile.value) importFile.value.value = ''
    await load()
  } catch (e) {
    err.value = e.data?.error || e.message
  } finally {
    backupBusy.value = false
  }
}

async function renewLibraryCert() {
  if (!form.value.tls_managed_cert_id) return
  if (!form.value.current_password) {
    err.value = t('system.general.needPasswordForTls')
    return
  }
  acmeBusy.value = true
  err.value = ''
  ok.value = ''
  try {
    await api.system.certificates.renew(form.value.tls_managed_cert_id)
    ok.value = t('system.general.certRenewed')
    await load()
  } catch (e) {
    err.value = e.data?.error || e.message
    await load()
  } finally {
    acmeBusy.value = false
  }
}

async function runAcme(action) {
  err.value = ''
  ok.value = ''
  warn.value = ''
  if (!form.value.current_password) {
    err.value = t('system.general.needPasswordForTls')
    return
  }
  acmeBusy.value = true
  try {
    await api.system.general.put(buildTlsPutBody())
    const res = await api.post('/api/v1/system/tls/acme', {
      action,
      current_password: form.value.current_password,
    })
    ok.value =
      res.message ||
      (action === 'obtain' ? t('system.general.certRequested') : t('system.general.certRenewed'))
    if (res.warning) warn.value = res.warning
    await load()
  } catch (e) {
    err.value = e.data?.error || e.message
    await load()
  } finally {
    acmeBusy.value = false
  }
}

function openVersionSwitchModal() {
  if (versionSwitchRunning.value) return
  if (!switchTag.value) {
    err.value = t('system.general.versionNeedTag')
    return
  }
  if (versionInfo.value?.current_tag && switchTag.value === versionInfo.value.current_tag) {
    err.value = t('system.general.versionAlreadyCurrent')
    return
  }
  versionSwitchModalErr.value = ''
  versionSwitchPassword.value = ''
  versionSwitchModalOpen.value = true
  nextTick(() => versionSwitchPasswordRef.value?.focus())
}

function resetVersionSwitchModal() {
  versionSwitchModalOpen.value = false
  versionSwitchPassword.value = ''
  versionSwitchModalErr.value = ''
}

function closeVersionSwitchModal() {
  if (versionSwitchSubmitting.value) return
  resetVersionSwitchModal()
}

async function confirmVersionSwitch() {
  versionSwitchModalErr.value = ''
  err.value = ''
  ok.value = ''
  warn.value = ''
  if (versionSwitchSubmitting.value || versionSwitchRunning.value) return
  if (!switchTag.value) {
    versionSwitchModalErr.value = t('system.general.versionNeedTag')
    return
  }
  if (!versionSwitchPassword.value) {
    versionSwitchModalErr.value = t('system.general.needPasswordForVersionSwitch')
    return
  }
  const passwd = versionSwitchPassword.value
  versionSwitchSubmitting.value = true
  try {
    await api.system.version.switchVerify({ current_password: passwd })
    const r = await api.system.version.switch({ tag: switchTag.value })
    resetVersionSwitchModal()
    const job = r?.job
    if (job?.state === 'ok') {
      applyVersionSwitchJob(job)
      await loadVersionInfo()
      return
    }
    ok.value = r.message || t('system.general.versionSwitchQueued')
    applyVersionSwitchJob(job?.state ? job : { state: 'running', target_tag: switchTag.value })
  } catch (e) {
    versionSwitchModalErr.value = e.data?.error || e.message
    if (e.data?.job) {
      resetVersionSwitchModal()
      applyVersionSwitchJob(e.data.job)
    }
  } finally {
    versionSwitchSubmitting.value = false
  }
}

async function dismissVersionSwitchTask() {
  if (versionSwitchSubmitting.value) return
  err.value = ''
  ok.value = ''
  try {
    const res = await api.system.version.switchReset()
    stopVersionSwitchPoll()
    versionSwitchJob.value = normalizeVersionSwitchJob(res?.job)
    ok.value = t('system.general.versionSwitchDismissed')
    await loadVersionInfo()
  } catch (e) {
    err.value = e.data?.error || e.message
  }
}

onMounted(load)
onUnmounted(stopVersionSwitchPoll)
</script>

<template>
  <div class="page-stack">
    <PageHeader
      :title="t('system.general.title')"
      :description="t('system.general.description')"
      :ok="ok"
      :warn="warn"
      :err="err"
    />
    <PageTabs v-model="activeTab" :tabs="generalTabs" />

    <div v-if="cfg" class="card card-body space-y-4">
      <section v-show="activeTab === 'basic'" class="space-y-4">
        <h3 class="text-sm font-semibold text-slate-800">{{ t('system.general.basic') }}</h3>
        <p class="text-sm text-slate-600">
          {{ t('system.general.adminSection') }}
          <span class="font-mono">{{ cfg.admin_user }}</span> · LAN
          <span class="font-mono">{{ cfg.dev_lan }}</span> · WAN
          <span class="font-mono">{{ cfg.dev_wan }}</span>
        </p>
        <div>
          <label class="text-xs text-slate-500">{{ t('system.general.hostname') }}</label>
          <input v-model="form.hostname" class="input-field mt-1 font-mono" />
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('system.general.displayName') }}</label>
          <input v-model="form.display_name" class="input-field mt-1" maxlength="64" :placeholder="t('system.general.displayNameDefault')" />
          <p class="text-xs text-slate-500 mt-1">{{ t('system.general.displayNameHint') }}</p>
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('system.general.adminPort') }}</label>
          <input
            v-model="form.admin_port"
            type="number"
            min="1"
            max="65535"
            class="input-field mt-1 font-mono w-32"
          />
          <p class="text-xs text-slate-500 mt-1">{{ t('system.general.adminPortHint') }}</p>
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('system.general.newPassword') }}</label>
          <input v-model="form.new_password" type="password" class="input-field mt-1" autocomplete="new-password" />
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('system.general.currentPassword') }}</label>
          <input v-model="form.current_password" type="password" class="input-field mt-1" autocomplete="current-password" />
        </div>
        <div class="border-t border-slate-200 pt-4 space-y-3">
          <h4 class="text-sm font-semibold text-slate-800">{{ t('system.general.backupSection') }}</h4>
          <p class="text-xs text-slate-500">{{ t('system.general.backupHint') }}</p>
          <div class="flex flex-wrap gap-2">
            <button type="button" class="btn-secondary text-sm" :disabled="backupBusy" @click="exportState">
              {{ t('system.general.exportState') }}
            </button>
          </div>
          <div class="space-y-2">
            <input ref="importFile" type="file" accept="application/json,.json" class="text-sm" />
            <label class="flex items-start gap-2 text-sm text-slate-700 cursor-pointer">
              <input v-model="importConfirm" type="checkbox" class="mt-1" />
              <span>{{ t('system.general.importConfirm') }}</span>
            </label>
            <button type="button" class="btn-secondary text-sm" :disabled="backupBusy" @click="importState">
              {{ t('system.general.importState') }}
            </button>
          </div>
        </div>
      </section>

      <section v-show="activeTab === 'version'" class="space-y-4">
        <h3 class="text-sm font-semibold text-slate-800">{{ t('system.general.versionSection') }}</h3>
        <p class="text-xs text-slate-500">{{ t('system.general.versionFormatHint') }}</p>
        <div v-if="versionInfo" class="text-xs text-slate-600 space-y-1 bg-slate-50 rounded p-3">
          <p>{{ t('system.general.currentVersion') }}: <span class="font-mono">{{ versionInfo.current_version || 'unknown' }}</span></p>
          <p>{{ t('system.general.currentTag') }}: <span class="font-mono">{{ versionInfo.current_tag || 'unknown' }}</span></p>
          <p>{{ t('system.general.binaryPath') }}: <span class="font-mono">{{ versionInfo.binary_path }}</span></p>
          <p v-if="versionInfo.list_error" class="text-amber-700">{{ versionInfo.list_error }}</p>
        </div>
        <div class="flex flex-wrap gap-2 items-end">
          <label class="flex-1 min-w-[16rem]">
            <span class="text-xs text-slate-500">{{ t('system.general.switchToVersion') }}</span>
            <select v-model="switchTag" class="input-field mt-1 font-mono" :disabled="versionSwitchRunning || !versionInfo?.root_required">
              <option v-for="r in (versionInfo?.releases || [])" :key="r.tag" :value="r.tag">
                {{ r.tag }}{{ r.prerelease ? ' (pre)' : '' }}
              </option>
            </select>
          </label>
          <button
            type="button"
            class="btn-secondary"
            :disabled="versionSwitchRunning || !versionInfo?.root_required"
            @click="openVersionSwitchModal"
          >
            {{ versionSwitchRunning ? t('system.general.versionSwitching') : t('system.general.switchVersion') }}
          </button>
          <button
            type="button"
            class="btn-secondary"
            :disabled="versionSwitchRunning"
            @click="loadVersionInfo"
          >
            {{ t('common.refresh') }}
          </button>
        </div>
        <div
          v-if="versionSwitchPanelVisible"
          class="p-3 rounded border text-xs space-y-2"
          :class="versionSwitchJob?.state === 'failed' ? 'border-red-200 bg-red-50' : versionSwitchJob?.state === 'ok' ? 'border-green-200 bg-green-50' : 'border-slate-200 bg-slate-50'"
        >
          <p class="text-sm">
            {{ t('system.general.versionSwitchTask') }}:
            <strong>{{ versionSwitchJob?.target_tag || switchTag }}</strong>
            · <strong>{{ versionSwitchJob?.state }}</strong>
          </p>
          <p v-if="versionSwitchJob?.message" class="text-slate-600">{{ versionSwitchJob.message }}</p>
          <p v-if="versionSwitchJob?.state === 'ok'" class="text-green-700">
            {{ t('system.general.versionSwitchSuccess') }}:
            {{ versionSwitchJob.result?.message || versionSwitchJob.message }}
          </p>
          <p v-if="versionSwitchJob?.state === 'failed'" class="text-red-700">
            {{ t('system.general.versionSwitchFailed') }}: {{ versionSwitchJob.message }}
          </p>
          <button
            v-if="!versionSwitchSubmitting"
            type="button"
            class="btn-secondary text-xs"
            @click="dismissVersionSwitchTask"
          >
            {{ t('system.general.versionSwitchDismiss') }}
          </button>
        </div>
        <p v-if="versionInfo && !versionInfo.root_required" class="text-xs text-amber-700">
          {{ t('system.general.versionRootHint') }}
        </p>
      </section>

      <section v-show="activeTab === 'tls'" class="space-y-4">
        <h3 class="text-sm font-semibold text-slate-800">{{ t('system.general.httpsSection') }}</h3>
        <div v-if="cfg.tls" class="text-xs text-slate-600 space-y-1 bg-slate-50 rounded p-3">
          <p>
            <span :class="cfg.tls.tls_active ? 'text-green-700' : 'text-slate-500'">
              {{ cfg.tls.tls_active ? t('system.general.tlsRunning') : t('system.general.tlsHttp') }}
            </span>
            · {{ cfg.tls.tls_enabled ? t('common.on') : t('common.off') }}
          </p>
          <p v-if="cfg.tls.has_cert_file">{{ t('system.general.certPath') }}: {{ cfg.tls.cert_subject || cfg.tls.cert_path }}</p>
          <p v-if="cfg.tls.cert_not_after">{{ t('system.general.certExpiry') }}: {{ cfg.tls.cert_not_after }}</p>
          <p v-if="cfg.tls.acme_last_ok">{{ t('system.general.acmeOk') }}: {{ cfg.tls.acme_last_ok }}</p>
          <p v-if="cfg.tls.acme_last_error" class="text-red-600">{{ t('system.general.acmeError') }}: {{ cfg.tls.acme_last_error }}</p>
        </div>

        <label class="flex items-center gap-2 text-sm cursor-pointer">
          <input v-model="form.tls_enabled" type="checkbox" class="rounded" />
          {{ t('system.general.enableHttps') }}
        </label>

        <CertSelect
          v-model="form.tls_managed_cert_id"
          allow-other-source
          :disabled="!form.tls_enabled"
        />
        <p class="text-xs text-slate-500">{{ t('system.general.tlsLibraryHint') }}</p>

        <div v-if="useLibraryCert && form.tls_enabled" class="space-y-2 p-3 rounded border border-slate-200 bg-slate-50">
          <p class="text-xs text-slate-600">
            {{ t('system.general.tlsUsingLibrary') }}
            <router-link to="/system/certificates" class="text-blue-600 hover:underline">{{ t('certificates.openManager') }}</router-link>
          </p>
          <button
            v-if="cfg.tls?.acme_enabled"
            type="button"
            class="btn-secondary text-sm"
            :disabled="acmeBusy"
            @click="renewLibraryCert"
          >
            {{ acmeBusy ? t('common.processing') : t('system.general.acmeRenewNow') }}
          </button>
        </div>

        <template v-if="!useLibraryCert">
        <label class="flex items-center gap-2 text-sm cursor-pointer">
          <input v-model="form.tls_acme_enabled" type="checkbox" class="rounded" :disabled="!form.tls_enabled" />
          {{ t('system.general.acmeEnable') }}
        </label>

        <div v-if="form.tls_acme_enabled" class="space-y-3 p-4 rounded-lg border border-blue-100 bg-blue-50/40">
          <p class="text-xs text-slate-600">{{ t('system.general.acmeHttp01') }}</p>
          <div>
            <label class="text-xs text-slate-500">{{ t('system.general.acmeDomain') }}</label>
            <input v-model="form.tls_domain" class="input-field mt-1 font-mono" placeholder="vpn.example.com" />
          </div>
          <div>
            <label class="text-xs text-slate-500">{{ t('system.general.acmeEmail') }}</label>
            <input v-model="form.tls_acme_email" type="email" class="input-field mt-1" placeholder="admin@example.com" />
          </div>
          <label class="flex items-center gap-2 text-sm">
            <input v-model="form.tls_acme_staging" type="checkbox" class="rounded" />
            {{ t('system.general.acmeStaging') }}
          </label>
          <div>
            <label class="text-xs text-slate-500">{{ t('system.general.acmeAutoRenew') }}</label>
            <input v-model.number="form.tls_acme_renew_days" type="number" min="7" max="60" class="input-field mt-1 max-w-xs" />
          </div>
          <div class="flex flex-wrap gap-2">
            <button
              type="button"
              class="btn-secondary text-sm"
              :disabled="acmeBusy || !form.tls_domain"
              @click="runAcme('obtain')"
            >
              {{ acmeBusy ? t('common.processing') : t('system.general.acmeRequest') }}
            </button>
            <button
              type="button"
              class="btn-secondary text-sm"
              :disabled="acmeBusy || !cfg.tls.has_cert_file"
              @click="runAcme('renew')"
            >
              {{ t('system.general.acmeRenewNow') }}
            </button>
          </div>
        </div>

        <template v-else>
          <p class="text-xs text-slate-500">{{ t('system.general.manualPem') }}</p>
          <div>
            <label class="text-xs text-slate-500">{{ t('system.general.pemCert') }}</label>
            <textarea v-model="form.tls_cert" class="input-field mt-1 font-mono text-xs h-24" spellcheck="false" />
            <input type="file" accept=".pem,.crt,.cer" class="text-xs mt-1" @change="readFile($event, 'tls_cert')" />
          </div>
          <div>
            <label class="text-xs text-slate-500">{{ t('system.general.pemKey') }}</label>
            <textarea v-model="form.tls_key" class="input-field mt-1 font-mono text-xs h-24" spellcheck="false" />
            <input type="file" accept=".pem,.key" class="text-xs mt-1" @change="readFile($event, 'tls_key')" />
          </div>
        </template>
        </template>
      </section>

      <button
        v-if="activeTab === 'basic' || activeTab === 'tls'"
        type="button"
        class="btn-primary"
        @click="save"
      >
        {{ t('common.save') }}
      </button>
    </div>

    <Teleport to="body">
      <div
        v-if="versionSwitchModalOpen"
        class="fixed inset-0 z-[60] flex items-center justify-center p-4 bg-black/40"
        role="presentation"
        @click.self="!versionSwitchSubmitting && closeVersionSwitchModal()"
      >
        <div
          class="bg-white rounded-xl shadow-xl w-full max-w-md border border-slate-200"
          role="dialog"
          aria-modal="true"
          aria-labelledby="version-switch-modal-title"
          @click.stop
        >
          <div class="flex items-center justify-between px-4 py-3 border-b border-slate-100">
            <h3 id="version-switch-modal-title" class="font-medium text-slate-900">
              {{ t('system.general.versionSwitchModalTitle') }}
            </h3>
            <button
              type="button"
              class="text-slate-500 hover:text-slate-800 text-xl leading-none px-2"
              :aria-label="t('common.cancel')"
              :disabled="versionSwitchSubmitting"
              @click="closeVersionSwitchModal"
            >
              ×
            </button>
          </div>
          <div class="p-4 space-y-3">
            <p class="text-sm text-slate-600">
              {{ t('system.general.versionSwitchModalBody', { tag: switchTag }) }}
            </p>
            <label class="block text-sm text-slate-700">
              {{ t('system.general.versionSwitchPasswordLabel') }}
              <input
                ref="versionSwitchPasswordRef"
                v-model="versionSwitchPassword"
                type="password"
                autocomplete="current-password"
                class="input-field mt-1 w-full"
                :disabled="versionSwitchSubmitting"
                @keydown.enter.prevent="confirmVersionSwitch"
              >
            </label>
            <p v-if="versionSwitchModalErr" class="text-sm text-red-600">{{ versionSwitchModalErr }}</p>
            <div class="flex justify-end gap-2 pt-1">
              <button
                type="button"
                class="btn-secondary"
                :disabled="versionSwitchSubmitting"
                @click="closeVersionSwitchModal"
              >
                {{ t('common.cancel') }}
              </button>
              <button
                type="button"
                class="btn-primary"
                :disabled="versionSwitchSubmitting"
                @click="confirmVersionSwitch"
              >
                {{ versionSwitchSubmitting ? t('common.processing') : t('system.general.versionSwitchConfirm') }}
              </button>
            </div>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>
