<script setup>
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'

const { t } = useI18n()
const items = ref([])
const err = ref('')
const ok = ref('')
const busy = ref(false)

const formMode = ref('acme')
const manual = ref({ name: '', cert_pem: '', key_pem: '', ca_pem: '' })
const acme = ref({ name: '', domain: '', email: '', staging: false })

async function load() {
  err.value = ''
  try {
    const data = await api.system.certificates.list()
    items.value = data.certificates || []
  } catch (e) {
    err.value = e.message
  }
}

async function create() {
  busy.value = true
  err.value = ''
  ok.value = ''
  try {
    const body =
      formMode.value === 'acme'
        ? {
            type: 'acme',
            name: acme.value.name || acme.value.domain,
            domain: acme.value.domain,
            email: acme.value.email,
            staging: acme.value.staging,
          }
        : {
            type: 'manual',
            name: manual.value.name,
            cert_pem: manual.value.cert_pem,
            key_pem: manual.value.key_pem,
            ca_pem: manual.value.ca_pem,
          }
    await api.system.certificates.create(body)
    ok.value = t('certificates.created')
    manual.value = { name: '', cert_pem: '', key_pem: '', ca_pem: '' }
    acme.value = { name: '', domain: '', email: '', staging: false }
    await load()
  } catch (e) {
    err.value = e.data?.error || e.message
  } finally {
    busy.value = false
  }
}

function renewFailMessage(e) {
  const summary = e.data?.renew_error_summary
  const raw = e.data?.error || e.message
  return summary || raw
}

async function renew(id) {
  busy.value = true
  err.value = ''
  ok.value = ''
  try {
    await api.system.certificates.renew(id)
    ok.value = t('certificates.renewed')
    await load()
  } catch (e) {
    err.value = renewFailMessage(e)
  } finally {
    busy.value = false
  }
}

async function resumeAutoRenew(c) {
  busy.value = true
  err.value = ''
  ok.value = ''
  try {
    await api.system.certificates.setAutoRenew(c.id, true)
    ok.value = t('certificates.autoRenewOn')
    await load()
  } catch (e) {
    err.value = e.data?.error || e.message
  } finally {
    busy.value = false
  }
}

function autoRenewLabel(c) {
  if (c.type !== 'acme') return '—'
  if (c.auto_renew_paused) return t('certificates.autoRenewPaused')
  if (c.auto_renew_enabled) return t('certificates.autoRenewOn')
  return t('certificates.autoRenewOff')
}

async function remove(c) {
  if (c.in_use?.length) {
    err.value = t('certificates.inUse', { places: c.in_use.map((u) => u.label || u.place).join(', ') })
    return
  }
  if (!confirm(t('certificates.deleteConfirm', { name: c.name }))) return
  busy.value = true
  err.value = ''
  try {
    await api.system.certificates.del(c.id)
    ok.value = t('certificates.deleted')
    await load()
  } catch (e) {
    err.value = e.data?.error || e.message
  } finally {
    busy.value = false
  }
}

onMounted(load)
</script>

<template>
  <div class="space-y-4">
    <PageHeader :title="t('certificates.title')" :subtitle="t('certificates.subtitle')" :ok="ok" :err="err" />
    <p class="text-xs text-slate-500 -mt-2">{{ t('certificates.acmeHint') }}</p>

    <p v-if="err" class="text-sm text-red-600">{{ err }}</p>
    <p v-if="ok" class="text-sm text-green-700">{{ ok }}</p>

    <div class="card p-4 space-y-3">
      <h3 class="text-sm font-medium">{{ t('certificates.addTitle') }}</h3>
      <div class="flex flex-wrap gap-4 text-sm">
        <label class="flex gap-2"><input v-model="formMode" type="radio" value="acme" /> {{ t('certificates.typeAcme') }}</label>
        <label class="flex gap-2"><input v-model="formMode" type="radio" value="manual" /> {{ t('certificates.typeManual') }}</label>
      </div>

      <div v-if="formMode === 'acme'" class="grid gap-3 sm:grid-cols-2">
        <label class="text-sm">{{ t('certificates.name') }}<input v-model="acme.name" class="input w-full mt-1" :placeholder="acme.domain" /></label>
        <label class="text-sm">{{ t('certificates.domain') }}<input v-model="acme.domain" class="input w-full mt-1" /></label>
        <label class="text-sm">{{ t('certificates.email') }}<input v-model="acme.email" type="email" class="input w-full mt-1" /></label>
        <label class="flex gap-2 text-sm items-end pb-1"><input v-model="acme.staging" type="checkbox" /> {{ t('certificates.staging') }}</label>
        <p class="sm:col-span-2 text-xs text-slate-500">{{ t('certificates.acmeHint') }}</p>
      </div>

      <div v-else-if="formMode === 'manual'" class="grid gap-3">
        <label class="text-sm">{{ t('certificates.name') }}<input v-model="manual.name" class="input w-full mt-1" /></label>
        <label class="text-sm">{{ t('certificates.certPem') }}<textarea v-model="manual.cert_pem" class="input w-full mt-1 font-mono text-xs" rows="6" /></label>
        <label class="text-sm">{{ t('certificates.keyPem') }}<textarea v-model="manual.key_pem" class="input w-full mt-1 font-mono text-xs" rows="4" /></label>
        <label class="text-sm">{{ t('certificates.caPem') }}<textarea v-model="manual.ca_pem" class="input w-full mt-1 font-mono text-xs" rows="3" /></label>
      </div>

      <button type="button" class="btn-primary" :disabled="busy" @click="create">
        {{ formMode === 'acme' ? t('certificates.addBtnAcme') : t('certificates.addBtn') }}
      </button>
    </div>

    <div class="card overflow-x-auto">
      <table class="w-full text-sm">
        <thead class="bg-slate-50 text-left">
          <tr>
            <th class="p-2">{{ t('certificates.name') }}</th>
            <th class="p-2">{{ t('certificates.type') }}</th>
            <th class="p-2">{{ t('certificates.domain') }}</th>
            <th class="p-2">{{ t('certificates.expires') }}</th>
            <th class="p-2">{{ t('certificates.autoRenewCol') }}</th>
            <th class="p-2">{{ t('certificates.inUseCol') }}</th>
            <th class="p-2"></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="c in items" :key="c.id" class="border-t align-top">
            <td class="p-2 font-medium">
              <div>{{ c.name }}</div>
              <p v-if="c.acme_last_error" class="text-xs text-red-600 mt-1 max-w-xs">
                <span class="font-medium">{{ t('certificates.renewFailTitle') }}:</span>
                {{ c.auto_renew_pause_reason || c.acme_last_error }}
              </p>
              <p v-if="c.auto_renew_paused" class="text-xs text-amber-700 mt-1 max-w-sm">{{ t('certificates.autoRenewPauseHint') }}</p>
            </td>
            <td class="p-2">{{ c.type }}</td>
            <td class="p-2 font-mono text-xs">{{ (c.domains || []).join(', ') }}</td>
            <td class="p-2">
              <span v-if="c.not_after">{{ c.not_after.slice(0, 10) }}</span>
              <span v-if="c.days_until_expiry >= 0" class="text-xs text-slate-500"> ({{ c.days_until_expiry }}d)</span>
            </td>
            <td class="p-2 text-xs">
              <span :class="c.auto_renew_paused ? 'text-amber-700' : 'text-slate-600'">{{ autoRenewLabel(c) }}</span>
            </td>
            <td class="p-2 text-xs">
              <span v-if="!c.in_use?.length">—</span>
              <span v-else>{{ c.in_use.map((u) => u.label || u.place).join(', ') }}</span>
            </td>
            <td class="p-2 whitespace-nowrap space-x-2">
              <button v-if="c.type === 'acme'" type="button" class="btn-secondary text-xs" :disabled="busy" @click="renew(c.id)">
                {{ t('certificates.renew') }}
              </button>
              <button
                v-if="c.type === 'acme' && c.auto_renew_paused"
                type="button"
                class="btn-secondary text-xs"
                :disabled="busy"
                @click="resumeAutoRenew(c)"
              >
                {{ t('certificates.autoRenewResume') }}
              </button>
              <button type="button" class="text-red-600 text-xs" :disabled="busy" @click="remove(c)">{{ t('common.delete') }}</button>
            </td>
          </tr>
          <tr v-if="!items.length">
            <td colspan="7" class="p-4 text-center text-slate-500">{{ t('certificates.noCerts') }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
