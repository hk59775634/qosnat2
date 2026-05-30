<script setup>
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'

const { t } = useI18n()
const managed = ref([])
const live = ref([])
const devLan = ref('')
const devWan = ref('')
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

async function load() {
  const d = await api.get('/api/v1/routes')
  managed.value = d.managed || []
  live.value = d.live || []
  devLan.value = d.dev_lan || ''
  devWan.value = d.dev_wan || ''
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
    await api.post('/api/v1/routes/apply', {})
    ok.value = t('network.routes.replayed')
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
  </div>
</template>
