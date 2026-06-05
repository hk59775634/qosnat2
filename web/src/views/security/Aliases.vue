<script setup>
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'

const { t } = useI18n()
const aliases = ref([])
const form = ref({ name: '', type: 'ipv4_addr', membersText: '', url: '', comment: '' })
const err = ref('')
const ok = ref('')

async function load() {
  const d = await api.firewall.aliases.list()
  aliases.value = d.aliases || []
}

function validateForm() {
  if (!String(form.value.name || '').trim()) {
    err.value = t('security.aliases.errName')
    return false
  }
  const members = form.value.membersText.split(/[\n,]+/).map((s) => s.trim()).filter(Boolean)
  const hasURL = !!String(form.value.url || '').trim()
  if (!members.length && !hasURL) {
    err.value = t('security.aliases.errMembers')
    return false
  }
  return true
}

async function add() {
  err.value = ''
  ok.value = ''
  if (!validateForm()) return
  const members = form.value.membersText.split(/[\n,]+/).map((s) => s.trim()).filter(Boolean)
  try {
    const body = {
      name: form.value.name.trim(),
      type: 'ipv4_addr',
      members,
      comment: form.value.comment,
    }
    const url = String(form.value.url || '').trim()
    if (url) body.url = url
    await api.firewall.aliases.add(body)
    form.value = { name: '', type: 'ipv4_addr', membersText: '', url: '', comment: '' }
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
      <p class="text-xs text-slate-500">{{ t('security.aliases.typeIpv4') }}</p>
      <textarea
        v-model="form.membersText"
        class="input-field font-mono text-xs h-24"
        :placeholder="t('security.aliases.membersPh')"
      />
      <input v-model="form.url" class="input-field font-mono text-xs" :placeholder="t('security.aliases.urlPh')" />
      <p class="text-xs text-slate-500">{{ t('security.aliases.urlHint') }}</p>
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
            <th>URL</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="a in aliases" :key="a.name">
            <td class="font-mono">{{ a.name }}</td>
            <td>
              <span v-if="(a.type || 'ipv4_addr') === 'asn'" class="text-amber-700 text-xs">
                {{ t('security.aliases.asnUnsupported') }}
              </span>
              <span v-else>{{ a.type || 'ipv4_addr' }}</span>
            </td>
            <td class="text-xs font-mono">
              {{ t('security.aliases.memberCount', { n: (a.members || []).length }) }}
              <span v-if="(a.members || []).length <= 8" class="text-slate-500"> · {{ (a.members || []).join(', ') }}</span>
            </td>
            <td class="text-xs font-mono max-w-xs truncate">
              <template v-if="a.url">
                <a :href="a.url" target="_blank" rel="noopener" class="text-blue-600 hover:underline">{{ a.url }}</a>
                <p v-if="a.url_fetched_at" class="text-slate-400">{{ a.url_fetched_at }}</p>
              </template>
              <span v-else class="text-slate-400">—</span>
            </td>
            <td class="whitespace-nowrap space-x-2">
              <button v-if="a.url" type="button" class="text-indigo-600 text-xs" @click="refreshAlias(a.name)">
                {{ t('security.aliases.refreshUrl') }}
              </button>
              <button type="button" class="text-red-600 text-xs" @click="remove(a.name)">{{ t('common.delete') }}</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
