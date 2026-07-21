<script setup>
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'

const { t } = useI18n()
const aliases = ref([])
const form = ref({
  name: '',
  type: 'ipv4_addr',
  membersText: '',
  domainsText: '',
  countriesText: '',
  asn: '',
  url: '',
  comment: '',
})
const err = ref('')
const ok = ref('')

const isFQDN = computed(() => form.value.type === 'fqdn')
const isGeoIP = computed(() => form.value.type === 'geoip')
const isPort = computed(() => form.value.type === 'port')
const isASN = computed(() => form.value.type === 'asn')

async function load() {
  const d = await api.firewall.aliases.list()
  aliases.value = d.aliases || []
}

function validateForm() {
  if (!String(form.value.name || '').trim()) {
    err.value = t('security.aliases.errName')
    return false
  }
  const hasURL = !!String(form.value.url || '').trim()
  if (isFQDN.value) {
    const domains = form.value.domainsText.split(/[\n,]+/).map((s) => s.trim()).filter(Boolean)
    if (!domains.length && !hasURL) {
      err.value = t('security.aliases.errDomains')
      return false
    }
    return true
  }
  if (isGeoIP.value) {
    const ccs = form.value.countriesText.split(/[\n,\s]+/).map((s) => s.trim()).filter(Boolean)
    if (!ccs.length) {
      err.value = t('security.aliases.errCountries')
      return false
    }
    return true
  }
  const members = form.value.membersText.split(/[\n,]+/).map((s) => s.trim()).filter(Boolean)
  if (!members.length && !hasURL && !isGeoIP.value) {
    err.value = t('security.aliases.errMembers')
    return false
  }
  return true
}

async function add() {
  err.value = ''
  ok.value = ''
  if (!validateForm()) return
  try {
    const body = {
      name: form.value.name.trim(),
      type: form.value.type,
      comment: form.value.comment,
      members: [],
    }
    const url = String(form.value.url || '').trim()
    if (url && !isPort.value && !isGeoIP.value) body.url = url
    if (isFQDN.value) {
      body.domains = form.value.domainsText.split(/[\n,]+/).map((s) => s.trim()).filter(Boolean)
    } else if (isGeoIP.value) {
      body.countries = form.value.countriesText.split(/[\n,\s]+/).map((s) => s.trim()).filter(Boolean)
    } else if (isASN.value) {
      const asn = parseInt(form.value.asn, 10)
      if (Number.isFinite(asn)) body.asn = asn
      body.members = form.value.membersText.split(/[\n,]+/).map((s) => s.trim()).filter(Boolean)
    } else {
      body.members = form.value.membersText.split(/[\n,]+/).map((s) => s.trim()).filter(Boolean)
    }
    await api.firewall.aliases.add(body)
    if (isGeoIP.value) {
      try {
        await api.firewall.aliases.refresh(body.name)
      } catch {
        /* refresh may be slow / fail offline */
      }
    }
    form.value = {
      name: '',
      type: 'ipv4_addr',
      membersText: '',
      domainsText: '',
      countriesText: '',
      asn: '',
      url: '',
      comment: '',
    }
    ok.value = t('common.saved')
    await load()
  } catch (e) {
    err.value = e?.data?.error || e?.message || String(e)
  }
}

async function refreshAlias(name) {
  err.value = ''
  ok.value = ''
  try {
    await api.firewall.aliases.refresh(name)
    ok.value = t('security.aliases.refreshed')
    await load()
  } catch (e) {
    err.value = e?.data?.error || e?.message || String(e)
  }
}

function canRefresh(a) {
  if ((a.type || '') === 'geoip' && (a.countries || []).length) return true
  if ((a.type || '') === 'fqdn' && ((a.domains || []).length || a.url)) return true
  return !!a.url
}

function typeLabel(a) {
  switch (a.type) {
    case 'fqdn':
      return t('security.aliases.typeFqdn')
    case 'asn':
      return t('security.aliases.typeAsn')
    case 'geoip':
      return t('security.aliases.typeGeoip')
    case 'port':
      return t('security.aliases.typePort')
    default:
      return t('security.aliases.typeIpv4')
  }
}

async function remove(name) {
  if (!confirm(t('security.aliases.confirmDelete', { name }))) return
  err.value = ''
  try {
    await api.firewall.aliases.del(name)
    await load()
  } catch (e) {
    err.value = e?.status === 409 ? t('security.aliases.errInUse') : e?.data?.error || e?.message || String(e)
  }
}

onMounted(load)
</script>

<template>
  <div class="page-stack">
    <PageHeader :title="t('security.aliases.title')" :description="t('security.aliases.description')" :err="err" :ok="ok" />
    <div class="card card-body mb-0 space-y-3 text-sm">
      <input v-model="form.name" class="input-field font-mono" :placeholder="t('security.aliases.namePh')" />
      <select v-model="form.type" class="input-field">
        <option value="ipv4_addr">{{ t('security.aliases.typeIpv4') }}</option>
        <option value="fqdn">{{ t('security.aliases.typeFqdn') }}</option>
        <option value="asn">{{ t('security.aliases.typeAsn') }}</option>
        <option value="geoip">{{ t('security.aliases.typeGeoip') }}</option>
        <option value="port">{{ t('security.aliases.typePort') }}</option>
      </select>
      <template v-if="isFQDN">
        <textarea
          v-model="form.domainsText"
          class="input-field font-mono text-xs h-24"
          :placeholder="t('security.aliases.domainsPh')"
        />
        <input v-model="form.url" class="input-field font-mono text-xs" :placeholder="t('security.aliases.fqdnUrlPh')" />
        <p class="text-xs text-slate-500">{{ t('security.aliases.fqdnHint') }}</p>
      </template>
      <template v-else-if="isGeoIP">
        <textarea
          v-model="form.countriesText"
          class="input-field font-mono text-xs h-20"
          :placeholder="t('security.aliases.countriesPh')"
        />
        <p class="text-xs text-slate-500">{{ t('security.aliases.geoipHint') }}</p>
      </template>
      <template v-else-if="isPort">
        <textarea
          v-model="form.membersText"
          class="input-field font-mono text-xs h-24"
          :placeholder="t('security.aliases.portHint')"
        />
      </template>
      <template v-else>
        <input
          v-if="isASN"
          v-model="form.asn"
          type="number"
          class="input-field font-mono text-xs"
          :placeholder="t('security.aliases.asnPh')"
        />
        <p v-if="isASN" class="text-xs text-slate-500">{{ t('security.aliases.asnHint') }}</p>
        <textarea
          v-model="form.membersText"
          class="input-field font-mono text-xs h-24"
          :placeholder="t('security.aliases.membersPh')"
        />
        <input v-model="form.url" class="input-field font-mono text-xs" :placeholder="t('security.aliases.urlPh')" />
        <p class="text-xs text-slate-500">{{ t('security.aliases.urlHint') }}</p>
      </template>
      <input v-model="form.comment" class="input-field" :placeholder="t('security.aliases.remarkPh')" />
      <button type="button" class="btn-primary" @click="add">{{ t('security.aliases.addApply') }}</button>
    </div>
    <div class="card overflow-x-auto">
      <table class="data w-full text-sm">
        <thead>
          <tr>
            <th>{{ t('common.name') }}</th>
            <th>{{ t('security.aliases.colType') }}</th>
            <th>{{ t('security.aliases.colMembers') }}</th>
            <th>{{ t('security.aliases.colSource') }}</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="a in aliases" :key="a.name">
            <td class="font-mono">{{ a.name }}</td>
            <td>{{ typeLabel(a) }}</td>
            <td class="text-xs font-mono">
              {{ t('security.aliases.memberCount', { n: (a.members || []).length }) }}
              <span v-if="(a.members || []).length <= 8" class="text-slate-500"> · {{ (a.members || []).join(', ') }}</span>
            </td>
            <td class="text-xs font-mono max-w-xs truncate">
              <template v-if="a.type === 'fqdn'">
                <span class="text-slate-600">{{ t('security.aliases.domainCount', { n: (a.domains || []).length }) }}</span>
                <p v-if="(a.domains || []).length <= 4" class="text-slate-500">{{ (a.domains || []).join(', ') }}</p>
                <template v-if="a.url">
                  <a :href="a.url" target="_blank" rel="noopener" class="text-blue-600 hover:underline block">{{ a.url }}</a>
                  <p v-if="a.url_fetched_at" class="text-slate-400">{{ a.url_fetched_at }}</p>
                </template>
                <p v-if="a.resolved_at" class="text-slate-400">{{ a.resolved_at }}</p>
              </template>
              <template v-else-if="a.type === 'geoip'">
                <span>{{ (a.countries || []).join(', ') }}</span>
                <p v-if="a.url_fetched_at" class="text-slate-400">{{ a.url_fetched_at }}</p>
              </template>
              <template v-else-if="a.url">
                <a :href="a.url" target="_blank" rel="noopener" class="text-blue-600 hover:underline">{{ a.url }}</a>
                <p v-if="a.url_fetched_at" class="text-slate-400">{{ a.url_fetched_at }}</p>
              </template>
              <span v-else class="text-slate-400">—</span>
            </td>
            <td class="whitespace-nowrap space-x-2">
              <button v-if="canRefresh(a)" type="button" class="text-indigo-600 text-xs" @click="refreshAlias(a.name)">
                {{ t('security.aliases.refresh') }}
              </button>
              <button type="button" class="text-red-600 text-xs" @click="remove(a.name)">{{ t('common.delete') }}</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
