<script setup>
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'

const { t } = useI18n()
const keys = ref([])
const name = ref('')
const role = ref('admin')
const created = ref(null)
const err = ref('')

async function load() {
  keys.value = await api.system.apiKeys.list()
}

async function add() {
  err.value = ''
  created.value = null
  try {
    const res = await api.system.apiKeys.create(name.value, role.value)
    created.value = res
    name.value = ''
    role.value = 'admin'
    await load()
  } catch (e) {
    err.value = e.message
  }
}

async function remove(id) {
  if (!confirm(t('system.apiKeys.confirmDelete'))) return
  await api.system.apiKeys.del(id)
  await load()
}

function roleLabel(r) {
  if (r === 'readonly') return t('system.apiKeys.roleReadOnly')
  if (r === 'firewall') return t('system.apiKeys.roleFirewall')
  return t('system.apiKeys.roleAdmin')
}

onMounted(load)
</script>

<template>
  <div class="page-stack">
    <PageHeader
      :title="t('system.apiKeys.title')"
      :description="t('system.apiKeys.description')" :err="err" />
    <div v-if="created?.key" class="card card-body mb-4 bg-amber-50 border-amber-200">
      <p class="text-sm font-medium text-amber-900 mb-1">{{ t('system.apiKeys.newSecret') }}</p>
      <code class="text-xs break-all">{{ created.key }}</code>
      <p v-if="created.role" class="text-xs text-amber-800 mt-2">{{ roleLabel(created.role) }}</p>
    </div>
    <div class="card card-body mb-0 flex flex-wrap gap-2">
      <input v-model="name" class="input-field flex-1 min-w-[10rem]" :placeholder="t('system.apiKeys.namePlaceholder')" />
      <select v-model="role" class="input-field w-auto">
        <option value="admin">{{ t('system.apiKeys.roleAdmin') }}</option>
        <option value="firewall">{{ t('system.apiKeys.roleFirewall') }}</option>
        <option value="readonly">{{ t('system.apiKeys.roleReadOnly') }}</option>
      </select>
      <button type="button" class="btn-primary shrink-0" @click="add">{{ t('common.create') }}</button>
    </div>
    <p class="text-xs text-slate-500 -mt-2">{{ t('system.apiKeys.roleHint') }}</p>
    <div class="card overflow-x-auto">
      <table class="data w-full text-sm">
        <thead>
          <tr>
            <th>{{ t('system.apiKeys.colName') }}</th>
            <th>{{ t('system.apiKeys.colRole') }}</th>
            <th>{{ t('system.apiKeys.colPrefix') }}</th>
            <th>{{ t('system.apiKeys.colCreated') }}</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="k in keys" :key="k.id">
            <td>{{ k.name }}</td>
            <td>{{ roleLabel(k.role) }}</td>
            <td class="font-mono text-xs">{{ k.key_prefix }}</td>
            <td class="text-xs text-slate-500">{{ k.created_at }}</td>
            <td>
              <button type="button" class="text-red-600 text-xs" @click="remove(k.id)">{{ t('common.delete') }}</button>
            </td>
          </tr>
          <tr v-if="!keys.length">
            <td colspan="5" class="text-slate-500 py-4">{{ t('system.apiKeys.empty') }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
