<script setup>
import { computed, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { emptyVhostRadius } from '@/lib/ocservVhostForm'
import { clientMbpsFromOcserv, ocservBpsFromClientMbps } from '@/lib/ocservRate'
import OCServRadiusHelpModal from '@/components/OCServRadiusHelpModal.vue'
import CertSelect from '@/components/CertSelect.vue'
import VhostPlainUsers from '@/views/vpn/VhostPlainUsers.vue'

const props = defineProps({
  modelValue: { type: Object, required: true },
  globalCfg: { type: Object, default: null },
  editing: { type: Boolean, default: false },
  domainReadonly: { type: Boolean, default: false },
  radiusSecret: { type: String, default: '' },
  radiusSecretSet: { type: Boolean, default: false },
  camouflageSecret: { type: String, default: '' },
  camouflageSecretSet: { type: Boolean, default: false },
  connection: { type: Object, default: null },
})

const emit = defineEmits([
  'update:modelValue',
  'update:radiusSecret',
  'update:camouflageSecret',
  'users-changed',
  'connect-copied',
])

const { t } = useI18n()
const router = useRouter()
const vhostTab = ref('general')

const vhostConnectIssueText = computed(() => {
  const issue = props.connection?.issue
  if (issue === 'no_cert') return t('ocserv.connectUrlIssueNoCert')
  if (issue === 'no_hostname') return t('ocserv.connectUrlIssueNoHostname')
  if (issue === 'camouflage_secret_missing') return t('ocserv.connectUrlIssueCamoSecret')
  return ''
})

async function copyVhostConnectUrl() {
  const url = props.connection?.url
  if (!url) return
  try {
    await navigator.clipboard.writeText(url)
    emit('connect-copied')
  } catch {
    /* parent may show copy error */
  }
}

const vhostSubTabs = [
  { id: 'general', labelKey: 'ocserv.vhostTabGeneral' },
  { id: 'auth', labelKey: 'ocserv.vhostTabAuth' },
  { id: 'users', labelKey: 'ocserv.vhostTabUsers' },
  { id: 'tls', labelKey: 'ocserv.vhostTabTls' },
  { id: 'network', labelKey: 'ocserv.vhostTabNetwork' },
  { id: 'routes', labelKey: 'ocserv.vhostTabRoutes' },
  { id: 'session', labelKey: 'ocserv.vhostTabSession' },
  { id: 'protocol', labelKey: 'ocserv.vhostTabProtocol' },
  { id: 'groups', labelKey: 'ocserv.vhostTabGroups' },
  { id: 'extra', labelKey: 'ocserv.vhostTabExtra' },
]

const v = computed({
  get: () => props.modelValue,
  set: (val) => emit('update:modelValue', val),
})

const effectiveAuth = computed(() => {
  const a = (v.value.auth_method || '').trim()
  if (a) return a
  return props.globalCfg?.auth_method || 'plain'
})

const isPlain = computed(() => effectiveAuth.value === 'plain')
const vhostDomain = computed(() => String(v.value.domain || '').trim())
const groupOptions = computed(() => props.globalCfg?.groups || [])
const isRadius = computed(() => effectiveAuth.value === 'radius')
const isCert = computed(() => effectiveAuth.value === 'certificate')
const globalIsRadius = computed(() => props.globalCfg?.auth_method === 'radius')

const radiusOwn = computed({
  get: () => v.value.radius != null,
  set: (on) => {
    patch({ radius: on ? emptyVhostRadius() : null })
  },
})

watch(
  [isRadius, globalIsRadius],
  () => {
    if (isRadius.value && !globalIsRadius.value && v.value.radius == null) {
      patch({ radius: emptyVhostRadius() })
    }
  },
  { immediate: true },
)

function goOcservTab(tab) {
  router.push({ name: 'vpn-ocserv', query: { tab } })
}

const dnsText = computed({
  get: () => (v.value.dns || []).join('\n'),
  set: (s) => patch({ dns: textToList(s) }),
})
const routesText = computed({
  get: () => (v.value.routes || []).join('\n'),
  set: (s) => patch({ routes: textToList(s) }),
})
const noRoutesText = computed({
  get: () => (v.value.no_routes || []).join('\n'),
  set: (s) => patch({ no_routes: textToList(s) }),
})
const iroutesText = computed({
  get: () => (v.value.iroutes || []).join('\n'),
  set: (s) => patch({ iroutes: textToList(s) }),
})
const nbnsText = computed({
  get: () => (v.value.nbns || []).join('\n'),
  set: (s) => patch({ nbns: textToList(s) }),
})
const selectGroupsText = computed({
  get: () => (v.value.select_groups || []).join('\n'),
  set: (s) => patch({ select_groups: textToList(s) }),
})

const vhostDownMbps = computed({
  get: () => clientMbpsFromOcserv(v.value.rx_data_per_sec, v.value.tx_data_per_sec).downMbps,
  set: (m) => {
    const caps = ocservBpsFromClientMbps(m, clientMbpsFromOcserv(v.value.rx_data_per_sec, v.value.tx_data_per_sec).upMbps)
    patch(caps)
  },
})
const vhostUpMbps = computed({
  get: () => clientMbpsFromOcserv(v.value.rx_data_per_sec, v.value.tx_data_per_sec).upMbps,
  set: (m) => {
    const caps = ocservBpsFromClientMbps(
      clientMbpsFromOcserv(v.value.rx_data_per_sec, v.value.tx_data_per_sec).downMbps,
      m,
    )
    patch(caps)
  },
})
function textToList(s) {
  return String(s || '')
    .split(/[\n,]+/)
    .map((x) => x.trim())
    .filter(Boolean)
}
function patch(partial) {
  emit('update:modelValue', { ...v.value, ...partial })
}

function onVhostCertSelect(id) {
  if (id) {
    patch({
      managed_cert_id: id,
      server_cert_path: '',
      server_key_path: '',
      ca_cert_path: '',
    })
  } else {
    patch({ managed_cert_id: '' })
  }
}

function patchRadius(partial) {
  const r = { ...(v.value.radius || {}), ...partial }
  patch({ radius: r })
}
</script>

<template>
  <div class="vhost-form space-y-4">
    <nav class="flex flex-wrap gap-1 border-b border-slate-300 pb-2">
      <button
        v-for="tab in vhostSubTabs"
        :key="tab.id"
        type="button"
        class="px-2 py-1 text-xs rounded"
        :class="vhostTab === tab.id ? 'bg-slate-800 text-white' : 'text-slate-600 hover:bg-slate-100'"
        @click="vhostTab = tab.id"
      >
        {{ t(tab.labelKey) }}
      </button>
    </nav>

    <!-- General -->
    <div v-show="vhostTab === 'general'" class="space-y-4 text-sm">
      <div
        v-if="connection"
        class="rounded-lg border border-blue-100 bg-blue-50/60 p-4 space-y-2 sm:col-span-2"
      >
        <div class="flex flex-wrap items-center justify-between gap-2">
          <h4 class="text-sm font-semibold text-slate-800">{{ t('ocserv.vhostConnectUrlTitle') }}</h4>
          <button
            v-if="connection.url"
            type="button"
            class="btn-secondary text-xs"
            @click="copyVhostConnectUrl"
          >
            {{ t('common.copy') }}
          </button>
        </div>
        <p v-if="vhostConnectIssueText" class="text-xs text-amber-800">{{ vhostConnectIssueText }}</p>
        <p v-if="connection.url" class="font-mono text-sm break-all text-blue-900 select-all">
          {{ connection.url }}
        </p>
        <p v-else class="text-xs text-slate-600">{{ t('ocserv.connectUrlIssueNoHostname') }}</p>
        <p class="text-xs text-slate-600">{{ t('ocserv.vhostConnectUrlHint') }}</p>
        <dl v-if="connection.cert_hostnames?.length" class="text-xs text-slate-600 grid gap-1 mt-2">
          <div class="flex flex-wrap gap-x-2">
            <dt class="text-slate-500">{{ t('ocserv.connectUrlCertNames') }}:</dt>
            <dd class="font-mono">{{ connection.cert_hostnames.join(', ') }}</dd>
          </div>
          <div
            v-if="connection.camouflage_enabled && connection.camouflage_secret"
            class="flex flex-wrap gap-x-2"
          >
            <dt class="text-slate-500">{{ t('ocserv.connectUrlCamo') }}:</dt>
            <dd class="font-mono break-all">{{ connection.camouflage_secret }}</dd>
          </div>
        </dl>
      </div>
      <div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
      <label v-if="domainReadonly" class="sm:col-span-2">
        {{ t('ocserv.vhostDomain') }}
        <p class="font-mono font-medium text-blue-800 mt-1">{{ v.domain }}</p>
      </label>
      <label v-else>
        {{ t('ocserv.vhostDomain') }}
        <input
          :value="v.domain"
          class="input w-full mt-1"
          :disabled="editing"
          placeholder="vpn.example.com"
          @input="patch({ domain: $event.target.value })"
        />
      </label>
      <label class="flex items-end gap-2 pb-1">
        <input
          :checked="v.enabled !== false"
          type="checkbox"
          @change="patch({ enabled: $event.target.checked })"
        />
        {{ t('ocserv.vhostEnabled') }}
      </label>
      <label class="sm:col-span-2">
        {{ t('common.comment') }}
        <input :value="v.comment" class="input w-full mt-1" @input="patch({ comment: $event.target.value })" />
      </label>
      </div>
    </div>

    <!-- Auth -->
    <div v-show="vhostTab === 'auth'" class="space-y-4 text-sm">
      <label class="vhost-field">
        {{ t('ocserv.vhostAuth') }}
        <select :value="v.auth_method" class="input" @change="patch({ auth_method: $event.target.value })">
          <option value="">{{ t('ocserv.vhostAuthEmpty') }}</option>
          <option value="plain">{{ t('ocserv.authPlain') }}</option>
          <option value="radius">{{ t('ocserv.authRadius') }}</option>
          <option value="certificate">{{ t('ocserv.authCert') }}</option>
        </select>
        <span class="vhost-hint block mt-1">
          {{ t('ocserv.vhostAuthEffective', { method: effectiveAuth }) }}
        </span>
      </label>

      <div v-if="isPlain" class="vhost-panel">
        <h5 class="font-medium text-slate-800">{{ t('ocserv.authPlain') }}</h5>
        <label class="vhost-field">
          {{ t('ocserv.vhostPlainPasswd') }}
          <input
            :value="v.plain_passwd_path"
            class="input font-mono text-xs"
            placeholder="/etc/ocserv/ocpasswd"
            @input="patch({ plain_passwd_path: $event.target.value })"
          />
          <span class="vhost-hint">{{ t('ocserv.vhostPlainPasswdHint') }}</span>
        </label>
      </div>

      <div v-if="isRadius" class="vhost-panel space-y-4">
        <div class="flex flex-wrap items-center gap-2">
          <h5 class="font-medium text-slate-800">{{ t('ocserv.authRadius') }}</h5>
          <OCServRadiusHelpModal button-class="text-blue-700 text-xs underline" />
        </div>
        <label
          v-if="globalIsRadius"
          class="flex gap-2 items-start rounded-md border border-slate-200 bg-white px-3 py-2 cursor-pointer"
        >
          <input v-model="radiusOwn" type="checkbox" class="mt-1" />
          <span>{{ t('ocserv.vhostRadiusOwn') }}</span>
        </label>
        <p v-else class="vhost-hint text-amber-800 bg-amber-50 border border-amber-200 rounded-md px-3 py-2">
          {{ t('ocserv.vhostRadiusDedicatedRequired') }}
        </p>

        <div v-if="radiusOwn" class="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <label class="vhost-field sm:col-span-2">
            {{ t('ocserv.radiusServer') }}
            <input
              :value="v.radius?.server"
              class="input"
              placeholder="103.6.4.138"
              @input="patchRadius({ server: $event.target.value })"
            />
          </label>
          <label class="vhost-field">
            {{ t('ocserv.radiusAuthPort') }}
            <input
              :value="v.radius?.auth_port"
              type="number"
              class="input"
              @input="patchRadius({ auth_port: Number($event.target.value) })"
            />
          </label>
          <label class="vhost-field">
            {{ t('ocserv.radiusAcctPort') }}
            <input
              :value="v.radius?.acct_port"
              type="number"
              class="input"
              @input="patchRadius({ acct_port: Number($event.target.value) })"
            />
          </label>
          <label class="vhost-field sm:col-span-2">
            {{ t('ocserv.radiusSecret') }}
            <input
              :value="radiusSecret"
              type="password"
              class="input"
              autocomplete="new-password"
              :placeholder="radiusSecretSet ? t('ocserv.radiusSecretPh') : t('ocserv.radiusSecretRequired')"
              @input="emit('update:radiusSecret', $event.target.value)"
            />
            <span v-if="radiusSecretSet && !radiusSecret" class="text-xs text-green-700 block mt-1">{{ t('ocserv.radiusSecretSaved') }}</span>
          </label>
          <label class="vhost-field sm:col-span-2">
            {{ t('ocserv.radiusNas') }}
            <input
              :value="v.radius?.nas_identifier"
              class="input"
              :placeholder="t('ocserv.radiusNasPh')"
              @input="patchRadius({ nas_identifier: $event.target.value })"
            />
          </label>
          <label class="flex gap-2 items-center sm:col-span-2">
            <input
              :checked="!!v.radius?.groupconfig"
              type="checkbox"
              @change="patchRadius({ groupconfig: $event.target.checked })"
            />
            <span>{{ t('ocserv.vhostFields.groupconfig') }}</span>
          </label>
          <label class="flex gap-2 items-center sm:col-span-2">
            <input
              :checked="!!v.radius?.acct_enabled"
              type="checkbox"
              @change="patchRadius({ acct_enabled: $event.target.checked })"
            />
            <span>{{ t('ocserv.vhostAcct') }}</span>
          </label>
          <label v-if="v.radius?.acct_enabled" class="vhost-field">
            {{ t('ocserv.vhostFields.statsReportTime') }}
            <input
              :value="v.radius?.stats_report_time"
              type="number"
              class="input max-w-xs"
              @input="patchRadius({ stats_report_time: Number($event.target.value) })"
            />
          </label>
          <label class="vhost-field sm:col-span-2">
            {{ t('ocserv.vhostFields.radcliConfigPath') }}
            <input
              :value="v.radius?.config_path"
              class="input font-mono text-xs"
              placeholder="/etc/radcli/vhosts/..."
              @input="patchRadius({ config_path: $event.target.value })"
            />
          </label>
        </div>
        <div v-else-if="globalIsRadius" class="rounded-md border border-blue-200 bg-blue-50/80 px-3 py-2">
          <p class="vhost-hint">{{ t('ocserv.vhostRadiusInheritGlobal') }}</p>
          <p class="font-mono text-sm mt-1 text-slate-800">
            {{ globalCfg?.radius?.server }}:{{ globalCfg?.radius?.auth_port || 1812 }}
          </p>
          <button type="button" class="text-blue-700 text-sm mt-2 underline" @click="goOcservTab('config')">
            {{ t('ocserv.vhostOpenServerRadius') }}
          </button>
        </div>
      </div>

      <div v-if="isCert" class="vhost-panel vhost-hint">
        {{ t('ocserv.vhostCertHint') }}
      </div>
    </div>

    <!-- Users -->
    <div v-show="vhostTab === 'users'" class="space-y-4 text-sm">
      <div v-if="isPlain" class="vhost-panel space-y-3">
        <h5 class="font-medium text-slate-800">{{ t('ocserv.vhostUsersPlainTitle') }}</h5>
        <p class="vhost-hint">{{ t('ocserv.vhostUsersPlainDesc') }}</p>
        <VhostPlainUsers
          v-if="vhostDomain"
          :domain="vhostDomain"
          :passwd-path="v.plain_passwd_path"
          :group-options="groupOptions"
          @changed="emit('users-changed')"
          @go-auth="vhostTab = 'auth'"
        />
      </div>
      <div v-else-if="isRadius" class="vhost-panel space-y-3">
        <h5 class="font-medium text-slate-800">{{ t('ocserv.vhostUsersRadiusTitle') }}</h5>
        <p class="vhost-hint">{{ t('ocserv.vhostUsersRadiusDesc') }}</p>
        <ul class="list-disc list-inside vhost-hint space-y-1 text-slate-700">
          <li>{{ t('ocserv.vhostUsersRadiusLi1') }}</li>
          <li>{{ t('ocserv.vhostUsersRadiusLi2') }}</li>
          <li>{{ t('ocserv.vhostUsersRadiusLi3') }}</li>
        </ul>
        <button
          v-if="radiusOwn"
          type="button"
          class="btn-secondary text-sm"
          @click="vhostTab = 'auth'"
        >
          {{ t('ocserv.vhostEditRadiusServer') }}
        </button>
        <button
          v-else-if="globalIsRadius"
          type="button"
          class="btn-secondary text-sm"
          @click="goOcservTab('config')"
        >
          {{ t('ocserv.vhostOpenServerRadius') }}
        </button>
      </div>
      <div v-else class="vhost-panel vhost-hint">
        {{ t('ocserv.vhostUsersOtherAuth', { method: effectiveAuth }) }}
      </div>
    </div>

    <!-- TLS -->
    <div v-show="vhostTab === 'tls'" class="space-y-3 text-sm">
      <CertSelect
        :model-value="v.managed_cert_id || ''"
        allow-inherit
        @update:model-value="onVhostCertSelect"
      />
      <p class="text-xs text-slate-500">{{ t('certificates.managedCertHint') }}</p>
      <div v-if="!v.managed_cert_id" class="grid grid-cols-1 sm:grid-cols-2 gap-2">
      <label class="sm:col-span-2">
        {{ t('ocserv.serverCert') }}
        <input :value="v.server_cert_path" class="input w-full mt-1 font-mono text-xs" @input="patch({ server_cert_path: $event.target.value })" />
      </label>
      <label class="sm:col-span-2">
        {{ t('ocserv.serverKey') }}
        <input :value="v.server_key_path" class="input w-full mt-1 font-mono text-xs" @input="patch({ server_key_path: $event.target.value })" />
      </label>
      <label class="sm:col-span-2">
        {{ t('ocserv.caCert') }}
        <input :value="v.ca_cert_path" class="input w-full mt-1 font-mono text-xs" @input="patch({ ca_cert_path: $event.target.value })" />
      </label>
      <label class="sm:col-span-2">
        {{ t('ocserv.vhostFields.crl') }}
        <input :value="v.crl_path" class="input w-full mt-1 font-mono text-xs" @input="patch({ crl_path: $event.target.value })" />
      </label>
      <label class="sm:col-span-2">
        {{ t('ocserv.vhostFields.dhParams') }}
        <input :value="v.dh_params_path" class="input w-full mt-1 font-mono text-xs" @input="patch({ dh_params_path: $event.target.value })" />
      </label>
      <label class="sm:col-span-2">
        {{ t('ocserv.vhostFields.tlsPriorities') }}
        <input :value="v.tls_priorities" class="input w-full mt-1 font-mono text-xs" @input="patch({ tls_priorities: $event.target.value })" />
      </label>
      <label>
        {{ t('ocserv.vhostFields.certUserOid') }}
        <input :value="v.cert_user_oid" class="input w-full mt-1 font-mono text-xs" @input="patch({ cert_user_oid: $event.target.value })" />
      </label>
      <label>
        {{ t('ocserv.vhostFields.certGroupOid') }}
        <input :value="v.cert_group_oid" class="input w-full mt-1 font-mono text-xs" @input="patch({ cert_group_oid: $event.target.value })" />
      </label>
      </div>
    </div>

    <!-- Network -->
    <div v-show="vhostTab === 'network'" class="grid grid-cols-1 sm:grid-cols-2 gap-2 text-sm">
      <label>
        {{ t('ocserv.ipv4Net') }}
        <input :value="v.ipv4_network" class="input w-full mt-1 font-mono" @input="patch({ ipv4_network: $event.target.value })" />
      </label>
      <label>
        {{ t('ocserv.ipv4Mask') }}
        <input :value="v.ipv4_netmask" class="input w-full mt-1 font-mono" @input="patch({ ipv4_netmask: $event.target.value })" />
      </label>
      <label>
        {{ t('ocserv.vhostFields.ipv6Network') }}
        <input :value="v.ipv6_network" class="input w-full mt-1 font-mono" @input="patch({ ipv6_network: $event.target.value })" />
      </label>
      <label>
        {{ t('ocserv.vhostFields.ipv6Prefix') }}
        <input :value="v.ipv6_prefix" type="number" class="input w-full mt-1" @input="patch({ ipv6_prefix: Number($event.target.value) })" />
      </label>
      <label class="sm:col-span-2">
        {{ t('ocserv.vhostFields.defaultDomain') }}
        <input :value="v.default_domain" class="input w-full mt-1" @input="patch({ default_domain: $event.target.value })" />
      </label>
      <label>
        {{ t('ocserv.vhostFields.mtu') }}
        <input :value="v.mtu" type="number" class="input w-full mt-1" @input="patch({ mtu: Number($event.target.value) })" />
      </label>
      <label class="flex items-end gap-2 pb-1">
        <input :checked="!!v.tunnel_all_dns" type="checkbox" @change="patch({ tunnel_all_dns: $event.target.checked })" />
        {{ t('ocserv.vhostFields.tunnelAllDns') }}
      </label>
      <label class="sm:col-span-2">
        {{ t('ocserv.dnsLines') }}
        <textarea v-model="dnsText" class="input w-full mt-1 font-mono text-xs" rows="2" />
      </label>
      <label class="sm:col-span-2">
        {{ t('ocserv.vhostFields.nbns') }}
        <textarea v-model="nbnsText" class="input w-full mt-1 font-mono text-xs" rows="2" />
      </label>
      <label>
        {{ t('ocserv.downCapM') }}
        <input v-model.number="vhostDownMbps" type="number" step="0.01" class="input w-full mt-1" />
        <span class="vhost-hint">tx-data-per-sec</span>
      </label>
      <label>
        {{ t('ocserv.upCapM') }}
        <input v-model.number="vhostUpMbps" type="number" step="0.01" class="input w-full mt-1" />
        <span class="vhost-hint">rx-data-per-sec</span>
      </label>
      <label>
        {{ t('ocserv.vhostFields.pktMtuSize') }}
        <input :value="v.pkt_mtu_size" type="number" class="input w-full mt-1" @input="patch({ pkt_mtu_size: Number($event.target.value) })" />
      </label>
    </div>

    <!-- Routes -->
    <div v-show="vhostTab === 'routes'" class="grid grid-cols-1 gap-2 text-sm">
      <label>
        {{ t('ocserv.vhostFields.route') }}
        <textarea v-model="routesText" class="input w-full mt-1 font-mono text-xs" rows="3" />
      </label>
      <label>
        {{ t('ocserv.vhostFields.noRoute') }}
        <textarea v-model="noRoutesText" class="input w-full mt-1 font-mono text-xs" rows="2" />
      </label>
      <label>
        {{ t('ocserv.vhostFields.iroute') }}
        <textarea v-model="iroutesText" class="input w-full mt-1 font-mono text-xs" rows="2" />
      </label>
      <label class="flex gap-2">
        <input :checked="!!v.expose_iroutes" type="checkbox" @change="patch({ expose_iroutes: $event.target.checked })" />
        {{ t('ocserv.vhostFields.exposeIroutes') }}
      </label>
    </div>

    <!-- Session -->
    <div v-show="vhostTab === 'session'" class="grid grid-cols-2 sm:grid-cols-3 gap-2 text-sm">
      <label>{{ t('ocserv.vhostFields.idleTimeout') }} <input :value="v.idle_timeout" type="number" class="input w-full mt-1" @input="patch({ idle_timeout: Number($event.target.value) })" /></label>
      <label>{{ t('ocserv.vhostFields.sessionTimeout') }} <input :value="v.session_timeout" type="number" class="input w-full mt-1" @input="patch({ session_timeout: Number($event.target.value) })" /></label>
      <label>{{ t('ocserv.vhostFields.mobileIdleTimeout') }} <input :value="v.mobile_idle_timeout" type="number" class="input w-full mt-1" @input="patch({ mobile_idle_timeout: Number($event.target.value) })" /></label>
      <label>{{ t('ocserv.vhostFields.maxSameClients') }} <input :value="v.max_same_clients" type="number" class="input w-full mt-1" @input="patch({ max_same_clients: Number($event.target.value) })" /></label>
      <label>{{ t('ocserv.vhostFields.keepalive') }} <input :value="v.keepalive" type="number" class="input w-full mt-1" @input="patch({ keepalive: Number($event.target.value) })" /></label>
      <label>{{ t('ocserv.vhostFields.dpd') }} <input :value="v.dpd" type="number" class="input w-full mt-1" @input="patch({ dpd: Number($event.target.value) })" /></label>
      <label>{{ t('ocserv.vhostFields.mobileDpd') }} <input :value="v.mobile_dpd" type="number" class="input w-full mt-1" @input="patch({ mobile_dpd: Number($event.target.value) })" /></label>
      <label>{{ t('ocserv.vhostFields.cookieTimeout') }} <input :value="v.cookie_timeout" type="number" class="input w-full mt-1" @input="patch({ cookie_timeout: Number($event.target.value) })" /></label>
      <label>{{ t('ocserv.vhostFields.rekeyTime') }} <input :value="v.rekey_time" type="number" class="input w-full mt-1" @input="patch({ rekey_time: Number($event.target.value) })" /></label>
      <label>
        {{ t('ocserv.vhostFields.rekeyMethod') }}
        <select :value="v.rekey_method" class="input w-full mt-1" @change="patch({ rekey_method: $event.target.value })">
          <option value=""></option>
          <option value="ssl">ssl</option>
          <option value="new-tunnel">new-tunnel</option>
        </select>
      </label>
      <label class="flex items-end gap-2"><input :checked="!!v.deny_roaming" type="checkbox" @change="patch({ deny_roaming: $event.target.checked })" /> {{ t('ocserv.vhostFields.denyRoaming') }}</label>
      <label class="flex items-end gap-2"><input :checked="!!v.persistent_cookies" type="checkbox" @change="patch({ persistent_cookies: $event.target.checked })" /> {{ t('ocserv.vhostFields.persistentCookies') }}</label>
      <label class="flex items-end gap-2"><input :checked="!!v.acct_enabled" type="checkbox" @change="patch({ acct_enabled: $event.target.checked })" /> {{ t('ocserv.vhostAcctInherit') }}</label>
      <label v-if="v.acct_enabled">{{ t('ocserv.vhostFields.statsReportTime') }} <input :value="v.stats_report_time" type="number" class="input w-full mt-1" @input="patch({ stats_report_time: Number($event.target.value) })" /></label>
    </div>

    <!-- Protocol -->
    <div v-show="vhostTab === 'protocol'" class="grid grid-cols-2 sm:grid-cols-3 gap-2 text-sm">
      <label class="flex gap-2 p-2 border rounded"><input :checked="!!v.compression" type="checkbox" @change="patch({ compression: $event.target.checked })" /> {{ t('ocserv.vhostFields.compression') }}</label>
      <label class="flex gap-2 p-2 border rounded"><input :checked="!!v.predictable_ips" type="checkbox" @change="patch({ predictable_ips: $event.target.checked })" /> {{ t('ocserv.vhostFields.predictableIps') }}</label>
      <label class="flex gap-2 p-2 border rounded"><input :checked="!!v.dtls_legacy" type="checkbox" @change="patch({ dtls_legacy: $event.target.checked })" /> {{ t('ocserv.vhostFields.dtlsLegacy') }}</label>
      <label class="flex gap-2 p-2 border rounded"><input :checked="!!v.cisco_client_compat" type="checkbox" @change="patch({ cisco_client_compat: $event.target.checked })" /> {{ t('ocserv.vhostFields.ciscoClientCompat') }}</label>
      <label
        class="flex gap-2 p-2 border rounded sm:col-span-2"
        :class="v.cisco_svc_client_compat ? 'border-amber-300 bg-amber-50/40' : ''"
      >
        <input
          :checked="!!v.cisco_svc_client_compat"
          type="checkbox"
          class="mt-0.5"
          @change="patch({ cisco_svc_client_compat: $event.target.checked })"
        />
        <span>
          <span class="font-medium">{{ t('ocserv.feat.cisco_svc_client_compat.label') }}</span>
          <span class="block text-xs text-amber-700">{{ t('ocserv.udpPortCiscoSvcHint') }}</span>
        </span>
      </label>
      <label class="flex gap-2 p-2 border rounded"><input :checked="!!v.no_udp" type="checkbox" @change="patch({ no_udp: $event.target.checked })" /> {{ t('ocserv.vhostFields.noUdp') }}</label>
    </div>

    <!-- Groups -->
    <div v-show="vhostTab === 'groups'" class="grid grid-cols-1 sm:grid-cols-2 gap-2 text-sm">
      <label class="sm:col-span-2">{{ t('ocserv.vhostFields.configPerUser') }} <input :value="v.config_per_user" class="input w-full mt-1 font-mono text-xs" @input="patch({ config_per_user: $event.target.value })" /></label>
      <label class="sm:col-span-2">{{ t('ocserv.vhostFields.configPerGroup') }} <input :value="v.config_per_group" class="input w-full mt-1 font-mono text-xs" @input="patch({ config_per_group: $event.target.value })" /></label>
      <label class="sm:col-span-2">{{ t('ocserv.vhostFields.defaultUserConfig') }} <input :value="v.default_user_config" class="input w-full mt-1 font-mono text-xs" @input="patch({ default_user_config: $event.target.value })" /></label>
      <label class="sm:col-span-2">{{ t('ocserv.vhostFields.defaultGroupConfig') }} <input :value="v.default_group_config" class="input w-full mt-1 font-mono text-xs" @input="patch({ default_group_config: $event.target.value })" /></label>
      <label class="sm:col-span-2">
        {{ t('ocserv.vhostFields.selectGroup') }}
        <textarea v-model="selectGroupsText" class="input w-full mt-1 font-mono text-xs" rows="2" />
      </label>
      <label class="flex gap-2 items-center"><input :checked="!!v.auto_select_group" type="checkbox" @change="patch({ auto_select_group: $event.target.checked })" /> {{ t('ocserv.vhostFields.autoSelectGroup') }}</label>
      <label>{{ t('ocserv.vhostFields.defaultSelectGroup') }} <input :value="v.default_select_group" class="input w-full mt-1" @input="patch({ default_select_group: $event.target.value })" /></label>
    </div>

    <!-- Extra -->
    <div v-show="vhostTab === 'extra'" class="grid grid-cols-1 gap-2 text-sm">
      <label>{{ t('ocserv.vhostFields.banner') }} <input :value="v.banner" class="input w-full mt-1" @input="patch({ banner: $event.target.value })" /></label>
      <label>{{ t('ocserv.vhostFields.preLoginBanner') }} <input :value="v.pre_login_banner" class="input w-full mt-1" @input="patch({ pre_login_banner: $event.target.value })" /></label>
      <label class="flex gap-2"><input :checked="!!v.camouflage" type="checkbox" @change="patch({ camouflage: $event.target.checked })" /> {{ t('ocserv.vhostFields.camouflage') }}</label>
      <label v-if="v.camouflage">
        {{ t('ocserv.vhostFields.camouflageSecret') }}
        <input
          :value="camouflageSecret"
          type="password"
          class="input w-full mt-1"
          :placeholder="camouflageSecretSet ? t('ocserv.radiusSecretPh') : ''"
          @input="emit('update:camouflageSecret', $event.target.value)"
        />
        <span v-if="camouflageSecretSet && !camouflageSecret" class="text-xs text-green-700">{{ t('ocserv.radiusSecretSaved') }}</span>
      </label>
      <label v-if="v.camouflage">{{ t('ocserv.vhostFields.camouflageRealm') }} <input :value="v.camouflage_realm" class="input w-full mt-1" @input="patch({ camouflage_realm: $event.target.value })" /></label>
      <label>{{ t('ocserv.vhostFields.connectScript') }} <input :value="v.connect_script" class="input w-full mt-1 font-mono text-xs" @input="patch({ connect_script: $event.target.value })" /></label>
      <label>{{ t('ocserv.vhostFields.disconnectScript') }} <input :value="v.disconnect_script" class="input w-full mt-1 font-mono text-xs" @input="patch({ disconnect_script: $event.target.value })" /></label>
    </div>
  </div>
</template>
