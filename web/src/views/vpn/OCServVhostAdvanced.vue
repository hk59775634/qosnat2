<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { api } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'
import OCServVhostEditor from '@/views/vpn/OCServVhostEditor.vue'
import {
  buildVhostPayload,
  emptyVhostForm,
  normalizeVhostFromApi,
} from '@/lib/ocservVhostForm'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()

const domain = computed(() => decodeURIComponent(route.params.domain || ''))
const globalCfg = ref(null)
const vhostForm = ref(emptyVhostForm())
const vhostRadiusSecret = ref('')
const vhostRadiusSecretSet = ref(false)
const vhostCamouflageSecret = ref('')
const vhostCamouflageSecretSet = ref(false)
const loading = ref(true)
const saving = ref(false)
const err = ref('')
const ok = ref('')
const notFound = ref(false)

async function load() {
  loading.value = true
  err.value = ''
  notFound.value = false
  try {
    const d = await api.get('/api/v1/vpn/ocserv')
    globalCfg.value = d.config || {}
    const vhosts = d.config?.vhosts || []
    const v = vhosts.find((x) => x.domain === domain.value)
    if (!v) {
      notFound.value = true
      return
    }
    vhostForm.value = normalizeVhostFromApi(v)
    const meta = (d.vhosts_meta || []).find((m) => m.domain === domain.value) || {}
    vhostRadiusSecret.value = ''
    vhostRadiusSecretSet.value = !!meta.radius_secret_set
    vhostCamouflageSecret.value = ''
    vhostCamouflageSecretSet.value = !!meta.camouflage_secret_set
  } catch (e) {
    err.value = e.message
  } finally {
    loading.value = false
  }
}

function goBack() {
  router.push({ name: 'vpn-ocserv', query: { tab: 'vhosts' } })
}

async function save() {
  err.value = ''
  ok.value = ''
  saving.value = true
  try {
    const body = buildVhostPayload(vhostForm.value, {
      radiusSecret: vhostRadiusSecret.value,
      camouflageSecret: vhostCamouflageSecret.value,
    })
    await api.put('/api/v1/vpn/ocserv/vhosts', body)
    ok.value = t('ocserv.vhostSaved')
    await load()
  } catch (e) {
    err.value = e.message
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<template>
  <div class="space-y-4 max-w-5xl">
    <PageHeader
      :title="t('ocserv.vhostAdvancedTitle', { domain })"
      :subtitle="t('ocserv.vhostAdvancedSubtitle')"
    >
      <template #actions>
        <button type="button" class="btn-secondary text-sm" @click="goBack">
          {{ t('ocserv.backToVhostList') }}
        </button>
      </template>
    </PageHeader>

    <p v-if="err" class="text-sm text-red-600">{{ err }}</p>
    <p v-if="ok" class="text-sm text-green-700">{{ ok }}</p>

    <div v-if="loading" class="text-sm text-slate-500">{{ t('common.loading') }}</div>

    <div v-else-if="notFound" class="card p-6 text-center space-y-3">
      <p class="text-slate-600">{{ t('ocserv.vhostNotFound', { domain }) }}</p>
      <button type="button" class="btn-secondary" @click="goBack">{{ t('ocserv.backToVhostList') }}</button>
    </div>

    <div v-else class="card p-4 space-y-4">
      <div class="flex items-center gap-2 text-sm border-b border-slate-200 pb-3">
        <span class="text-slate-500">{{ t('ocserv.vhostEditing') }}</span>
        <span class="font-mono font-medium text-blue-800">{{ domain }}</span>
      </div>

      <OCServVhostEditor
        v-model="vhostForm"
        :global-cfg="globalCfg"
        :editing="true"
        domain-readonly
        v-model:radius-secret="vhostRadiusSecret"
        :radius-secret-set="vhostRadiusSecretSet"
        v-model:camouflage-secret="vhostCamouflageSecret"
        :camouflage-secret-set="vhostCamouflageSecretSet"
        @users-changed="load"
      />

      <div class="flex gap-2 pt-2 border-t border-slate-100">
        <button type="button" class="btn-primary" :disabled="saving" @click="save">
          {{ saving ? t('common.processing') : t('common.save') }}
        </button>
        <button type="button" class="btn-secondary" @click="goBack">{{ t('common.cancel') }}</button>
      </div>
      <p class="text-xs text-slate-500">{{ t('ocserv.vhostAdvancedApplyHint') }}</p>
    </div>
  </div>
</template>
