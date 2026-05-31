<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { api } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'

const { t } = useI18n()
const list = ref([])
const interfaces = ref([])
const defaults = ref({})
const devLan = ref('')
const devWan = ref('')
const err = ref('')
const ok = ref('')

const form = ref({
  interface: '',
  ip_version: 'ipv4',
  proto: 'tcp',
  src_addr: '0.0.0.0/0',
  dst_addr: '',
  dst_port: 443,
  redirect_ip: '192.168.1.10',
  redirect_port: 443,
  comment: '',
})

const bindIface = computed({
  get: () => form.value.interface || defaults.value.interface || devWan.value,
  set: (v) => {
    form.value.interface = v
  },
})

const ifaceAddrs = computed(() => {
  const iface = interfaces.value.find((i) => i.name === bindIface.value)
  return iface?.addrs || []
})

const srcAddrPlaceholder = computed(() =>
  form.value.ip_version === 'ipv6' ? '::/0' : '0.0.0.0/0',
)

watch(
  () => form.value.ip_version,
  (ver) => {
    if (ver === 'ipv6') {
      if (form.value.src_addr === '0.0.0.0/0' || !form.value.src_addr) {
        form.value.src_addr = '::/0'
      }
    } else if (form.value.src_addr === '::/0') {
      form.value.src_addr = '0.0.0.0/0'
    }
  },
)

watch(bindIface, (name) => {
  const iface = interfaces.value.find((i) => i.name === name)
  if (iface?.addrs?.length && !form.value.dst_addr) {
    form.value.dst_addr = iface.addrs[0].split('/')[0]
  }
})

async function load() {
  const d = await api.wanForwards.list()
  list.value = d.forwards || []
  interfaces.value = d.interfaces || []
  defaults.value = d.defaults || {}
  devLan.value = d.dev_lan || ''
  devWan.value = d.dev_wan || ''
  if (!form.value.interface) {
    form.value.interface = defaults.value.interface || devWan.value
  }
  if (!form.value.dst_addr && defaults.value.dst_addr) {
    form.value.dst_addr = defaults.value.dst_addr
  }
}

async function add() {
  err.value = ''
  ok.value = ''
  try {
    const body = {
      interface: bindIface.value,
      ip_version: form.value.ip_version,
      proto: form.value.proto,
      src_addr: form.value.src_addr || srcAddrPlaceholder.value,
      dst_addr: form.value.dst_addr || undefined,
      dst_port: form.value.dst_port,
      redirect_ip: form.value.redirect_ip,
      redirect_port: form.value.redirect_port || form.value.dst_port,
      comment: form.value.comment,
    }
    await api.wanForwards.add(body)
    ok.value = t('common.saved')
    await load()
  } catch (e) {
    err.value = e.message
  }
}

async function remove(f) {
  if (!confirm(t('nat.forwards.confirmDelete'))) return
  err.value = ''
  try {
    await api.wanForwards.del(f.id)
    await load()
  } catch (e) {
    err.value = e.message
  }
}

function fmtRule(f) {
  const dst = f.dst_addr ? `${f.dst_addr}:` : ':'
  return `${f.interface || devWan.value} ${f.ip_version} ${f.proto} ${f.src_addr} → ${dst}${f.dst_port} ⇒ ${f.redirect_ip}:${f.redirect_port}`
}

function forwardFirewallIds(f) {
  if (!f?.id) return []
  const p = String(f.proto || 'tcp').toLowerCase()
  if (p === 'tcp_udp' || p === 'both') {
    return [`auto-fwd-${f.id}-tcp`, `auto-fwd-${f.id}-udp`]
  }
  if (p === 'udp') return [`auto-fwd-${f.id}-udp`]
  return [`auto-fwd-${f.id}-tcp`]
}

function linkageStep2Text() {
  return t('nat.forwards.linkageStep2', { wan: bindIface.value || devWan.value || 'WAN', lan: devLan.value || 'LAN' })
}

function linkageStep3Text() {
  return t('nat.forwards.linkageStep3', { lan: devLan.value || 'LAN' })
}

onMounted(load)
</script>

<template>
  <div class="page-stack">
    <PageHeader :title="t('nat.forwards.title')" :description="t('nat.forwards.description')" :ok="ok" :err="err" />

    <div class="card card-body border border-blue-100 bg-blue-50/40 text-sm space-y-2 mb-0">
      <h3 class="font-medium text-pfsense-nav">{{ t('nat.forwards.linkageTitle') }}</h3>
      <p class="text-xs text-slate-600 leading-relaxed">{{ t('nat.forwards.linkageIntro') }}</p>
      <ol class="text-xs text-slate-700 list-decimal list-inside space-y-1.5 leading-relaxed">
        <li>{{ t('nat.forwards.linkageStep1') }}</li>
        <li>{{ linkageStep2Text() }}</li>
        <li>{{ linkageStep3Text() }}</li>
      </ol>
      <p class="text-xs text-amber-900 bg-amber-50 border border-amber-100 rounded px-2 py-1.5 leading-relaxed">
        {{ t('nat.forwards.linkageNote') }}
      </p>
      <RouterLink
        :to="{ name: 'firewall-rules', query: { chain: 'forward' } }"
        class="inline-block text-sm text-blue-700 hover:underline font-medium"
      >
        {{ t('nat.forwards.linkageFirewallLink') }} →
      </RouterLink>
      <p class="text-xs text-slate-500">{{ t('nat.forwards.linkageDeepLinkHint') }}</p>
    </div>

    <div class="card card-body mb-0">
      <h3 class="font-medium mb-3">{{ t('nat.forwards.addRule') }}</h3>
      <div class="grid sm:grid-cols-2 lg:grid-cols-3 gap-3 text-sm">
        <div>
          <label class="text-xs text-slate-500">{{ t('nat.forwards.iface') }}</label>
          <select v-model="bindIface" class="input-field font-mono">
            <option v-for="iface in interfaces" :key="iface.name" :value="iface.name">
              {{ iface.name }}
              <template v-if="iface.addrs?.length"> — {{ iface.addrs.join(', ') }}</template>
            </option>
          </select>
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('nat.forwards.ipVersion') }}</label>
          <select v-model="form.ip_version" class="input-field">
            <option value="ipv4">IPv4</option>
            <option value="ipv6">IPv6</option>
          </select>
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('nat.forwards.proto') }}</label>
          <select v-model="form.proto" class="input-field">
            <option value="tcp">TCP</option>
            <option value="udp">UDP</option>
            <option value="tcp_udp">TCP + UDP</option>
          </select>
        </div>
        <div>
          <label class="text-xs text-slate-500">src (CIDR)</label>
          <input v-model="form.src_addr" class="input-field font-mono" :placeholder="srcAddrPlaceholder" />
        </div>
        <div>
          <label class="text-xs text-slate-500">dst</label>
          <select v-model="form.dst_addr" class="input-field font-mono">
            <option value="">{{ t('nat.forwards.anyDaddr') }}</option>
            <option v-for="a in ifaceAddrs" :key="a" :value="a.split('/')[0]">{{ a }}</option>
          </select>
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('nat.forwards.match') }}</label>
          <input v-model.number="form.dst_port" type="number" class="input-field" />
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('nat.forwards.redirect') }} IP</label>
          <input v-model="form.redirect_ip" class="input-field font-mono" />
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('nat.forwards.redirect') }}</label>
          <input v-model.number="form.redirect_port" type="number" class="input-field" />
        </div>
        <div class="sm:col-span-2 lg:col-span-3">
          <label class="text-xs text-slate-500">{{ t('common.comment') }}</label>
          <input v-model="form.comment" class="input-field" />
        </div>
      </div>
      <button type="button" class="btn-primary mt-3" @click="add">{{ t('common.add') }}</button>
    </div>

    <div class="card card-body table-wrap">
      <table class="data w-full text-sm">
        <thead>
          <tr>
            <th>{{ t('nat.forwards.iface') }}</th>
            <th>{{ t('nat.forwards.ipVersion') }}</th>
            <th>{{ t('nat.forwards.proto') }}</th>
            <th>{{ t('nat.forwards.match') }}</th>
            <th>{{ t('nat.forwards.redirect') }}</th>
            <th>{{ t('nat.forwards.colFirewall') }}</th>
            <th>{{ t('common.comment') }}</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="f in list" :key="f.id">
            <td class="font-mono text-xs">{{ f.interface || devWan }}</td>
            <td>{{ f.ip_version }}</td>
            <td>{{ f.proto }}</td>
            <td class="font-mono text-xs max-w-xs truncate" :title="fmtRule(f)">
              {{ f.src_addr }} → {{ f.dst_addr || '*' }}:{{ f.dst_port }}
            </td>
            <td class="font-mono text-xs">{{ f.redirect_ip }}:{{ f.redirect_port }}</td>
            <td class="font-mono text-[10px] text-slate-600 align-top">
              <RouterLink
                v-for="fid in forwardFirewallIds(f)"
                :key="fid"
                :to="{ name: 'firewall-rules', query: { chain: 'forward', rule: fid } }"
                class="block whitespace-nowrap text-blue-700 hover:underline"
                :title="t('nat.forwards.linkageFirewallLink')"
              >
                {{ fid }}
              </RouterLink>
            </td>
            <td>{{ f.comment }}</td>
            <td>
              <button type="button" class="text-red-600 text-xs" @click="remove(f)">{{ t('common.delete') }}</button>
            </td>
          </tr>
          <tr v-if="!list.length">
            <td colspan="8" class="text-center text-slate-400 py-3">{{ t('nat.forwards.noRules') }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
