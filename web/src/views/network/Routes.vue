<script setup>
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'
import FloatingTerminal from '@/components/FloatingTerminal.vue'

const { t } = useI18n()
const managed = ref([])
const live = ref([])
const devLan = ref('')
const devWan = ref('')
const routeBackend = ref('kernel')
const frrBootOnStartup = ref(false)
const frrStatus = ref({})
const frrRoot = ref(false)
const frrInstallJob = ref(null)
const installingFrr = ref(false)
const frrInstallPoll = ref(null)
const frrInstallPollErrs = ref(0)
const showTerminal = ref(false)
const showConfig = ref(false)
const configTab = ref('frr.conf')
const configPath = ref('')
const configContent = ref('')
const configSaving = ref(false)
const err = ref('')
const ok = ref('')
const form = ref({
  dest: '10.0.0.0/8',
  gateway: '',
  device: '',
  metric: '',
  comment: '',
  enabled: true,
})

const configTabs = [
  { id: 'frr.conf', labelKey: 'network.routes.frrConfigTabFrr' },
  { id: 'extra', labelKey: 'network.routes.frrConfigTabExtra' },
  { id: 'managed', labelKey: 'network.routes.frrConfigTabManaged' },
  { id: 'daemons', labelKey: 'network.routes.frrConfigTabDaemons' },
]

async function loadFrr() {
  try {
    const f = await api.frr.get()
    routeBackend.value = f.route_backend || 'kernel'
    frrBootOnStartup.value = !!f.boot_on_startup
    frrStatus.value = f.status || {}
    frrRoot.value = !!f.root
    frrInstallJob.value = f.install_job || null
  } catch {
    frrStatus.value = {}
  }
}

async function load() {
  const d = await api.get('/api/v1/routes')
  managed.value = d.managed || []
  live.value = d.live || []
  devLan.value = d.dev_lan || ''
  devWan.value = d.dev_wan || ''
  if (d.route_backend) routeBackend.value = d.route_backend
  await loadFrr()
}

function stopFrrInstallPoll() {
  if (frrInstallPoll.value) {
    clearInterval(frrInstallPoll.value)
    frrInstallPoll.value = null
  }
}

function startFrrInstallPoll() {
  stopFrrInstallPoll()
  frrInstallPollErrs.value = 0
  frrInstallPoll.value = setInterval(async () => {
    try {
      const j = await api.frr.installStatus()
      frrInstallPollErrs.value = 0
      frrInstallJob.value = j
      if (j.state === 'ok') {
        stopFrrInstallPoll()
        installingFrr.value = false
        frrInstallJob.value = null
        ok.value = t('network.routes.frrInstalledOk')
        await loadFrr()
      } else if (j.state === 'failed') {
        stopFrrInstallPoll()
        installingFrr.value = false
        err.value = j.message || t('network.routes.frrInstallFailed')
      }
    } catch {
      frrInstallPollErrs.value += 1
      if (frrInstallPollErrs.value >= 3) {
        stopFrrInstallPoll()
        installingFrr.value = false
      }
    }
  }, 3000)
}

async function installFrr() {
  err.value = ''
  ok.value = ''
  if (!frrRoot.value) {
    err.value = t('network.routes.frrInstallNeedRoot')
    return
  }
  try {
    installingFrr.value = true
    const r = await api.frr.install()
    if (r?.job?.state === 'ok') {
      installingFrr.value = false
      ok.value = r.message || t('network.routes.frrInstalledOk')
      await loadFrr()
      return
    }
    ok.value = r.message || t('network.routes.frrInstalling')
    frrInstallJob.value = r.job || { state: 'running' }
    startFrrInstallPoll()
  } catch (e) {
    installingFrr.value = false
    err.value = e.message
  }
}

async function saveFrr() {
  err.value = ''
  ok.value = ''
  try {
    await api.frr.put({
      route_backend: routeBackend.value,
      boot_on_startup: frrBootOnStartup.value,
    })
    ok.value = t('network.routes.frrSaved')
    await loadFrr()
  } catch (e) {
    err.value = e.message
  }
}

async function frrService(action) {
  err.value = ''
  try {
    await api.frr.service(action)
    ok.value = t('network.routes.frrServiceDone')
    await loadFrr()
  } catch (e) {
    err.value = e.message
  }
}

async function openConfigModal() {
  showConfig.value = true
  await loadConfigTab(configTab.value)
}

async function loadConfigTab(which) {
  configTab.value = which
  try {
    const d = await api.frr.getConfig(which)
    configPath.value = d.path || ''
    configContent.value = d.content || ''
  } catch (e) {
    err.value = e.message
    configContent.value = ''
  }
}

async function saveConfig() {
  configSaving.value = true
  err.value = ''
  try {
    await api.frr.putConfig(configTab.value, configContent.value)
    ok.value = t('network.routes.frrConfigSaved')
  } catch (e) {
    err.value = e.message
  } finally {
    configSaving.value = false
  }
}

async function addRoute() {
  err.value = ''
  ok.value = ''
  try {
    await api.post('/api/v1/routes', {
      dest: form.value.dest,
      gateway: form.value.gateway || undefined,
      device: form.value.device || undefined,
      metric: form.value.metric ? Number(form.value.metric) : 0,
      comment: form.value.comment,
      enabled: form.value.enabled,
    })
    ok.value = t('network.routes.added')
    await load()
  } catch (e) {
    err.value = e.message
  }
}

async function toggleEnabled(r) {
  if (r.locked) return
  err.value = ''
  try {
    await api.put(`/api/v1/routes/${r.id}`, { ...r, enabled: !r.enabled })
    await load()
  } catch (e) {
    err.value = e.data?.error || e.message
  }
}

async function remove(r) {
  if (r.locked) {
    err.value = t('network.routes.deleteForbidden')
    return
  }
  if (!confirm(t('network.routes.confirmDelete', { dest: r.dest }))) return
  err.value = ''
  try {
    await api.del(`/api/v1/routes/${r.id}`)
    await load()
  } catch (e) {
    err.value = e.data?.error || e.message
  }
}

function sourceLabel(r) {
  if (r.source === 'wan') return t('network.routes.sourceWan')
  if (r.source === 'egress') return t('network.routes.sourceEgress')
  return t('network.routes.sourceManual')
}

function routeTableLabel(r) {
  const tbl = r.table || 0
  if (tbl <= 0 || tbl === 254) return 'main'
  return `${t('network.routes.routeTable')} ${tbl}`
}

async function applyAll() {
  err.value = ''
  try {
    const d = await api.post('/api/v1/routes/apply', {})
    const r = d.result || {}
    const applied = r.applied ?? 0
    const skipped = r.skipped ?? 0
    ok.value =
      applied === 0 && skipped > 0
        ? t('network.routes.replayNoop')
        : t('network.routes.replayed', { applied, skipped })
    await load()
  } catch (e) {
    err.value = e.message
  }
}

function fmtRoute(r) {
  let s = r.dest || r.Dest
  if (r.gateway) s += ` via ${r.gateway}`
  if (r.device) s += ` dev ${r.device}`
  else if (r.dev) s += ` dev ${r.dev}`
  if (r.metric) s += ` metric ${r.metric}`
  return s
}

onMounted(load)
onBeforeUnmount(stopFrrInstallPoll)
</script>

<template>
  <div class="page-stack">
    <PageHeader :title="t('network.routes.title')" :description="t('network.routes.description')" :ok="ok" :err="err" />

    <div class="card card-body mb-0">
      <h3 class="font-medium mb-3">{{ t('network.routes.add') }}</h3>
      <div class="grid sm:grid-cols-2 gap-3 text-sm">
        <div>
          <label class="text-xs text-slate-500">{{ t('network.routes.dest') }}</label>
          <input v-model="form.dest" class="input-field font-mono" />
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('network.routes.gateway') }}</label>
          <input v-model="form.gateway" class="input-field font-mono" />
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('network.routes.iface') }}</label>
          <input v-model="form.device" class="input-field font-mono" :placeholder="devLan || 'ens19'" />
        </div>
        <div>
          <label class="text-xs text-slate-500">Metric</label>
          <input v-model="form.metric" type="number" class="input-field" />
        </div>
        <div class="sm:col-span-2">
          <label class="text-xs text-slate-500">{{ t('common.comment') }}</label>
          <input v-model="form.comment" class="input-field" />
        </div>
        <label class="flex items-center gap-2 sm:col-span-2">
          <input v-model="form.enabled" type="checkbox" /> {{ t('network.routes.enabled') }}
        </label>
      </div>
      <div class="flex gap-2 mt-4">
        <button type="button" class="btn-primary" @click="addRoute">{{ t('network.routes.addApply') }}</button>
        <button type="button" class="btn-secondary" @click="applyAll">{{ t('network.routes.replay') }}</button>
      </div>
    </div>

    <div class="card card-body mb-0 space-y-4">
      <div>
        <h3 class="font-medium">{{ t('network.routes.frrSection') }}</h3>
        <p class="text-xs text-slate-500 mt-1">{{ t('network.routes.frrHint') }}</p>
      </div>

      <div class="grid sm:grid-cols-2 gap-3 text-sm">
        <div class="sm:col-span-2">
          <label class="text-xs text-slate-500">{{ t('network.routes.routeBackend') }}</label>
          <select v-model="routeBackend" class="input-field mt-1">
            <option value="kernel">{{ t('network.routes.routeBackendKernel') }}</option>
            <option value="frr">{{ t('network.routes.routeBackendFrr') }}</option>
          </select>
        </div>
      </div>

      <template v-if="routeBackend === 'frr'">
        <div class="flex flex-wrap items-center gap-2 text-xs">
          <span
            class="inline-flex px-2 py-0.5 rounded"
            :class="frrStatus.package_installed ? 'bg-emerald-100 text-emerald-800' : 'bg-amber-100 text-amber-800'"
          >
            {{ frrStatus.package_installed ? t('network.routes.frrInstalled') : t('network.routes.frrNotInstalled') }}
          </span>
          <span
            v-if="frrStatus.package_installed"
            class="inline-flex px-2 py-0.5 rounded"
            :class="frrStatus.active ? 'bg-emerald-100 text-emerald-800' : 'bg-slate-100 text-slate-600'"
          >
            {{ frrStatus.active ? t('network.routes.frrActive') : t('network.routes.frrInactive') }}
          </span>
          <span v-if="frrStatus.version" class="text-slate-500 font-mono">{{ frrStatus.version }}</span>
        </div>

        <div v-if="!frrStatus.package_installed" class="flex flex-wrap items-center gap-2">
          <button
            type="button"
            class="btn-primary"
            :disabled="installingFrr"
            @click="installFrr"
          >
            {{ installingFrr ? t('network.routes.frrInstalling') : t('network.routes.frrInstall') }}
          </button>
          <p v-if="frrInstallJob?.log_tail" class="text-xs text-slate-500 font-mono whitespace-pre-wrap max-w-full">
            {{ frrInstallJob.log_tail }}
          </p>
        </div>

        <label class="flex items-start gap-2 text-sm">
          <input v-model="frrBootOnStartup" type="checkbox" class="mt-0.5" />
          <span>
            {{ t('network.routes.frrBootOnStartup') }}
            <span class="block text-xs text-slate-500">{{ t('network.routes.frrBootHint') }}</span>
          </span>
        </label>

        <div class="flex flex-wrap gap-2">
          <button type="button" class="btn-secondary" @click="saveFrr">{{ t('network.routes.frrSave') }}</button>
          <template v-if="frrStatus.package_installed">
            <button type="button" class="btn-secondary" @click="frrService('start')">
              {{ t('network.routes.frrServiceStart') }}
            </button>
            <button type="button" class="btn-secondary" @click="frrService('stop')">
              {{ t('network.routes.frrServiceStop') }}
            </button>
            <button type="button" class="btn-secondary" @click="frrService('restart')">
              {{ t('network.routes.frrServiceRestart') }}
            </button>
          </template>
          <button type="button" class="btn-secondary" @click="showTerminal = true">
            {{ t('network.routes.frrOpenVtysh') }}
          </button>
          <button type="button" class="btn-secondary" @click="openConfigModal">
            {{ t('network.routes.frrEditConfig') }}
          </button>
        </div>
      </template>

      <div v-else class="flex flex-wrap gap-2">
        <button type="button" class="btn-secondary" @click="saveFrr">{{ t('network.routes.frrSave') }}</button>
      </div>
    </div>

    <div class="grid lg:grid-cols-2 gap-3">
      <section class="card table-wrap p-4">
        <h3 class="font-medium mb-3">{{ t('network.routes.managed') }}</h3>
        <table class="data w-full text-sm">
          <thead>
            <tr>
              <th>{{ t('network.routes.dest') }}</th>
              <th>{{ t('network.routes.gateway') }}</th>
              <th>{{ t('network.routes.routeTable') }}</th>
              <th>{{ t('network.routes.source') }}</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="r in managed" :key="r.id" :class="{ 'bg-slate-50': r.locked }">
              <td class="font-mono text-xs">{{ r.dest }}</td>
              <td class="font-mono text-xs">
                {{ r.gateway || '—' }} / {{ r.device || '—' }}
                <span v-if="r.metric" class="text-slate-400"> metric {{ r.metric }}</span>
              </td>
              <td class="text-xs">{{ routeTableLabel(r) }}</td>
              <td class="text-xs max-w-xs">
                <span
                  v-if="r.locked"
                  class="inline-block px-1.5 py-0.5 rounded bg-amber-100 text-amber-800 text-[10px] font-medium mr-1"
                >{{ t('network.routes.autoManaged') }}</span>
                <span class="text-slate-600">{{ sourceLabel(r) }}</span>
                <p v-if="r.source_note" class="text-slate-500 mt-0.5 leading-snug">{{ r.source_note }}</p>
                <p v-else-if="r.comment && !r.locked" class="text-slate-400 mt-0.5">{{ r.comment }}</p>
              </td>
              <td class="whitespace-nowrap text-xs">
                <template v-if="r.locked">
                  <span class="text-slate-400">{{ t('network.routes.autoManagedNoEdit') }}</span>
                </template>
                <template v-else>
                  <button type="button" class="text-blue-600 mr-2" @click="toggleEnabled(r)">
                    {{ r.enabled ? t('common.disabled') : t('common.enabled') }}
                  </button>
                  <button type="button" class="text-red-600" @click="remove(r)">{{ t('common.delete') }}</button>
                </template>
              </td>
            </tr>
            <tr v-if="!managed.length">
              <td colspan="5" class="text-center text-slate-400 py-4">{{ t('common.noData') }}</td>
            </tr>
          </tbody>
        </table>
      </section>

      <section class="card table-wrap p-4">
        <h3 class="font-medium mb-3">{{ t('network.routes.kernelMain') }}</h3>
        <table class="data w-full text-xs">
          <thead>
            <tr>
              <th>{{ t('network.routes.route') }}</th>
              <th>{{ t('network.routes.protocol') }}</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(r, i) in live" :key="i" :class="{ 'bg-green-50': r.managed }">
              <td class="font-mono">{{ fmtRoute(r) }}</td>
              <td>{{ r.protocol || '—' }}</td>
              <td>
                <span v-if="r.managed" class="text-green-700">{{ t('network.routes.managed') }}</span>
              </td>
            </tr>
          </tbody>
        </table>
      </section>
    </div>

    <FloatingTerminal
      :open="showTerminal"
      shell="vtysh"
      :title="t('network.routes.frrTerminalTitle')"
      @close="showTerminal = false"
    />

    <Teleport to="body">
      <div
        v-if="showConfig"
        class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/40"
        role="dialog"
        aria-modal="true"
        @click.self="showConfig = false"
      >
        <div class="card w-full max-w-3xl shadow-xl flex flex-col max-h-[90vh]" @click.stop>
          <div class="flex items-center justify-between gap-2 px-4 py-3 border-b border-slate-200">
            <div>
              <h3 class="font-semibold text-sm">{{ t('network.routes.frrConfigTitle') }}</h3>
              <p v-if="configPath" class="text-xs text-slate-500 font-mono mt-0.5">{{ configPath }}</p>
            </div>
            <button
              type="button"
              class="text-slate-400 hover:text-slate-600 text-xl leading-none"
              @click="showConfig = false"
            >
              ×
            </button>
          </div>
          <div class="flex flex-wrap gap-1 px-4 pt-3 border-b border-slate-100">
            <button
              v-for="tab in configTabs"
              :key="tab.id"
              type="button"
              class="text-xs px-3 py-1.5 rounded-t border-b-2 -mb-px"
              :class="
                configTab === tab.id
                  ? 'border-blue-500 text-blue-700 bg-blue-50'
                  : 'border-transparent text-slate-600 hover:bg-slate-50'
              "
              @click="loadConfigTab(tab.id)"
            >
              {{ t(tab.labelKey) }}
            </button>
          </div>
          <div class="p-4 flex-1 min-h-0 flex flex-col gap-3">
            <textarea
              v-model="configContent"
              class="input-field font-mono text-xs flex-1 min-h-[20rem] resize-y"
              spellcheck="false"
            />
            <div class="flex justify-end gap-2">
              <button type="button" class="btn-secondary" @click="showConfig = false">{{ t('common.cancel') }}</button>
              <button type="button" class="btn-primary" :disabled="configSaving" @click="saveConfig">
                {{ t('common.save') }}
              </button>
            </div>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>
