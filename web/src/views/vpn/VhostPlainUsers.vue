<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/api/client'

const props = defineProps({
  domain: { type: String, required: true },
  passwdPath: { type: String, default: '' },
  groupOptions: { type: Array, default: () => [] },
})

const emit = defineEmits(['changed'])

const { t } = useI18n()
const users = ref([])
const passwdFile = ref('')
const loading = ref(false)
const err = ref('')
const userSearch = ref('')
const userForm = ref({ username: '', password: '', comment: '', group: '' })
const editingUser = ref(null)

const canManage = computed(() => !!String(props.passwdPath || '').trim())

const filteredUsers = computed(() => {
  const q = userSearch.value.trim().toLowerCase()
  if (!q) return users.value
  return users.value.filter(
    (u) =>
      u.username.toLowerCase().includes(q) ||
      (u.comment || '').toLowerCase().includes(q) ||
      (u.group || '').toLowerCase().includes(q),
  )
})

async function load() {
  if (!canManage.value || !props.domain) return
  loading.value = true
  err.value = ''
  try {
    const d = await api.get(
      `/api/v1/vpn/ocserv/vhosts/users?domain=${encodeURIComponent(props.domain)}`,
    )
    users.value = d.users || []
    passwdFile.value = d.passwd_path || props.passwdPath
  } catch (e) {
    err.value = e.message
  } finally {
    loading.value = false
  }
}

async function addUser() {
  err.value = ''
  try {
    await api.post(`/api/v1/vpn/ocserv/vhosts/users?domain=${encodeURIComponent(props.domain)}`, userForm.value)
    userForm.value = { username: '', password: '', comment: '', group: '' }
    await load()
    emit('changed')
  } catch (e) {
    err.value = e.message
  }
}

function startEditUser(u) {
  editingUser.value = { username: u.username, password: '', comment: u.comment || '', group: u.group || '' }
}

function cancelEditUser() {
  editingUser.value = null
}

async function saveEditUser() {
  if (!editingUser.value) return
  err.value = ''
  try {
    const body = {
      username: editingUser.value.username,
      comment: editingUser.value.comment,
      group: editingUser.value.group,
    }
    if (editingUser.value.password) body.password = editingUser.value.password
    await api.put(
      `/api/v1/vpn/ocserv/vhosts/users?domain=${encodeURIComponent(props.domain)}`,
      body,
    )
    editingUser.value = null
    await load()
    emit('changed')
  } catch (e) {
    err.value = e.message
  }
}

async function delUser(name) {
  if (!confirm(t('ocserv.deleteUserConfirm', { name }))) return
  err.value = ''
  try {
    await api.del(
      `/api/v1/vpn/ocserv/vhosts/users?domain=${encodeURIComponent(props.domain)}&username=${encodeURIComponent(name)}`,
    )
    await load()
    emit('changed')
  } catch (e) {
    err.value = e.message
  }
}

watch(() => [props.domain, props.passwdPath], load, { immediate: true })
onMounted(load)
</script>

<template>
  <div class="space-y-3">
    <p v-if="err" class="text-sm text-red-600">{{ err }}</p>
    <p v-if="loading" class="text-sm text-slate-500">{{ t('common.loading') }}</p>

    <template v-if="canManage">
      <p class="text-xs font-mono bg-white border border-slate-200 rounded px-2 py-1">
        {{ t('ocserv.vhostPlainPasswd') }}: {{ passwdFile || passwdPath }}
      </p>

      <div v-if="editingUser" class="border rounded-lg p-4 bg-blue-50/50 space-y-3">
        <h4 class="text-sm font-medium">{{ t('ocserv.editUser', { name: editingUser.username }) }}</h4>
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-2 text-sm">
          <label class="text-sm">
            {{ t('ocserv.newPassword') }}
            <input v-model="editingUser.password" type="password" class="input w-full mt-1" :placeholder="t('ocserv.passwordMinPh')" />
          </label>
          <label class="text-sm">
            {{ t('ocserv.colGroup') }}
            <select v-model="editingUser.group" class="input w-full mt-1">
              <option value="">{{ t('ocserv.groupDefaultOpt') }}</option>
              <option v-for="g in groupOptions" :key="g.name" :value="g.name">{{ g.label || g.name }}</option>
            </select>
          </label>
          <label class="text-sm sm:col-span-2">
            {{ t('common.comment') }}
            <input v-model="editingUser.comment" class="input w-full mt-1" />
          </label>
        </div>
        <div class="flex gap-2">
          <button type="button" class="btn-primary text-sm" @click="saveEditUser">{{ t('common.save') }}</button>
          <button type="button" class="btn-secondary text-sm" @click="cancelEditUser">{{ t('common.cancel') }}</button>
        </div>
      </div>

      <div class="grid grid-cols-1 sm:grid-cols-4 gap-2 text-sm">
        <input v-model="userForm.username" class="input" :placeholder="t('ocserv.usernamePh')" />
        <input v-model="userForm.password" type="password" class="input" :placeholder="t('ocserv.passwordMinPh')" />
        <input v-model="userForm.comment" class="input" :placeholder="t('common.comment')" />
        <select v-model="userForm.group" class="input">
          <option value="">{{ t('ocserv.groupOptional') }}</option>
          <option v-for="g in groupOptions" :key="g.name" :value="g.name">{{ g.label || g.name }}</option>
        </select>
      </div>
      <button type="button" class="btn-secondary text-sm" @click="addUser">{{ t('ocserv.addUser') }}</button>

      <input
        v-model="userSearch"
        type="search"
        class="input w-full max-w-md text-sm"
        :placeholder="t('ocserv.searchUsers')"
      />

      <div class="table-wrap overflow-x-auto">
        <table class="data w-full text-sm">
          <thead>
            <tr>
              <th>{{ t('ocserv.colUser') }}</th>
              <th>{{ t('ocserv.colComment') }}</th>
              <th>{{ t('ocserv.colGroup') }}</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="u in filteredUsers" :key="u.username">
              <td class="font-mono">{{ u.username }}</td>
              <td>{{ u.comment || '—' }}</td>
              <td>{{ u.group || '—' }}</td>
              <td class="whitespace-nowrap space-x-2">
                <button type="button" class="text-blue-600 text-sm" @click="startEditUser(u)">{{ t('common.edit') }}</button>
                <button type="button" class="text-red-600 text-sm" @click="delUser(u.username)">{{ t('common.delete') }}</button>
              </td>
            </tr>
            <tr v-if="!filteredUsers.length && !loading">
              <td colspan="4" class="text-center text-slate-400 py-6">
                {{ users.length && userSearch.trim() ? t('ocserv.noUsersMatch') : t('ocserv.noUsers') }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <p class="text-xs text-slate-500">{{ t('ocserv.vhostUsersPlainSyncHint') }}</p>
    </template>

    <div v-else class="vhost-panel vhost-hint space-y-2">
      <p>{{ t('ocserv.vhostUsersPlainNeedPath') }}</p>
      <button type="button" class="text-blue-700 text-sm underline" @click="$emit('go-auth')">
        {{ t('ocserv.vhostUsersPlainGoAuth') }}
      </button>
    </div>
  </div>
</template>
