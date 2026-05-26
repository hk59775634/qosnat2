<script setup>
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/api/client'
import { setDisplayName } from '@/composables/useBranding'
import PageHeader from '@/components/PageHeader.vue'
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

async function load() {
  cfg.value = await api.system.general.get()
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

function buildPutBody() {
  return {
    hostname: form.value.hostname,
    display_name: form.value.display_name,
    admin_port: form.value.admin_port || undefined,
    new_password: form.value.new_password || undefined,
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
      form.value.new_password ||
      adminPortTouched() ||
      (tlsSettingsTouched() && (form.value.tls_enabled || form.value.tls_acme_enabled || form.value.tls_cert))
    if (needPw && !form.value.current_password) {
      err.value = t('system.general.needPasswordForTls')
      return
    }
    const res = await api.system.general.put(buildPutBody())
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
    await api.system.general.put(buildPutBody())
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

onMounted(load)
</script>

<template>
  <div class="page-stack">
    <PageHeader
      :title="t('system.general.title')"
      :description="t('system.general.description')"
    />
    <p v-if="ok" class="text-green-700 text-sm mb-2">{{ ok }}</p>
    <p v-if="warn" class="text-amber-700 text-sm mb-2">{{ warn }}</p>
    <p v-if="err" class="text-red-600 text-sm mb-2">{{ err }}</p>

    <div v-if="cfg" class="card card-body space-y-6">
      <section class="space-y-4">
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
      </section>

      <section class="space-y-4 pt-4 border-t border-slate-200">
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

      <button type="button" class="btn-primary" @click="save">{{ t('common.save') }}</button>
    </div>
  </div>
</template>
