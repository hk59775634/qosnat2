<script setup>
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'

const { t } = useI18n()
const tunnels = ref([])
const err = ref('')
const ok = ref('')
const editing = ref(null)
const form = ref({
  vni: 100,
  local: '',
  remote: '',
  port: 4789,
  underlay: '',
  ipv4: '',
  up: true,
})

async function load() {
  const d = await api.network.vxlan.list()
  tunnels.value = d.vxlan_tunnels || []
}

function reset() {
  editing.value = null
  form.value = { vni: 100, local: '', remote: '', port: 4789, underlay: '', ipv4: '', up: true }
}

function startEdit(tun) {
  editing.value = tun.id
  form.value = {
    vni: tun.vni,
    local: tun.local,
    remote: tun.remote,
    port: tun.port || 4789,
    underlay: tun.underlay || '',
    ipv4: (tun.ipv4 || []).join('\n'),
    up: tun.up !== false,
  }
}

async function submit() {
  err.value = ''
  const ipv4 = form.value.ipv4.split(/[\n,]+/).map((s) => s.trim()).filter(Boolean)
  const body = {
    vni: form.value.vni,
    local: form.value.local,
    remote: form.value.remote,
    port: form.value.port,
    underlay: form.value.underlay,
    ipv4,
    up: form.value.up,
  }
  try {
    if (editing.value) {
      await api.network.vxlan.put(editing.value, body)
      ok.value = t('common.saved')
    } else {
      await api.network.vxlan.add(body)
      ok.value = t('common.saved')
    }
    reset()
    await load()
  } catch (e) {
    err.value = e.message
  }
}

async function remove(id) {
  if (!confirm(t('common.delete') + '?')) return
  await api.network.vxlan.del(id)
  await load()
}

onMounted(load)
</script>

<template>
  <div class="page-stack">
    <PageHeader :title="t('network.vxlan.title')" :description="t('network.vxlan.description')" />
    <p v-if="ok" class="text-green-700 text-sm">{{ ok }}</p>
    <p v-if="err" class="text-red-600 text-sm">{{ err }}</p>

    <div class="card card-body grid sm:grid-cols-2 gap-3 text-sm">
      <div>
        <label class="text-xs text-slate-500">VNI</label>
        <input v-model.number="form.vni" type="number" class="input-field mt-1" />
      </div>
      <div>
        <label class="text-xs text-slate-500">{{ t('network.vxlan.udpPort') }}</label>
        <input v-model.number="form.port" type="number" class="input-field mt-1" />
      </div>
      <div>
        <label class="text-xs text-slate-500">{{ t('network.vxlan.localVtep') }}</label>
        <input v-model="form.local" class="input-field mt-1 font-mono" />
      </div>
      <div>
        <label class="text-xs text-slate-500">{{ t('network.vxlan.remoteVtep') }}</label>
        <input v-model="form.remote" class="input-field mt-1 font-mono" />
      </div>
      <div>
        <label class="text-xs text-slate-500">{{ t('network.vxlan.underlay') }}</label>
        <input v-model="form.underlay" class="input-field mt-1 font-mono" />
      </div>
      <div class="sm:col-span-2">
        <label class="text-xs text-slate-500">{{ t('network.vxlan.overlayIpv4') }}</label>
        <textarea v-model="form.ipv4" class="input-field mt-1 font-mono h-14" />
      </div>
      <label class="flex items-center gap-2">
        <input v-model="form.up" type="checkbox" /> {{ t('common.up') }}
      </label>
      <div class="sm:col-span-2 flex gap-2">
        <button type="button" class="btn-primary" @click="submit">{{ editing ? t('common.save') : t('common.create') }}</button>
        <button v-if="editing" type="button" class="btn-secondary" @click="reset">{{ t('common.cancel') }}</button>
      </div>
    </div>

    <div class="card overflow-x-auto">
      <table class="data w-full text-sm">
        <thead>
          <tr><th>{{ t('common.name') }}</th><th>VNI</th><th>local</th><th>remote</th><th>IPv4</th><th></th></tr>
        </thead>
        <tbody>
          <tr v-for="tun in tunnels" :key="tun.id">
            <td class="font-mono">{{ tun.name }}</td>
            <td>{{ tun.vni }}</td>
            <td class="font-mono text-xs">{{ tun.local }}</td>
            <td class="font-mono text-xs">{{ tun.remote }}</td>
            <td class="font-mono text-xs">{{ (tun.ipv4 || []).join(', ') || '—' }}</td>
            <td class="text-right space-x-2">
              <button type="button" class="text-xs text-blue-600" @click="startEdit(tun)">{{ t('common.edit') }}</button>
              <button type="button" class="text-red-600 text-xs" @click="remove(tun.id)">{{ t('common.delete') }}</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
