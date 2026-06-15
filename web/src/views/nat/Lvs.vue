<script setup>
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'

const { t } = useI18n()
const cfg = ref({ enabled: false, mode: 'nat', virtual_servers: [] })
const status = ref({})
const devWan = ref('')
const ocservHint = ref({ default_port: 443, default_persistence_sec: 3600, default_scheduler: 'sh' })
const warnings = ref([])
const err = ref('')
const ok = ref('')
const installing = ref(false)
const applying = ref(false)
const rsText = ref('10.0.0.10:80\n10.0.0.11:80')

const form = ref({
  vip: '',
  port: 80,
  protocol: 'tcp',
  scheduler: 'rr',
  persistence_sec: 0,
  auto_vip: true,
  comment: '',
})

const ocservForm = ref({
  vip: '',
  port: 443,
  persistence_sec: 3600,
  scheduler: 'sh',
  auto_vip: true,
})
const ocservNodesText = ref('10.0.0.10\n10.0.0.11')
const rsAddDraft = ref({})

function rsDraftKey(vsId) {
  return vsId || ''
}

function getRsDraft(vsId) {
  return rsAddDraft.value[rsDraftKey(vsId)] || ''
}

function setRsDraft(vsId, val) {
  rsAddDraft.value = { ...rsAddDraft.value, [rsDraftKey(vsId)]: val }
}

function parseRealServers(text, defaultPort) {
  return text
    .split(/[\n,]+/)
    .map((line) => line.trim())
    .filter(Boolean)
    .map((line) => {
      const [ip, portStr] = line.split(':')
      const port = portStr ? Number(portStr) : defaultPort
      return { ip: ip.trim(), port: port || defaultPort, weight: 1 }
    })
}

function parseNodeIPs(text) {
  return text
    .split(/[\n,]+/)
    .map((line) => line.trim())
    .filter(Boolean)
}

async function load() {
  const d = await api.lvs.get()
  cfg.value = { ...cfg.value, ...(d.config || {}) }
  status.value = d.status || {}
  devWan.value = d.dev_wan || ''
  ocservHint.value = { ...ocservHint.value, ...(d.ocserv_hint || {}) }
  warnings.value = d.warnings || []
  ocservForm.value.port = ocservHint.value.default_port || 443
  ocservForm.value.persistence_sec = ocservHint.value.default_persistence_sec || 3600
  ocservForm.value.scheduler = ocservHint.value.default_scheduler || 'sh'
}

async function installIpvsadm() {
  installing.value = true
  err.value = ''
  try {
    await api.lvs.install()
    ok.value = t('nat.lvs.installedOk')
    await load()
  } catch (e) {
    err.value = e.message
  } finally {
    installing.value = false
  }
}

async function applyAll() {
  applying.value = true
  err.value = ''
  try {
    await api.lvs.put(cfg.value)
    await api.lvs.apply()
    ok.value = t('nat.lvs.applied')
    await load()
  } catch (e) {
    err.value = e.message
  } finally {
    applying.value = false
  }
}

async function addVS() {
  err.value = ''
  ok.value = ''
  try {
    const real_servers = parseRealServers(rsText.value, form.value.port)
    if (!real_servers.length) {
      err.value = t('nat.lvs.needRealServers')
      return
    }
    await api.lvs.addVirtualServer({
      ...form.value,
      wan_device: devWan.value,
      real_servers,
    })
    ok.value = t('nat.lvs.added')
    await load()
  } catch (e) {
    err.value = e.message
  }
}

async function addOcservCluster() {
  err.value = ''
  ok.value = ''
  try {
    const nodes = parseNodeIPs(ocservNodesText.value)
    if (!nodes.length) {
      err.value = t('nat.lvs.needRealServers')
      return
    }
    await api.lvs.addOcservCluster({
      ...ocservForm.value,
      nodes,
    })
    ok.value = t('nat.lvs.ocservAdded')
    await load()
  } catch (e) {
    err.value = e.message
  }
}

async function addRsToVS(vs) {
  err.value = ''
  ok.value = ''
  const line = getRsDraft(vs.id).trim()
  if (!line) {
    err.value = t('nat.lvs.needRealServers')
    return
  }
  const parsed = parseRealServers(line, vs.port)
  if (!parsed.length) {
    err.value = t('nat.lvs.needRealServers')
    return
  }
  try {
    await api.lvs.addRealServer({ vs_id: vs.id, real_servers: parsed })
    setRsDraft(vs.id, '')
    ok.value = t('nat.lvs.rsAdded')
    await load()
  } catch (e) {
    err.value = e.message
  }
}

async function removeRs(vs, rs) {
  if ((vs.real_servers || []).length <= 1) {
    err.value = t('nat.lvs.cannotRemoveLastRs')
    return
  }
  if (!confirm(t('nat.lvs.confirmRemoveRs'))) return
  err.value = ''
  try {
    await api.lvs.delRealServer(vs.id, rs.ip, rs.port)
    ok.value = t('nat.lvs.rsRemoved')
    await load()
  } catch (e) {
    err.value = e.message
  }
}

async function removeVS(id) {
  if (!confirm(t('nat.lvs.confirmDelete'))) return
  err.value = ''
  try {
    await api.lvs.delVirtualServer(id)
    await load()
  } catch (e) {
    err.value = e.message
  }
}

function protocolLabel(vs) {
  if (vs.service === 'ocserv' || vs.protocol === 'tcp_udp') return 'TCP + UDP'
  return vs.protocol
}

onMounted(load)
</script>

<template>
  <div class="page-stack">
    <PageHeader :title="t('nat.lvs.title')" :description="t('nat.lvs.description')" :ok="ok" :err="err" />

    <div v-for="(w, i) in warnings" :key="i" class="card card-body text-xs text-amber-800 bg-amber-50">
      {{ t('nat.lvs.localOcservWarn') }} {{ w }}
    </div>

    <div class="card card-body space-y-3 text-sm">
      <div class="flex flex-wrap items-center gap-2 text-xs">
        <span
          class="inline-flex px-2 py-0.5 rounded"
          :class="status.installed ? 'bg-emerald-100 text-emerald-800' : 'bg-amber-100 text-amber-800'"
        >
          {{ status.installed ? t('nat.lvs.ipvsadmOk') : t('nat.lvs.ipvsadmMissing') }}
        </span>
        <span v-if="status.summary" class="text-slate-500">{{ status.summary }}</span>
      </div>
      <p class="text-xs text-slate-600">{{ t('nat.lvs.modeHint') }}</p>
      <div v-if="!status.installed" class="flex gap-2">
        <button type="button" class="btn-primary" :disabled="installing" @click="installIpvsadm">
          {{ installing ? t('nat.lvs.installing') : t('nat.lvs.install') }}
        </button>
      </div>
      <div class="flex flex-wrap gap-3 items-center">
        <label class="flex items-center gap-2">
          <input v-model="cfg.enabled" type="checkbox" />
          {{ t('nat.lvs.enabled') }}
        </label>
        <label class="flex items-center gap-2">
          <span class="text-xs text-slate-500">{{ t('nat.lvs.mode') }}</span>
          <select v-model="cfg.mode" class="input-field text-xs">
            <option value="nat">NAT</option>
            <option value="dr">DR</option>
          </select>
        </label>
        <button type="button" class="btn-primary" :disabled="applying" @click="applyAll">
          {{ t('nat.lvs.saveApply') }}
        </button>
      </div>
    </div>

    <div class="card card-body space-y-3 text-sm">
      <h3 class="font-medium">{{ t('nat.lvs.ocservClusterTitle') }}</h3>
      <p class="text-xs text-slate-600">{{ t('nat.lvs.ocservClusterHint') }}</p>
      <p class="text-xs text-amber-800 bg-amber-50 p-2 rounded">{{ t('nat.lvs.ocservProxyProtoHint') }}</p>
      <form class="grid md:grid-cols-2 lg:grid-cols-4 gap-3 items-end" @submit.prevent="addOcservCluster">
        <div>
          <label class="text-xs text-slate-500">VIP</label>
          <input v-model="ocservForm.vip" class="input-field font-mono mt-0.5" placeholder="203.0.113.10" />
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('nat.lvs.port') }}</label>
          <input v-model.number="ocservForm.port" type="number" min="1" max="65535" class="input-field mt-0.5" />
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('nat.lvs.persistence') }}</label>
          <input v-model.number="ocservForm.persistence_sec" type="number" min="0" class="input-field mt-0.5" />
        </div>
        <div class="md:col-span-2">
          <label class="text-xs text-slate-500">{{ t('nat.lvs.ocservNodes') }}</label>
          <textarea
            v-model="ocservNodesText"
            class="input-field font-mono text-xs min-h-[4rem] mt-0.5"
            :placeholder="t('nat.lvs.ocservNodesPh')"
          />
        </div>
        <div class="flex flex-col gap-2">
          <label class="flex items-center gap-2 text-xs">
            <input v-model="ocservForm.auto_vip" type="checkbox" />
            {{ t('nat.lvs.autoVip') }}
          </label>
          <button type="submit" class="btn-secondary">{{ t('nat.lvs.addOcservCluster') }}</button>
        </div>
      </form>
    </div>

    <form class="card card-body grid md:grid-cols-2 lg:grid-cols-4 gap-3 items-end text-sm" @submit.prevent="addVS">
      <div>
        <label class="text-xs text-slate-500">VIP</label>
        <input v-model="form.vip" class="input-field font-mono mt-0.5" placeholder="203.0.113.10" />
      </div>
      <div>
        <label class="text-xs text-slate-500">{{ t('nat.lvs.port') }}</label>
        <input v-model.number="form.port" type="number" min="1" max="65535" class="input-field mt-0.5" />
      </div>
      <div>
        <label class="text-xs text-slate-500">{{ t('nat.forwards.proto') }}</label>
        <select v-model="form.protocol" class="input-field mt-0.5">
          <option value="tcp">TCP</option>
          <option value="udp">UDP</option>
          <option value="tcp_udp">TCP + UDP</option>
        </select>
      </div>
      <div>
        <label class="text-xs text-slate-500">{{ t('nat.lvs.scheduler') }}</label>
        <select v-model="form.scheduler" class="input-field mt-0.5">
          <option value="rr">rr</option>
          <option value="wlc">wlc</option>
          <option value="sh">sh</option>
        </select>
      </div>
      <div class="md:col-span-2">
        <label class="text-xs text-slate-500">{{ t('nat.lvs.realServers') }}</label>
        <textarea v-model="rsText" class="input-field font-mono text-xs min-h-[4rem] mt-0.5" />
        <p class="text-xs text-slate-500 mt-1">{{ t('nat.lvs.realServersHint') }}</p>
      </div>
      <div>
        <label class="text-xs text-slate-500">{{ t('nat.lvs.persistence') }}</label>
        <input v-model.number="form.persistence_sec" type="number" min="0" class="input-field mt-0.5" />
      </div>
      <div class="flex flex-col gap-2">
        <label class="flex items-center gap-2 text-xs">
          <input v-model="form.auto_vip" type="checkbox" />
          {{ t('nat.lvs.autoVip') }}
        </label>
        <button type="submit" class="btn-secondary">{{ t('nat.lvs.add') }}</button>
      </div>
    </form>

    <div class="card table-wrap card-body !p-2">
      <table class="data w-full text-sm">
        <thead>
          <tr>
            <th>VIP</th>
            <th>{{ t('nat.lvs.port') }}</th>
            <th>{{ t('nat.forwards.proto') }}</th>
            <th>{{ t('nat.lvs.scheduler') }}</th>
            <th>{{ t('nat.lvs.backends') }}</th>
            <th class="w-24 text-right">{{ t('common.actions') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="vs in cfg.virtual_servers || []" :key="vs.id">
            <td class="font-mono">{{ vs.vip }}</td>
            <td>{{ vs.port }}</td>
            <td>
              {{ protocolLabel(vs) }}
              <span v-if="vs.service === 'ocserv'" class="text-xs text-slate-500"> (OCServ)</span>
            </td>
            <td>{{ vs.scheduler }}</td>
            <td class="font-mono text-xs align-top">
              <ul class="space-y-1 mb-2">
                <li v-for="(rs, i) in vs.real_servers" :key="i" class="flex items-center justify-between gap-2">
                  <span>{{ rs.ip }}:{{ rs.port }}</span>
                  <button
                    type="button"
                    class="text-red-600 text-xs shrink-0"
                    :disabled="(vs.real_servers || []).length <= 1"
                    @click="removeRs(vs, rs)"
                  >
                    {{ t('common.delete') }}
                  </button>
                </li>
              </ul>
              <div class="flex gap-1">
                <input
                  :value="getRsDraft(vs.id)"
                  class="input-field font-mono text-xs flex-1 min-w-0"
                  :placeholder="t('nat.lvs.addRsPh')"
                  @input="setRsDraft(vs.id, $event.target.value)"
                  @keyup.enter="addRsToVS(vs)"
                />
                <button type="button" class="btn-secondary text-xs shrink-0" @click="addRsToVS(vs)">
                  {{ t('nat.lvs.addRs') }}
                </button>
              </div>
            </td>
            <td class="text-right">
              <button type="button" class="btn-danger" @click="removeVS(vs.id)">{{ t('common.delete') }}</button>
            </td>
          </tr>
          <tr v-if="!(cfg.virtual_servers || []).length">
            <td colspan="6" class="text-center text-slate-400 py-6">{{ t('nat.lvs.noRules') }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
