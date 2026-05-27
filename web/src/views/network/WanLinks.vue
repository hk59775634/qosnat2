<script setup>
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'

const { t } = useI18n()
const links = ref([])
const egress = ref([])
const resolved = ref([])
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
  wan_link_id: '',
  snat_ip: '',
  priority: 100,
  enabled: true,
})

const linkOptions = computed(() =>
  (links.value || []).filter((w) => w.enabled).map((w) => ({ id: w.id, label: `${w.name} (${w.device})` }))
)

function resolvedRow(policyId) {
  return resolved.value.find((r) => r.policy?.id === policyId)
}

async function load() {
  const [wan, eg] = await Promise.all([api.network.wanLinks.list(), api.network.egressPolicies.list()])
  links.value = wan.wan_links || []
  devWan.value = wan.dev_wan || ''
  egress.value = eg.egress_policies || []
  resolved.value = eg.resolved || []
  if (!form.value.device && devWan.value) form.value.device = devWan.value
  if (!egForm.value.wan_link_id && links.value.length) {
    const us = links.value.find((w) => w.device === 'ens19') || links.value[0]
    egForm.value.wan_link_id = us.id
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
            <td>{{ w.name }}</td>
            <td class="font-mono">{{ w.device }}</td>
            <td class="font-mono">{{ w.gateway }}</td>
            <td>{{ w.tier }}</td>
            <td>{{ w.metric }}</td>
            <td>{{ w.weight }}</td>
            <td><button type="button" class="text-red-600 text-xs" @click="remove(w.id)">{{ t('common.delete') }}</button></td>
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
          <label class="text-xs text-slate-500">{{ t('network.wanLinks.sourceCidr') }}</label>
          <input v-model="egForm.cidr" class="input-field mt-1 font-mono" placeholder="10.250.0.0/24" />
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
      <button type="button" class="btn-primary" :disabled="!egForm.wan_link_id" @click="addEgress">{{ t('common.add') }}</button>
    </div>

    <div class="table-wrap card">
      <table class="data w-full text-sm">
        <thead>
          <tr>
            <th>{{ t('common.name') }}</th>
            <th>{{ t('network.wanLinks.sourceCidr') }}</th>
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
            <td colspan="7" class="text-center text-slate-400 py-3">{{ t('network.wanLinks.noEgress') }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
