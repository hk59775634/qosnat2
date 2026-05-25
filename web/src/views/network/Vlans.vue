<script setup>
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'

const { t } = useI18n()
const vlans = ref([])
const netplanPath = ref('')
const err = ref('')
const ok = ref('')
const editing = ref(null)
const form = ref({ parent: '', vid: 100, ipv4: '', up: true })

function parseIPv4(text) {
  return text
    .split(/[\n,]+/)
    .map((s) => s.trim())
    .filter(Boolean)
}

async function load() {
  const d = await api.network.vlans.list()
  vlans.value = d.vlans || []
  netplanPath.value = d.netplan_path || ''
}

function resetForm() {
  editing.value = null
  form.value = { parent: '', vid: 100, ipv4: '', up: true }
}

function startEdit(v) {
  editing.value = v.id
  form.value = {
    parent: v.parent,
    vid: v.vid,
    ipv4: (v.ipv4 || []).join('\n'),
    up: v.up !== false,
  }
}

async function submit() {
  err.value = ''
  ok.value = ''
  const payload = {
    parent: form.value.parent,
    vid: form.value.vid,
    ipv4: parseIPv4(form.value.ipv4),
    up: form.value.up,
  }
  try {
    if (editing.value) {
      await api.network.vlans.put(editing.value, payload)
      ok.value = t('common.saved')
    } else {
      await api.network.vlans.add(payload)
      ok.value = t('common.saved')
    }
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
    await api.network.vlans.del(id)
    if (editing.value === id) resetForm()
    await load()
  } catch (e) {
    err.value = e.message
  }
}

async function previewNetplan() {
  const d = await api.network.netplan.preview()
  ok.value = `${t('network.vlans.previewNetplan')} ${d.path || netplanPath.value}`
}

onMounted(load)
</script>

<template>
  <div class="page-stack">
    <PageHeader :title="t('network.vlans.title')" :description="t('network.vlans.description')" />
    <p v-if="ok" class="text-green-700 text-sm mb-2">{{ ok }}</p>
    <p v-if="err" class="text-red-600 text-sm mb-2">{{ err }}</p>

    <div class="card card-body mb-0 space-y-3 text-sm">
      <h3 class="font-medium">{{ editing ? t('network.vlans.edit') : t('network.vlans.new') }}</h3>
      <div class="grid sm:grid-cols-2 gap-3">
        <div>
          <label class="text-xs text-slate-500">{{ t('network.vlans.parent') }}</label>
          <input v-model="form.parent" class="input-field mt-1 font-mono" />
        </div>
        <div>
          <label class="text-xs text-slate-500">VID</label>
          <input v-model.number="form.vid" type="number" min="1" max="4094" class="input-field mt-1" />
        </div>
        <div class="sm:col-span-2">
          <label class="text-xs text-slate-500">{{ t('network.vlans.ipv4') }}</label>
          <textarea v-model="form.ipv4" class="input-field mt-1 font-mono h-16" />
        </div>
        <label class="flex items-center gap-2">
          <input v-model="form.up" type="checkbox" /> {{ t('network.vlans.upOnCreate') }}
        </label>
      </div>
      <div class="flex flex-wrap gap-2">
        <button type="button" class="btn-primary" @click="submit">{{ editing ? t('common.save') : t('common.create') }}</button>
        <button v-if="editing" type="button" class="btn-secondary" @click="resetForm">{{ t('common.cancel') }}</button>
        <button type="button" class="btn-secondary text-xs" @click="previewNetplan">{{ t('network.vlans.previewNetplan') }}</button>
      </div>
    </div>

    <div class="table-wrap card">
      <table class="data w-full text-sm">
        <thead>
          <tr><th>{{ t('common.name') }}</th><th>{{ t('network.vlans.parent') }}</th><th>VID</th><th>{{ t('network.vlans.ipv4') }}</th><th></th></tr>
        </thead>
        <tbody>
          <tr v-for="v in vlans" :key="v.id" :class="{ 'bg-blue-50': editing === v.id }">
            <td class="font-mono">{{ v.name || `${v.parent}.${v.vid}` }}</td>
            <td class="font-mono">{{ v.parent }}</td>
            <td>{{ v.vid }}</td>
            <td class="font-mono text-xs">{{ (v.ipv4 || []).join(', ') || '—' }}</td>
            <td class="text-right whitespace-nowrap space-x-2">
              <button type="button" class="text-xs text-blue-600" @click="startEdit(v)">{{ t('common.edit') }}</button>
              <button type="button" class="text-red-600 text-xs" @click="remove(v.id)">{{ t('common.delete') }}</button>
            </td>
          </tr>
          <tr v-if="!vlans.length">
            <td colspan="5" class="text-center text-slate-400 py-3">{{ t('network.vlans.noVlan') }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
