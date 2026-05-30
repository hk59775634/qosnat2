<script setup>
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'
import StatusBadge from '@/components/StatusBadge.vue'

const { t } = useI18n()

const status = ref({})
const nptv6Enabled = ref(false)
const nptv6Rules = ref([])
const nat64Enabled = ref(false)
const nat64Prefix = ref('64:ff9b::/96')
const nat64Pool4 = ref('10.255.255.0/24')
const dns64Mode = ref('local_unbound')
const dns64Upstream = ref('')
const dns64Forwarders = ref('')
const dns64Serve = ref(true)
const dns64AccessAllow = ref('')
const dns64Listen = ref('')
const newNpt = ref({ internal_prefix: '', external_prefix: '', description: '', oif: '' })
const msg = ref('')
const err = ref('')

const joolOk = computed(() => !nat64Enabled.value || !!status.value?.jool_active)
const unboundOk = computed(() => {
  if (!nat64Enabled.value) return true
  if (dns64Mode.value !== 'local_unbound') return true
  return !!status.value?.unbound_active
})
const joolMissing = computed(() => nat64Enabled.value && status.value?.jool_installed === false)
const unboundMissing = computed(
  () => nat64Enabled.value && dns64Mode.value === 'local_unbound' && status.value?.unbound_installed === false,
)

async function load() {
  msg.value = ''
  err.value = ''
  const [sum, npt, n64] = await Promise.all([
    api.nat.summary(),
    api.nat.nptv6.get(),
    api.nat.nat64.get(),
  ])
  status.value = sum.status || n64.status || {}
  nptv6Enabled.value = !!npt.nptv6_enabled
  nptv6Rules.value = npt.nptv6_rules || []
  nat64Enabled.value = !!n64.nat64_enabled
  nat64Prefix.value = n64.nat64_prefix || '64:ff9b::/96'
  nat64Pool4.value = n64.nat64_pool4 || '10.255.255.0/24'
  const d = n64.dns64 || {}
  dns64Mode.value = d.mode || 'local_unbound'
  dns64Serve.value = d.serve_to_clients !== false
  dns64Upstream.value = (d.upstream || []).join('\n')
  dns64Forwarders.value = (d.forwarders || ['1.1.1.1', '8.8.8.8']).join('\n')
  dns64AccessAllow.value = (d.access_allow || []).join('\n')
  dns64Listen.value = displayUnboundListen(d.unbound_listen, d.serve_to_clients !== false)
}

function isLoopbackListen(s) {
  const x = (s || '').trim().toLowerCase()
  return x.startsWith('127.') || x === '::1' || x.startsWith('[::1]')
}

function displayUnboundListen(raw, serveToClients) {
  const s = (raw || '').trim()
  if (!serveToClients && isLoopbackListen(s)) return ''
  return s
}

function parseLines(s) {
  return s
    .split(/[\n,]+/)
    .map((x) => x.trim())
    .filter(Boolean)
}

async function saveNptv6() {
  err.value = ''
  msg.value = ''
  if (nptv6Enabled.value && nptv6Rules.value.length === 0) {
    err.value = t('nat.ipv6.nptNeedRule')
    return
  }
  try {
    await api.nat.nptv6.put({
      nptv6_enabled: nptv6Enabled.value,
      nptv6_rules: nptv6Rules.value,
    })
    msg.value = t('common.saved')
    await load()
  } catch (e) {
    err.value = e.message
  }
}

async function saveNat64() {
  err.value = ''
  msg.value = ''
  try {
    await api.nat.nat64.put({
      nat64_enabled: nat64Enabled.value,
      nat64_prefix: nat64Prefix.value.trim(),
      nat64_pool4: nat64Pool4.value.trim(),
      dns64: {
        mode: dns64Mode.value,
        upstream: parseLines(dns64Upstream.value),
        forwarders: parseLines(dns64Forwarders.value),
        serve_to_clients: dns64Serve.value,
        access_allow: parseLines(dns64AccessAllow.value),
        unbound_listen: dns64Serve.value
          ? dns64Listen.value.trim() || '127.0.0.1:5353'
          : dns64Listen.value.trim(),
      },
    })
    msg.value = t('common.saved')
    await load()
  } catch (e) {
    err.value = e.message
  }
}

function addNptRule() {
  const internal = newNpt.value.internal_prefix.trim()
  const external = newNpt.value.external_prefix.trim()
  if (!internal || !external) {
    err.value = t('nat.ipv6.nptNeedPrefixes')
    return
  }
  err.value = ''
  nptv6Rules.value.push({
    id: `nptv6-${Date.now()}`,
    internal_prefix: internal,
    external_prefix: external,
    description: newNpt.value.description.trim(),
    oif: newNpt.value.oif.trim(),
  })
  newNpt.value = { internal_prefix: '', external_prefix: '', description: '', oif: '' }
}

function removeNptRule(i) {
  nptv6Rules.value.splice(i, 1)
}

onMounted(load)
</script>

<template>
  <div class="page-stack">
    <PageHeader :title="t('nat.ipv6.title')" :description="t('nat.ipv6.description')" :err="err" />
    <p v-if="msg" class="text-green-700 text-sm">{{ msg }}</p>

    <div class="card p-4 space-y-4">
      <div class="flex flex-wrap items-center gap-3">
        <h2 class="text-lg font-semibold">{{ t('nat.ipv6.nptTitle') }}</h2>
        <label class="inline-flex items-center gap-2 text-sm">
          <input v-model="nptv6Enabled" type="checkbox" class="rounded" />
          {{ t('nat.ipv6.enabled') }}
        </label>
      </div>
      <p class="text-sm text-slate-600">{{ t('nat.ipv6.nptHint') }}</p>

      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="text-left text-slate-500 border-b">
              <th class="py-2">{{ t('nat.ipv6.internalPrefix') }}</th>
              <th>{{ t('nat.ipv6.externalPrefix') }}</th>
              <th>{{ t('nat.ipv6.oif') }}</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(r, i) in nptv6Rules" :key="r.id || i" class="border-b border-slate-100">
              <td class="py-2 font-mono text-xs">{{ r.internal_prefix }}</td>
              <td class="font-mono text-xs">{{ r.external_prefix }}</td>
              <td class="font-mono text-xs">{{ r.oif || '—' }}</td>
              <td>
                <button type="button" class="text-red-600 text-xs" @click="removeNptRule(i)">
                  {{ t('common.delete') }}
                </button>
              </td>
            </tr>
            <tr v-if="!nptv6Rules.length">
              <td colspan="4" class="py-3 text-slate-500">{{ t('nat.ipv6.noNptRules') }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="grid md:grid-cols-2 lg:grid-cols-4 gap-3">
        <input v-model="newNpt.internal_prefix" class="input-field font-mono text-sm" :placeholder="t('nat.ipv6.internalPrefixPh')" />
        <input v-model="newNpt.external_prefix" class="input-field font-mono text-sm" :placeholder="t('nat.ipv6.externalPrefixPh')" />
        <input v-model="newNpt.oif" class="input-field font-mono text-sm" :placeholder="t('nat.ipv6.oifPh')" />
        <button type="button" class="btn-secondary" @click="addNptRule">{{ t('nat.ipv6.addNptRule') }}</button>
      </div>
      <button type="button" class="btn-primary" @click="saveNptv6">{{ t('nat.ipv6.applyNpt') }}</button>
    </div>

    <div class="card p-4 space-y-4">
      <div class="flex flex-wrap items-center gap-3">
        <h2 class="text-lg font-semibold">{{ t('nat.ipv6.nat64Title') }}</h2>
        <label class="inline-flex items-center gap-2 text-sm">
          <input v-model="nat64Enabled" type="checkbox" class="rounded" />
          {{ t('nat.ipv6.enabled') }}
        </label>
      </div>
      <p class="text-sm text-slate-600">{{ t('nat.ipv6.nat64Hint') }}</p>

      <p v-if="joolMissing || unboundMissing" class="text-amber-700 text-sm">
        {{ t('nat.ipv6.depsMissing') }}
      </p>
      <div v-if="nat64Enabled" class="flex flex-wrap gap-4">
        <StatusBadge
          :label="t('nat.ipv6.jool')"
          :ok="joolOk"
          :detail="status.jool_installed === false ? t('nat.ipv6.notInstalled') : ''"
        />
        <StatusBadge
          v-if="dns64Mode === 'local_unbound'"
          :label="t('nat.ipv6.unbound')"
          :ok="unboundOk"
          :detail="status.unbound_installed === false ? t('nat.ipv6.notInstalled') : ''"
        />
      </div>

      <div class="grid md:grid-cols-2 gap-4">
        <div>
          <label class="text-xs text-slate-500">{{ t('nat.ipv6.nat64Prefix') }}</label>
          <input v-model="nat64Prefix" class="input-field mt-1 font-mono text-sm" />
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('nat.ipv6.nat64Pool4') }}</label>
          <input v-model="nat64Pool4" class="input-field mt-1 font-mono text-sm" />
        </div>
      </div>

      <div>
        <p class="text-sm font-medium mb-2">{{ t('nat.ipv6.dns64Title') }}</p>
        <div class="flex flex-wrap gap-4 text-sm mb-3">
          <label class="inline-flex items-center gap-2">
            <input v-model="dns64Mode" type="radio" value="local_unbound" />
            {{ t('nat.ipv6.dnsLocal') }}
          </label>
          <label class="inline-flex items-center gap-2">
            <input v-model="dns64Mode" type="radio" value="upstream" />
            {{ t('nat.ipv6.dnsUpstream') }}
          </label>
        </div>
        <label class="inline-flex items-center gap-2 text-sm mb-3">
          <input v-model="dns64Serve" type="checkbox" class="rounded" />
          {{ t('nat.ipv6.serveToClients') }}
        </label>
        <p v-if="!dns64Serve" class="text-sm text-slate-600 mb-3">{{ t('nat.ipv6.vpnDnsHint') }}</p>
        <p v-if="status.recommended_dns?.hint" class="text-xs font-mono text-slate-600 mb-2">
          {{ status.recommended_dns.hint }}
          <template v-if="status.recommended_dns.address">
            — {{ status.recommended_dns.address }}:{{ status.recommended_dns.port || 53 }}
          </template>
          <template v-if="status.recommended_dns.servers?.length">
            — {{ status.recommended_dns.servers.join(', ') }}
          </template>
        </p>
        <div v-if="dns64Mode === 'local_unbound'" class="grid gap-2">
          <label class="text-xs text-slate-500">{{ t('nat.ipv6.forwarders') }}</label>
          <textarea v-model="dns64Forwarders" class="input-field font-mono text-sm h-20" />
          <template v-if="!dns64Serve">
            <label class="text-xs text-slate-500">{{ t('nat.ipv6.unboundListen') }}</label>
            <input v-model="dns64Listen" class="input-field font-mono text-sm" :placeholder="t('nat.ipv6.unboundListenPh')" />
            <label class="text-xs text-slate-500">{{ t('nat.ipv6.accessAllow') }}</label>
            <textarea v-model="dns64AccessAllow" class="input-field font-mono text-sm h-20" :placeholder="t('nat.ipv6.accessAllowPh')" />
            <p class="text-xs text-slate-500">{{ t('nat.ipv6.dnsDirectHint') }}</p>
          </template>
          <p v-else class="text-xs text-slate-500">{{ t('nat.ipv6.dnsLocalHint') }}</p>
        </div>
        <div v-else class="grid gap-2">
          <label class="text-xs text-slate-500">{{ t('nat.ipv6.upstream') }}</label>
          <textarea v-model="dns64Upstream" class="input-field font-mono text-sm h-20" :placeholder="t('nat.ipv6.upstreamPh')" />
          <p class="text-xs text-slate-500">{{ t('nat.ipv6.dnsUpstreamHint') }}</p>
        </div>
      </div>

      <button type="button" class="btn-primary" @click="saveNat64">{{ t('nat.ipv6.applyNat64') }}</button>
    </div>
  </div>
</template>
