<script setup>
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { api } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'

const { t } = useI18n()
const list = ref([])
const ifaces = ref([])
const netplanPath = ref('')
const err = ref('')
const ok = ref('')
const editing = ref(null)
const form = ref({
  interface: '',
  address: '',
  comment: '',
  enabled: true,
})

async function load() {
  err.value = ''
  try {
    const [vip, ifc] = await Promise.all([api.network.virtualIPs.list(), api.interfaces.list()])
    list.value = vip.virtual_ips || []
    netplanPath.value = vip.netplan_path || ''
    ifaces.value = (ifc.interfaces || []).filter((i) => i.netplan_manageable !== false)
    if (!form.value.interface) {
      form.value.interface = vip.dev_wan || ifaces.value[0]?.name || ''
    }
  } catch (e) {
    err.value = e.message
  }
}

function resetForm() {
  editing.value = null
  form.value = {
    interface: form.value.interface,
    address: '',
    comment: '',
    enabled: true,
  }
}

function startEdit(v) {
  editing.value = v.id
  form.value = {
    interface: v.interface,
    address: v.host || v.address,
    comment: v.comment || '',
    enabled: v.enabled !== false,
  }
}

async function submit() {
  err.value = ''
  ok.value = ''
  const payload = {
    type: 'ip_alias',
    interface: form.value.interface,
    address: form.value.address,
    comment: form.value.comment,
    enabled: form.value.enabled,
  }
  try {
    if (editing.value) {
      await api.network.virtualIPs.put(editing.value, payload)
    } else {
      await api.network.virtualIPs.add(payload)
    }
    ok.value = t('common.saved')
    resetForm()
    await load()
  } catch (e) {
    err.value = e.message
  }
}

async function remove(id) {
  if (!confirm(t('common.delete') + '?')) return
  err.value = ''
  try {
    await api.network.virtualIPs.del(id)
    if (editing.value === id) resetForm()
    await load()
  } catch (e) {
    err.value = e.message
  }
}

async function useInShared(host) {
  err.value = ''
  ok.value = ''
  try {
    await api.sharedIPs.add(host)
    ok.value = t('network.virtualIps.addedToShared', { ip: host })
  } catch (e) {
    err.value = e.message
  }
}

onMounted(load)
</script>

<template>
  <div class="page-stack">
    <PageHeader
      :title="t('network.virtualIps.title')"
      :description="t('network.virtualIps.description')"
      :ok="ok"
      :err="err"
    />

    <p class="text-sm text-slate-600 mb-3">
      {{ t('network.virtualIps.hint') }}
      <RouterLink to="/nat/outbound" class="text-blue-600 hover:underline">{{ t('nav.outboundNat') }}</RouterLink>
      ·
      <RouterLink to="/nat/forwards" class="text-blue-600 hover:underline">{{ t('nav.portForwards') }}</RouterLink>
      ·
      <RouterLink to="/network/interfaces" class="text-blue-600 hover:underline">{{ t('nav.interfaces') }}</RouterLink>
    </p>
    <p v-if="netplanPath" class="text-xs text-slate-400 mb-3 font-mono">{{ netplanPath }}</p>

    <div class="card card-body mb-0 space-y-3 text-sm">
      <h3 class="font-medium">{{ editing ? t('network.virtualIps.edit') : t('network.virtualIps.new') }}</h3>
      <div class="grid sm:grid-cols-2 gap-3">
        <div>
          <label class="text-xs text-slate-500">{{ t('network.virtualIps.iface') }}</label>
          <select v-model="form.interface" class="input-field mt-1 font-mono">
            <option v-for="i in ifaces" :key="i.name" :value="i.name">{{ i.name }}</option>
          </select>
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('network.virtualIps.address') }}</label>
          <input
            v-model="form.address"
            class="input-field mt-1 font-mono"
            :placeholder="t('network.virtualIps.addressPlaceholder')"
          />
        </div>
        <div class="sm:col-span-2">
          <label class="text-xs text-slate-500">{{ t('network.virtualIps.comment') }}</label>
          <input v-model="form.comment" class="input-field mt-1" />
        </div>
        <label class="flex items-center gap-2">
          <input v-model="form.enabled" type="checkbox" />
          {{ t('network.virtualIps.enabled') }}
        </label>
      </div>
      <div class="flex flex-wrap gap-2">
        <button type="button" class="btn-primary" @click="submit">
          {{ editing ? t('common.save') : t('common.create') }}
        </button>
        <button v-if="editing" type="button" class="btn-secondary" @click="resetForm">{{ t('common.cancel') }}</button>
      </div>
    </div>

    <div class="table-wrap card">
      <table class="data w-full text-sm">
        <thead>
          <tr>
            <th>{{ t('network.virtualIps.address') }}</th>
            <th>{{ t('network.virtualIps.iface') }}</th>
            <th>{{ t('network.virtualIps.status') }}</th>
            <th>{{ t('network.virtualIps.comment') }}</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="v in list" :key="v.id" :class="{ 'bg-blue-50': editing === v.id }">
            <td class="font-mono">{{ v.host || v.address }}</td>
            <td class="font-mono">{{ v.interface }}</td>
            <td>
              <span v-if="!v.enabled" class="text-slate-400">{{ t('network.virtualIps.disabled') }}</span>
              <span v-else-if="v.assigned" class="text-green-700">{{ t('network.virtualIps.assigned') }}</span>
              <span v-else class="text-amber-700">{{ t('network.virtualIps.notAssigned') }}</span>
            </td>
            <td class="text-xs text-slate-600">{{ v.comment || '—' }}</td>
            <td class="text-right whitespace-nowrap space-x-2">
              <button type="button" class="text-xs text-blue-600" @click="startEdit(v)">{{ t('common.edit') }}</button>
              <button
                type="button"
                class="text-xs text-blue-600"
                :disabled="!v.enabled"
                @click="useInShared(v.host || v.address)"
              >
                {{ t('network.virtualIps.addShared') }}
              </button>
              <button type="button" class="text-red-600 text-xs" @click="remove(v.id)">{{ t('common.delete') }}</button>
            </td>
          </tr>
          <tr v-if="!list.length">
            <td colspan="5" class="text-center text-slate-400 py-3">{{ t('network.virtualIps.empty') }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
