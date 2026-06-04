<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/api/client'

const { t } = useI18n()
import PageHeader from '@/components/PageHeader.vue'

const cfg = ref(null)
const status = ref(null)
const interfaces = ref([])
const devLan = ref('')
const devWan = ref('')
const rendered = ref('')
const leases = ref([])
const err = ref('')
const ok = ref('')
const dnsRoot = ref(false)
const dnsText = ref('')
const upstreamDnsText = ref('')
const trustedDnsText = ref('')
const untrustedDnsText = ref('')
const chnroutes = ref({ path: '', exists: false, entries: 0 })
const staticForm = ref({ mac: '', ip: '', hostname: '', comment: '' })
const installingDnsmasq = ref(false)
const dnsmasqInstallJob = ref(null)
const dnsmasqInstallPoll = ref(null)
const dnsmasqInstallPollErrs = ref(0)

const dnsmasqInstallRunning = computed(
  () => installingDnsmasq.value || dnsmasqInstallJob.value?.state === 'running',
)

const bindIface = computed({
  get: () => cfg.value?.interface || devLan.value || '',
  set: (v) => {
    if (cfg.value) cfg.value.interface = v
  },
})

const dnsmasqStatusLabel = computed(() => {
  if (!status.value?.installed) return t('network.dhcp.statusNotInstalled')
  return status.value.active ? t('network.dhcp.statusRunning') : t('network.dhcp.statusStopped')
})

const serviceActive = computed(() => !!(cfg.value?.enabled || cfg.value?.dns_enabled))

async function load() {
  const d = await api.get('/api/v1/dhcp')
  cfg.value = d.config || {}
  status.value = d.status
  interfaces.value = d.interfaces || []
  devLan.value = d.dev_lan || ''
  devWan.value = d.dev_wan || ''
  rendered.value = d.rendered || ''
  leases.value = d.leases || []
  dnsText.value = (cfg.value.dns_servers || []).join('\n')
  upstreamDnsText.value = (cfg.value.upstream_dns || []).join('\n')
  trustedDnsText.value = (cfg.value.trusted_dns || []).join('\n')
  untrustedDnsText.value = (cfg.value.untrusted_dns || []).join('\n')
  chnroutes.value = d.chnroutes || { path: cfg.value.chnroutes_file || '', exists: false, entries: 0 }
  dnsRoot.value = !!d.root
  dnsmasqInstallJob.value = d.dnsmasq_install_job || null
  installingDnsmasq.value = d.dnsmasq_install_job?.state === 'running'
  if (installingDnsmasq.value && !dnsmasqInstallPoll.value) {
    startDnsmasqInstallPoll()
  }
  if (!cfg.value.interface && devLan.value) {
    cfg.value.interface = devLan.value
  }
}

function stopDnsmasqInstallPoll() {
  if (dnsmasqInstallPoll.value) {
    clearInterval(dnsmasqInstallPoll.value)
    dnsmasqInstallPoll.value = null
  }
}

function startDnsmasqInstallPoll() {
  stopDnsmasqInstallPoll()
  dnsmasqInstallPollErrs.value = 0
  dnsmasqInstallPoll.value = setInterval(async () => {
    try {
      const j = await api.dhcp.installChnroutesDnsmasqStatus()
      dnsmasqInstallPollErrs.value = 0
      dnsmasqInstallJob.value = j
      if (j.state === 'ok') {
        stopDnsmasqInstallPoll()
        installingDnsmasq.value = false
        dnsmasqInstallJob.value = null
        ok.value = t('network.dhcp.dnsmasqChnroutesInstalled')
        await load()
      } else if (j.state === 'failed') {
        stopDnsmasqInstallPoll()
        installingDnsmasq.value = false
        err.value = j.message || t('network.dhcp.dnsmasqChnroutesInstallFailed')
      }
    } catch {
      dnsmasqInstallPollErrs.value += 1
      if (dnsmasqInstallPollErrs.value >= 3) {
        stopDnsmasqInstallPoll()
        installingDnsmasq.value = false
        err.value = t('network.dhcp.dnsmasqChnroutesInstallStatusLost')
      }
    }
  }, 3000)
}

async function installChnroutesDnsmasq() {
  err.value = ''
  ok.value = ''
  if (!dnsRoot.value) {
    err.value = t('network.dhcp.dnsmasqChnroutesInstallNeedRoot')
    return
  }
  try {
    installingDnsmasq.value = true
    const r = await api.dhcp.installChnroutesDnsmasq()
    const state = r?.job?.state || ''
    if (state === 'ok') {
      installingDnsmasq.value = false
      dnsmasqInstallJob.value = null
      ok.value = r.message || t('network.dhcp.dnsmasqChnroutesInstalled')
      await load()
      return
    }
    ok.value = r.message || t('network.dhcp.dnsmasqChnroutesInstalling')
    dnsmasqInstallJob.value = r.job || { state: 'running' }
    startDnsmasqInstallPoll()
  } catch (e) {
    installingDnsmasq.value = false
    if (e.data?.job) dnsmasqInstallJob.value = e.data.job
    err.value = e.message
  }
}

async function save(applyAfter) {
  err.value = ''
  ok.value = ''
  try {
    cfg.value.dns_servers = dnsText.value
      .split(/[\n,]+/)
      .map((s) => s.trim())
      .filter(Boolean)
    cfg.value.upstream_dns = upstreamDnsText.value
      .split(/[\n,]+/)
      .map((s) => s.trim())
      .filter(Boolean)
    cfg.value.trusted_dns = trustedDnsText.value
      .split(/[\n,]+/)
      .map((s) => s.trim())
      .filter(Boolean)
    cfg.value.untrusted_dns = untrustedDnsText.value
      .split(/[\n,]+/)
      .map((s) => s.trim())
      .filter(Boolean)
    await api.put('/api/v1/dhcp', cfg.value)
    ok.value = t('network.dhcp.saved')
    if (applyAfter) {
      await api.post('/api/v1/dhcp/apply', {})
      ok.value = serviceActive.value ? t('network.dhcp.savedStarted') : t('network.dhcp.savedStopped')
    }
    await load()
  } catch (e) {
    err.value = e.message
  }
}

function addStatic() {
  if (!staticForm.value.mac || !staticForm.value.ip) {
    err.value = t('network.dhcp.staticNeedMacIp')
    return
  }
  if (!cfg.value.static_leases) cfg.value.static_leases = []
  cfg.value.static_leases.push({ ...staticForm.value })
  staticForm.value = { mac: '', ip: '', hostname: '', comment: '' }
  err.value = ''
}

function removeStatic(i) {
  cfg.value.static_leases.splice(i, 1)
}

async function updateChnroutes() {
  err.value = ''
  ok.value = ''
  try {
    const d = await api.post('/api/v1/dhcp/chnroutes/update', {
      path: cfg.value.chnroutes_file || undefined,
    })
    chnroutes.value = d.chnroutes || chnroutes.value
    ok.value = t('network.dhcp.chnroutesUpdated', { n: d.entries ?? chnroutes.value.entries ?? 0 })
  } catch (e) {
    err.value = e.message
  }
}

onMounted(load)
onBeforeUnmount(stopDnsmasqInstallPoll)
</script>

<template>
  <div class="page-stack">
    <PageHeader :title="t('network.dhcp.title')" :description="t('network.dhcp.description')" :ok="ok" :err="err" />

    <div v-if="cfg" class="card card-body mb-0 space-y-3">
      <div class="flex flex-wrap gap-x-6 gap-y-2">
        <label class="flex items-center gap-2 text-sm font-medium">
          <input v-model="cfg.enabled" type="checkbox" /> {{ t('network.dhcp.enable') }}
        </label>
        <label class="flex items-center gap-2 text-sm font-medium">
          <input v-model="cfg.dns_enabled" type="checkbox" /> {{ t('network.dhcp.enableDns') }}
        </label>
      </div>
      <p class="text-xs text-slate-500">{{ t('network.dhcp.serviceModeHint') }}</p>

      <div v-if="serviceActive" class="grid sm:grid-cols-2 gap-3 text-sm">
        <div class="sm:col-span-2">
          <label class="text-xs text-slate-500">{{ t('network.dhcp.bindIface') }}</label>
          <select v-model="bindIface" class="input-field font-mono">
            <option v-for="iface in interfaces" :key="iface.name" :value="iface.name">
              {{ iface.name }}
              <template v-if="iface.addrs?.length"> — {{ iface.addrs.join(', ') }}</template>
              {{ iface.up ? '' : ' (down)' }}
            </option>
          </select>
          <p class="text-xs text-slate-400 mt-1">{{ t('network.dhcp.bindIfaceDefault', { lan: devLan }) }}</p>
        </div>
      </div>

      <div v-if="cfg.enabled" class="grid sm:grid-cols-2 gap-3 text-sm border-t border-slate-100 pt-3">
        <h3 class="sm:col-span-2 font-medium text-sm">{{ t('network.dhcp.dhcpSection') }}</h3>
        <div>
          <label class="text-xs text-slate-500">{{ t('network.dhcp.gateway') }}</label>
          <input v-model="cfg.router" class="input-field font-mono" placeholder="192.168.1.1" />
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('network.dhcp.poolStart') }}</label>
          <input v-model="cfg.range_start" class="input-field font-mono" />
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('network.dhcp.poolEnd') }}</label>
          <input v-model="cfg.range_end" class="input-field font-mono" />
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('network.dhcp.netmask') }}</label>
          <input v-model="cfg.netmask" class="input-field font-mono" placeholder="255.255.255.0" />
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('network.dhcp.leaseTimeSec') }}</label>
          <input v-model.number="cfg.lease_time_sec" type="number" class="input-field" />
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('network.dhcp.domainOptional') }}</label>
          <input v-model="cfg.domain" class="input-field" />
        </div>
        <label class="flex items-center gap-2 sm:col-span-2 text-sm">
          <input v-model="cfg.authoritative" type="checkbox" /> {{ t('network.dhcp.authoritativeCheck') }}
        </label>
      </div>

      <div v-if="cfg.dns_enabled" class="grid sm:grid-cols-2 gap-3 text-sm border-t border-slate-100 pt-3">
        <div class="sm:col-span-2">
          <h3 class="font-medium text-sm mb-2">{{ t('network.dhcp.dnsSection') }}</h3>
          <p class="text-xs text-slate-500 mb-3">{{ t('network.dhcp.dnsSectionHint') }}</p>
        </div>
        <div v-if="cfg.enabled" class="sm:col-span-2">
          <label class="text-xs text-slate-500">{{ t('network.dhcp.dnsServersMultiline') }}</label>
          <textarea v-model="dnsText" class="input-field font-mono h-16" rows="2" />
          <p class="text-xs text-slate-400 mt-1">{{ t('network.dhcp.dnsServersHint') }}</p>
        </div>
        <div class="sm:col-span-2">
          <label class="text-xs text-slate-500">{{ t('network.dhcp.upstreamDnsMultiline') }}</label>
          <textarea
            v-model="upstreamDnsText"
            class="input-field font-mono h-16"
            rows="2"
            :placeholder="t('network.dhcp.upstreamDnsPh')"
            :disabled="cfg.chnroutes_enabled"
          />
          <p class="text-xs text-slate-400 mt-1">{{ t('network.dhcp.upstreamDnsHint') }}</p>
        </div>

        <div class="sm:col-span-2 border-t border-slate-100 pt-3 space-y-3">
          <label class="flex items-center gap-2 text-sm font-medium">
            <input v-model="cfg.chnroutes_enabled" type="checkbox" :disabled="!status?.chnroutes_support" />
            {{ t('network.dhcp.chnroutesEnable') }}
          </label>
          <p v-if="!status?.chnroutes_support" class="text-xs text-amber-700">
            {{ t('network.dhcp.chnroutesUnsupported') }}
          </p>
          <p v-else class="text-xs text-slate-500">{{ t('network.dhcp.chnroutesHint') }}</p>
          <div v-if="!status?.chnroutes_support" class="space-y-2">
            <button
              type="button"
              class="btn-secondary text-sm"
              :disabled="dnsmasqInstallRunning || !dnsRoot"
              :title="!dnsRoot ? t('network.dhcp.dnsmasqChnroutesInstallNeedRoot') : ''"
              @click="installChnroutesDnsmasq"
            >
              {{
                dnsmasqInstallRunning
                  ? t('network.dhcp.dnsmasqChnroutesInstalling')
                  : t('network.dhcp.dnsmasqChnroutesInstallBtn')
              }}
            </button>
            <div
              v-if="
                dnsmasqInstallRunning
                  || (dnsmasqInstallJob && (dnsmasqInstallJob.state === 'running' || dnsmasqInstallJob.state === 'failed'))
              "
              class="p-3 rounded border text-xs space-y-2"
              :class="dnsmasqInstallJob?.state === 'failed' ? 'border-red-200 bg-red-50' : 'border-slate-200 bg-slate-50'"
            >
              <div class="flex flex-wrap gap-x-3 gap-y-1 text-sm">
                <span>
                  {{ t('network.dhcp.dnsmasqChnroutesInstallTask') }}:
                  <strong>{{ dnsmasqInstallJob?.state || 'running' }}</strong>
                </span>
                <span v-if="dnsmasqInstallJob?.message" class="text-slate-600">{{ dnsmasqInstallJob.message }}</span>
              </div>
              <pre
                v-if="dnsmasqInstallJob?.log_tail"
                class="max-h-40 overflow-auto whitespace-pre-wrap font-mono text-[11px] text-slate-700"
              >{{ dnsmasqInstallJob.log_tail }}</pre>
            </div>
          </div>

          <template v-if="cfg.chnroutes_enabled">
            <div>
              <label class="text-xs text-slate-500">{{ t('network.dhcp.chnroutesFile') }}</label>
              <input
                v-model="cfg.chnroutes_file"
                class="input-field font-mono mt-1"
                placeholder="/etc/qosnat2/chnroutes.txt"
              />
              <p class="text-xs text-slate-400 mt-1">
                {{ t('network.dhcp.chnroutesFileStatus', {
                  n: chnroutes.entries || 0,
                  status: chnroutes.exists ? t('network.dhcp.chnroutesExists') : t('network.dhcp.chnroutesMissing'),
                }) }}
              </p>
            </div>
            <div>
              <label class="text-xs text-slate-500">{{ t('network.dhcp.trustedDnsMultiline') }}</label>
              <textarea v-model="trustedDnsText" class="input-field font-mono h-16" rows="2" />
              <p class="text-xs text-slate-400 mt-1">{{ t('network.dhcp.trustedDnsHint') }}</p>
            </div>
            <div>
              <label class="text-xs text-slate-500">{{ t('network.dhcp.untrustedDnsMultiline') }}</label>
              <textarea v-model="untrustedDnsText" class="input-field font-mono h-16" rows="2" />
              <p class="text-xs text-slate-400 mt-1">{{ t('network.dhcp.untrustedDnsHint') }}</p>
            </div>
            <button type="button" class="btn-secondary text-sm" @click="updateChnroutes">
              {{ t('network.dhcp.chnroutesUpdate') }}
            </button>
            <p class="text-xs text-slate-400">{{ t('network.dhcp.chnroutesSource') }}</p>
          </template>
        </div>
      </div>

      <div v-if="cfg.enabled" class="border-t border-slate-200 pt-4 space-y-3">
        <h3 class="font-medium text-sm">{{ t('network.dhcp.ipv6Section') }}</h3>
        <label class="flex items-center gap-2 text-sm">
          <input v-model="cfg.ipv6_enabled" type="checkbox" /> {{ t('network.dhcp.ipv6Enable') }}
        </label>
        <div v-if="cfg.ipv6_enabled" class="grid sm:grid-cols-2 gap-3 text-sm">
          <div class="sm:col-span-2">
            <label class="text-xs text-slate-500">{{ t('network.dhcp.ipv6Prefix') }}</label>
            <input v-model="cfg.ipv6_prefix" class="input-field mt-1 font-mono" />
          </div>
          <div>
            <label class="text-xs text-slate-500">{{ t('network.dhcp.ipv6Start') }}</label>
            <input v-model="cfg.ipv6_start" class="input-field mt-1 font-mono" placeholder="2001:db8::100" />
          </div>
          <div>
            <label class="text-xs text-slate-500">{{ t('network.dhcp.ipv6End') }}</label>
            <input v-model="cfg.ipv6_end" class="input-field mt-1 font-mono" placeholder="2001:db8::200" />
          </div>
        </div>
        <label class="flex items-center gap-2 text-sm">
          <input v-model="cfg.ra_enabled" type="checkbox" /> {{ t('network.dhcp.raEnable') }}
        </label>
        <div v-if="cfg.ra_enabled" class="text-sm max-w-xs">
          <label class="text-xs text-slate-500">{{ t('network.dhcp.raInterval') }}</label>
          <input v-model.number="cfg.ra_interval_sec" type="number" class="input-field mt-1" />
        </div>
      </div>

      <div class="flex flex-wrap gap-2">
        <button type="button" class="btn-primary" @click="save(true)">{{ t('network.dhcp.saveApply') }}</button>
        <button type="button" class="btn-secondary" @click="save(false)">{{ t('network.dhcp.saveOnly') }}</button>
      </div>

      <p class="text-xs text-slate-500">
        dnsmasq:
        <span :class="status?.active ? 'text-green-700' : 'text-slate-500'">
          {{ dnsmasqStatusLabel }}
        </span>
        <span v-if="status?.version"> · {{ status.version }}</span>
        <span v-if="status?.chnroutes_support"> · chnroutes</span>
        <span v-if="status?.config"> · {{ status.config }}</span>
      </p>
    </div>

    <div v-if="cfg" class="grid lg:grid-cols-2 gap-3">
      <section class="card p-4">
        <h3 class="font-medium mb-3">{{ t('network.dhcp.staticLeases') }}</h3>
        <div class="grid sm:grid-cols-2 gap-2 text-sm mb-3">
          <input v-model="staticForm.mac" class="input-field font-mono text-xs" placeholder="aa:bb:cc:dd:ee:ff" />
          <input v-model="staticForm.ip" class="input-field font-mono text-xs" placeholder="192.168.1.50" />
          <input v-model="staticForm.hostname" class="input-field text-xs" :placeholder="t('network.dhcp.hostname')" />
          <button type="button" class="btn-secondary text-xs" @click="addStatic">{{ t('common.add') }}</button>
        </div>
        <table class="data w-full text-xs">
          <thead>
            <tr>
              <th>{{ t('network.dhcp.mac') }}</th>
              <th>{{ t('network.dhcp.ip') }}</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(sl, i) in cfg.static_leases" :key="i">
              <td class="font-mono">{{ sl.mac }}</td>
              <td class="font-mono">{{ sl.ip }}</td>
              <td>
                <button type="button" class="text-red-600" @click="removeStatic(i)">{{ t('common.delete') }}</button>
              </td>
            </tr>
            <tr v-if="!cfg.static_leases?.length">
              <td colspan="3" class="text-center text-slate-400 py-3">{{ t('network.dhcp.none') }}</td>
            </tr>
          </tbody>
        </table>
      </section>

      <section class="card p-4">
        <h3 class="font-medium mb-3">{{ t('network.dhcp.currentLeases') }}</h3>
        <table class="data w-full text-xs">
          <thead>
            <tr>
              <th>{{ t('network.dhcp.leaseType') }}</th>
              <th>{{ t('network.dhcp.ip') }}</th>
              <th>{{ t('network.dhcp.mac') }}</th>
              <th>{{ t('network.dhcp.hostname') }}</th>
              <th>{{ t('network.dhcp.expires') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(l, i) in leases" :key="i">
              <td>{{ l.family }}</td>
              <td class="font-mono">{{ l.ip }}</td>
              <td class="font-mono">{{ l.mac }}</td>
              <td>{{ l.hostname || '—' }}</td>
              <td class="font-mono text-slate-500">{{ l.expires || l.expires_unix }}</td>
            </tr>
            <tr v-if="!leases.length">
              <td colspan="5" class="text-center text-slate-400 py-3">{{ t('network.dhcp.noLeases') }}</td>
            </tr>
          </tbody>
        </table>
        <pre
          v-if="status?.leases_raw"
          class="text-xs font-mono bg-slate-50 p-2 rounded overflow-auto max-h-24 mt-2 text-slate-500"
        >{{ status.leases_raw }}</pre>
        <h3 class="font-medium mt-4 mb-2 text-sm">{{ t('network.dhcp.previewConfig') }}</h3>
        <pre class="text-xs font-mono bg-slate-50 p-2 rounded overflow-auto max-h-40">{{
          rendered || t('network.dhcp.previewEmpty')
        }}</pre>
      </section>
    </div>
  </div>
</template>
