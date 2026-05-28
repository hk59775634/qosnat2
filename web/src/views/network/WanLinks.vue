<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'

const { t } = useI18n()
const links = ref([])
const egress = ref([])
const resolved = ref([])
const cloudflareCIDRs = ref([])
const warpStatus = ref({ installed: false, service_up: false, connected: false, interface: '', root: false })
const installingWarp = ref(false)
const warpInstallJob = ref(null)
const warpInstallPoll = ref(null)
const warpInstallPollErrs = ref(0)
const warpConnecting = ref(false)
const warpConnectResult = ref(null)
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
  cidr: '10.250.0.0/24',
  match: 'source',
  wan_link_id: '',
  snat_ip: '',
  priority: 100,
  enabled: true,
})

const linkOptions = computed(() =>
  (links.value || []).filter((w) => w.enabled).map((w) => ({ id: w.id, label: `${w.name} (${w.device})` }))
)
const warpInstallRunning = computed(() => installingWarp.value || warpInstallJob.value?.state === 'running')

function resolvedRow(policyId) {
  return resolved.value.find((r) => r.policy?.id === policyId)
}

async function load() {
  const [wan, eg, ws] = await Promise.all([
    api.network.wanLinks.list(),
    api.network.egressPolicies.list(),
    api.network.warp.status(),
  ])
  links.value = wan.wan_links || []
  devWan.value = wan.dev_wan || ''
  egress.value = eg.egress_policies || []
  resolved.value = eg.resolved || []
  cloudflareCIDRs.value = eg.cloudflare_cdn_cidrs_ipv4 || []
  warpStatus.value = ws || warpStatus.value
  warpInstallJob.value = normalizeWarpJob(ws?.install_job)
  installingWarp.value = ws?.install_job?.state === 'running'
  if (installingWarp.value && !warpInstallPoll.value) {
    startWarpInstallPoll()
  }
  if (!form.value.device && devWan.value) form.value.device = devWan.value
  if (!egForm.value.wan_link_id && links.value.length) {
    const us = links.value.find((w) => w.device === 'ens19') || links.value[0]
    egForm.value.wan_link_id = us.id
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

async function connectWarp() {
  err.value = ''
  ok.value = ''
  warpConnectResult.value = null
  warpConnecting.value = true
  try {
    const r = await api.network.warp.connect()
    warpConnectResult.value = r?.health || null
    ok.value = (r?.health?.connected ? t('network.wanLinks.warpConnected') : t('network.wanLinks.warpConnectPending'))
    await load()
    const warpLink = links.value.find((w) => w.warp_managed)
    if (warpLink) {
      egForm.value.wan_link_id = warpLink.id
    }
  } catch (e) {
    warpConnectResult.value = e?.data?.diagnostics ? { diagnostics: e.data.diagnostics } : null
    err.value = e.message
  } finally {
    warpConnecting.value = false
  }
}

async function disconnectWarp() {
  err.value = ''
  ok.value = ''
  try {
    await api.network.warp.disconnect()
    ok.value = t('network.wanLinks.warpDisconnected')
    await load()
  } catch (e) {
    err.value = e.message
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

async function remove(id) {
  if (!confirm(t('common.delete') + '?')) return
  err.value = ''
  try {
    await api.network.wanLinks.del(id)
    await load()
  } catch (e) {
    err.value = e.message
  }
}

async function addEgress() {
  err.value = ''
  try {
    const body = { ...egForm.value }
    if (!body.snat_ip) delete body.snat_ip
    await api.network.egressPolicies.add(body)
    ok.value = t('common.saved')
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
  let added = 0
  for (const cidr of prefixes) {
    const exists = (egress.value || []).some((p) => p.cidr === cidr)
    if (exists) continue
    await api.network.egressPolicies.add({
      name: `Cloudflare CDN ${cidr}`,
      cidr,
      match: 'destination',
      wan_link_id: egForm.value.wan_link_id,
      snat_ip: egForm.value.snat_ip || undefined,
      priority: egForm.value.priority || 100,
      enabled: true,
    })
    added += 1
  }
  ok.value = `Cloudflare CDN 策略已导入 ${added} 条`
  await load()
}

async function removeEgress(id) {
  if (!confirm(t('common.delete') + '?')) return
  err.value = ''
  try {
    await api.network.egressPolicies.del(id)
    await load()
  } catch (e) {
    err.value = e.message
  }
}

onMounted(load)
onUnmounted(stopWarpInstallPoll)
</script>

<template>
  <div class="page-stack">
    <PageHeader :title="t('network.wanLinks.title')" :description="t('network.wanLinks.description')" />
    <p v-if="ok" class="text-green-700 text-sm mb-2">{{ ok }}</p>
    <p v-if="err" class="text-red-600 text-sm mb-2">{{ err }}</p>

    <div class="card card-body mb-0 space-y-3 text-sm">
      <h3 class="font-medium text-slate-800">{{ t('network.wanLinks.title') }}</h3>
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

    <div class="card card-body mb-0 space-y-3 text-sm">
      <div>
        <h3 class="font-medium text-slate-800">{{ t('network.wanLinks.warpTitle') }}</h3>
        <p class="text-xs text-slate-500 mt-1">{{ t('network.wanLinks.warpHint') }}</p>
      </div>
      <div class="text-xs text-slate-600 rounded bg-slate-50 p-2">
        {{ t('network.wanLinks.warpState') }}:
        {{ warpStatus.installed ? t('network.wanLinks.warpInstalledLabel') : t('network.wanLinks.warpNotInstalledLabel') }}
        · {{ warpStatus.connected ? t('network.wanLinks.warpConnectedLabel') : t('network.wanLinks.warpDisconnectedLabel') }}
        <span v-if="warpStatus.interface" class="font-mono"> · {{ warpStatus.interface }}</span>
      </div>
      <div class="flex flex-wrap gap-2">
        <button type="button" class="btn-secondary" :disabled="!warpStatus.root || warpStatus.installed || warpInstallRunning" @click="installWarp">
          {{ warpInstallRunning ? t('network.wanLinks.warpInstalling') : t('network.wanLinks.warpInstallBtn') }}
        </button>
        <button type="button" class="btn-secondary" :disabled="!warpStatus.root || !warpStatus.installed || warpStatus.connected" @click="connectWarp">
          {{ warpConnecting ? t('network.wanLinks.warpConnecting') : t('network.wanLinks.warpConnectBtn') }}
        </button>
        <button type="button" class="btn-secondary" :disabled="!warpStatus.root || !warpStatus.installed || !warpStatus.connected" @click="disconnectWarp">
          {{ t('network.wanLinks.warpDisconnectBtn') }}
        </button>
      </div>
      <div v-if="warpConnecting || warpConnectResult" class="mt-1 p-3 rounded border text-xs space-y-2 border-slate-200 bg-slate-50">
        <div class="flex gap-3 text-sm">
          <span>{{ t('network.wanLinks.warpConnectTask') }}: <strong>{{ warpConnecting ? 'running' : 'done' }}</strong></span>
          <span v-if="warpConnectResult?.netns_status || warpConnectResult?.diagnostics?.netns_warp_status" class="text-slate-600">
            {{ warpConnectResult?.netns_status || warpConnectResult?.diagnostics?.netns_warp_status }}
          </span>
        </div>
        <pre
          v-if="warpConnectResult?.diagnostics"
          class="max-h-32 overflow-auto whitespace-pre-wrap font-mono text-[11px] text-slate-700"
        >{{ JSON.stringify(warpConnectResult.diagnostics, null, 2) }}</pre>
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

    <div class="table-wrap card">
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
          <tr v-for="w in links" :key="w.id">
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
            <td>
              <button
                v-if="!w.warp_managed"
                type="button"
                class="text-red-600 text-xs"
                @click="remove(w.id)"
              >{{ t('common.delete') }}</button>
              <span v-else class="text-slate-400 text-xs">{{ t('network.wanLinks.warpManagedNoDelete') }}</span>
            </td>
          </tr>
          <tr v-if="!links.length">
            <td colspan="7" class="text-center text-slate-400 py-3">{{ t('network.wanLinks.noExtra') }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div class="card card-body space-y-3 text-sm">
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
          <label class="text-xs text-slate-500">{{ t('network.wanLinks.targetCidr') }}</label>
          <input v-model="egForm.cidr" class="input-field mt-1 font-mono" placeholder="10.250.0.0/24" />
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('network.wanLinks.matchType') }}</label>
          <select v-model="egForm.match" class="input-field mt-1">
            <option value="source">{{ t('network.wanLinks.matchSource') }}</option>
            <option value="destination">{{ t('network.wanLinks.matchDestination') }}</option>
          </select>
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('network.wanLinks.wanLink') }}</label>
          <select v-model="egForm.wan_link_id" class="input-field mt-1">
            <option value="">{{ t('network.interfaces.choose') }}</option>
            <option v-for="o in linkOptions" :key="o.id" :value="o.id">{{ o.label }}</option>
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
      </div>
    </div>

    <div class="table-wrap card">
      <table class="data w-full text-sm">
        <thead>
          <tr>
            <th>{{ t('common.name') }}</th>
            <th>{{ t('network.wanLinks.targetCidr') }}</th>
            <th>{{ t('network.wanLinks.matchType') }}</th>
            <th>{{ t('network.wanLinks.wanLink') }}</th>
            <th>SNAT</th>
            <th>{{ t('network.wanLinks.routeTable') }}</th>
            <th>{{ t('network.wanLinks.priority') }}</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="p in egress" :key="p.id">
            <td>{{ p.name || p.id }}</td>
            <td class="font-mono">{{ p.cidr }}</td>
            <td>{{ p.match === 'destination' ? t('network.wanLinks.matchDestination') : t('network.wanLinks.matchSource') }}</td>
            <td class="font-mono">{{ links.find((w) => w.id === p.wan_link_id)?.name || p.wan_link_id }}</td>
            <td class="font-mono text-xs">
              {{ resolvedRow(p.id)?.snat_ip || p.snat_ip || t('network.wanLinks.snatAuto') }}
            </td>
            <td>{{ resolvedRow(p.id)?.table ?? '—' }}</td>
            <td>{{ p.priority }}</td>
            <td>
              <button type="button" class="text-red-600 text-xs" @click="removeEgress(p.id)">{{ t('common.delete') }}</button>
            </td>
          </tr>
          <tr v-if="!egress.length">
            <td colspan="8" class="text-center text-slate-400 py-3">{{ t('network.wanLinks.noEgress') }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
