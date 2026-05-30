<script setup>
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'

const { t } = useI18n()
const tenants = ref([])
const err = ref('')
const ok = ref('')
const editing = ref(null)
const form = ref({ name: '', cidrsText: '100.64.0.0/24', down: '8mbit', up: '8mbit', device: '' })

async function load() {
  const d = await api.shaper.tenants.list()
  tenants.value = d.tenants || []
}

function reset() {
  editing.value = null
  form.value = { name: '', cidrsText: '100.64.0.0/24', down: '8mbit', up: '8mbit', device: '' }
}

function startEdit(tenant) {
  editing.value = tenant.id
  form.value = {
    name: tenant.name,
    cidrsText: (tenant.cidrs || []).join('\n'),
    down: tenant.down,
    up: tenant.up,
    device: tenant.device || '',
  }
}

async function submit() {
  err.value = ''
  const cidrs = form.value.cidrsText.split(/[\n,]+/).map((s) => s.trim()).filter(Boolean)
  const body = {
    name: form.value.name,
    cidrs,
    down: form.value.down,
    up: form.value.up,
    device: form.value.device,
  }
  try {
    if (editing.value) {
      await api.shaper.tenants.put(editing.value, body)
      ok.value = t('common.saved')
    } else {
      await api.shaper.tenants.add(body)
      ok.value = t('common.saved')
    }
    reset()
    await load()
  } catch (e) {
    err.value = e.message
  }
}

async function remove(id) {
  if (!confirm(t('shaper.tenants.confirmDelete', { id }))) return
  await api.shaper.tenants.del(id)
  await load()
}

onMounted(load)
</script>

<template>
  <div class="page-stack">
    <PageHeader :title="t('shaper.tenants.title')" :description="t('shaper.tenants.description')" :ok="ok" :err="err" />

    <div class="card card-body space-y-3 text-sm">
      <h3 class="font-medium">{{ editing ? t('shaper.tenants.edit') : t('shaper.tenants.new') }}</h3>
      <input v-model="form.name" class="input-field" :placeholder="t('common.name')" />
      <textarea v-model="form.cidrsText" class="input-field font-mono h-20" placeholder="CIDR" />
      <div class="grid sm:grid-cols-2 gap-2">
        <input v-model="form.down" class="input-field" :placeholder="t('shaper.profiles.downCap')" />
        <input v-model="form.up" class="input-field" :placeholder="t('shaper.profiles.upCap')" />
      </div>
      <input v-model="form.device" class="input-field font-mono" :placeholder="t('shaper.ifaceSelect')" />
      <div class="flex gap-2">
        <button type="button" class="btn-primary" @click="submit">{{ editing ? t('common.save') : t('common.create') }}</button>
        <button v-if="editing" type="button" class="btn-secondary" @click="reset">{{ t('common.cancel') }}</button>
      </div>
    </div>

    <div class="card overflow-x-auto">
      <table class="data w-full text-sm">
        <thead>
          <tr>
            <th>{{ t('common.name') }}</th>
            <th>CIDR</th>
            <th>{{ t('shaper.profiles.colDown') }}</th>
            <th>{{ t('shaper.profiles.colUp') }}</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="tenant in tenants" :key="tenant.id">
            <td>{{ tenant.name }}</td>
            <td class="font-mono text-xs">{{ (tenant.cidrs || []).join(', ') }}</td>
            <td>{{ tenant.down }}</td>
            <td>{{ tenant.up }}</td>
            <td class="text-right space-x-2">
              <button type="button" class="text-blue-600 text-xs" @click="startEdit(tenant)">{{ t('common.edit') }}</button>
              <button type="button" class="text-red-600 text-xs" @click="remove(tenant.id)">{{ t('common.delete') }}</button>
            </td>
          </tr>
          <tr v-if="!tenants.length">
            <td colspan="5" class="text-center text-slate-400 py-3">{{ t('shaper.tenants.noTenants') }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
