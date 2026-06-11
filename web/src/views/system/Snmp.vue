<script setup>
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'

const { t } = useI18n()
const cfg = ref({
  enabled: false,
  port: 161,
  listen_localhost_only: true,
  sys_location: '',
  sys_contact: '',
  sys_name: '',
  ro_community: '',
  allowed_networks: ['127.0.0.1/32'],
})
const status = ref({})
const rendered = ref('')
const allowedText = ref('127.0.0.1/32')
const snmpRoot = ref(false)
const err = ref('')
const ok = ref('')
const saving = ref(false)
const applying = ref(false)
const installing = ref(false)

const statusLabel = computed(() => {
  if (!status.value?.installed) return t('system.snmp.notInstalled')
  return status.value.active ? t('system.snmp.running') : t('system.snmp.stopped')
})

function syncAllowedText() {
  allowedText.value = (cfg.value.allowed_networks || []).join('\n')
}

function applyAllowedText() {
  cfg.value.allowed_networks = allowedText.value
    .split(/[\n,]+/)
    .map((s) => s.trim())
    .filter(Boolean)
}

async function load() {
  const d = await api.snmp.get()
  cfg.value = { ...cfg.value, ...(d.config || {}) }
  status.value = d.status || {}
  rendered.value = d.rendered || ''
  snmpRoot.value = !!d.root
  syncAllowedText()
}

async function installSnmpd() {
  err.value = ''
  ok.value = ''
  if (!snmpRoot.value) {
    err.value = t('system.snmp.needRoot')
    return
  }
  installing.value = true
  try {
    await api.snmp.install()
    ok.value = t('system.snmp.installedOk')
    await load()
  } catch (e) {
    err.value = e.message
  } finally {
    installing.value = false
  }
}

async function save() {
  saving.value = true
  err.value = ''
  ok.value = ''
  try {
    applyAllowedText()
    await api.snmp.put({ ...cfg.value })
    ok.value = t('system.snmp.saved')
    await load()
  } catch (e) {
    err.value = e.message
  } finally {
    saving.value = false
  }
}

async function apply() {
  applying.value = true
  err.value = ''
  ok.value = ''
  try {
    applyAllowedText()
    await api.snmp.put({ ...cfg.value })
    await api.snmp.apply()
    ok.value = t('system.snmp.applied')
    await load()
  } catch (e) {
    err.value = e.message
  } finally {
    applying.value = false
  }
}

async function service(action) {
  err.value = ''
  try {
    await api.snmp.service(action)
    ok.value = t('system.snmp.serviceDone')
    await load()
  } catch (e) {
    err.value = e.message
  }
}

onMounted(load)
</script>

<template>
  <div class="page-stack">
    <PageHeader :title="t('system.snmp.title')" :description="t('system.snmp.description')" :ok="ok" :err="err" />

    <div class="card card-body space-y-4">
      <div class="flex flex-wrap items-center gap-2 text-xs">
        <span
          class="inline-flex px-2 py-0.5 rounded"
          :class="status.installed ? 'bg-emerald-100 text-emerald-800' : 'bg-amber-100 text-amber-800'"
        >
          {{ statusLabel }}
        </span>
        <span v-if="status.version" class="text-slate-500 font-mono">{{ status.version }}</span>
      </div>

      <div v-if="!status.installed" class="flex flex-wrap gap-2">
        <button type="button" class="btn-primary" :disabled="installing" @click="installSnmpd">
          {{ installing ? t('system.snmp.installing') : t('system.snmp.install') }}
        </button>
      </div>

      <label class="flex items-center gap-2 text-sm">
        <input v-model="cfg.enabled" type="checkbox" />
        {{ t('system.snmp.enabled') }}
      </label>

      <div class="grid sm:grid-cols-2 gap-3 text-sm">
        <div>
          <label class="text-xs text-slate-500">{{ t('system.snmp.port') }}</label>
          <input v-model.number="cfg.port" type="number" class="input-field font-mono" min="1" max="65535" />
        </div>
        <div class="flex items-end">
          <label class="flex items-center gap-2 text-sm">
            <input v-model="cfg.listen_localhost_only" type="checkbox" />
            {{ t('system.snmp.localhostOnly') }}
          </label>
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('system.snmp.roCommunity') }}</label>
          <input v-model="cfg.ro_community" type="password" class="input-field font-mono" autocomplete="new-password" />
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('system.snmp.sysName') }}</label>
          <input v-model="cfg.sys_name" class="input-field" />
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('system.snmp.sysLocation') }}</label>
          <input v-model="cfg.sys_location" class="input-field" />
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('system.snmp.sysContact') }}</label>
          <input v-model="cfg.sys_contact" class="input-field" />
        </div>
        <div class="sm:col-span-2">
          <label class="text-xs text-slate-500">{{ t('system.snmp.allowedNetworks') }}</label>
          <textarea
            v-model="allowedText"
            class="input-field font-mono text-xs min-h-[4rem]"
            :placeholder="t('system.snmp.allowedNetworksPh')"
          />
          <p class="text-xs text-slate-500 mt-1">{{ t('system.snmp.allowedNetworksHint') }}</p>
        </div>
      </div>

      <div class="flex flex-wrap gap-2">
        <button type="button" class="btn-secondary" :disabled="saving" @click="save">{{ t('common.save') }}</button>
        <button type="button" class="btn-primary" :disabled="applying" @click="apply">
          {{ t('system.snmp.saveApply') }}
        </button>
        <template v-if="status.installed">
          <button type="button" class="btn-secondary" @click="service('start')">{{ t('system.snmp.start') }}</button>
          <button type="button" class="btn-secondary" @click="service('stop')">{{ t('system.snmp.stop') }}</button>
          <button type="button" class="btn-secondary" @click="service('restart')">{{ t('system.snmp.restart') }}</button>
        </template>
      </div>
    </div>

    <div v-if="rendered" class="card card-body">
      <h3 class="font-medium text-sm mb-2">{{ t('system.snmp.preview') }}</h3>
      <pre class="text-xs font-mono bg-slate-50 p-3 rounded overflow-x-auto whitespace-pre-wrap">{{ rendered }}</pre>
    </div>
  </div>
</template>
