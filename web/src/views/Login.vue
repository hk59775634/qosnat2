<script setup>
import { ref, onMounted, computed } from 'vue'
import { displayName } from '@/composables/useBranding'
import { useRouter, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { api } from '@/api/client'
import LanguageSwitcher from '@/components/LanguageSwitcher.vue'

const router = useRouter()
const route = useRoute()
const { t } = useI18n()

onMounted(async () => {
  try {
    const h = await api.health()
    if (h.suggest_https) {
      tlsPending.value = true
      const port = h.admin_port || location.port || '8080'
      const host = location.hostname || 'localhost'
      httpsHint.value = t('login.httpsPending', { host, port })
    } else if (h.tls_active && location.protocol === 'http:') {
      const port = h.admin_port || location.port || '8080'
      const host = location.hostname || 'localhost'
      httpsHint.value = t('login.httpsSuggest', { host, port })
    }
  } catch {
    /* ignore */
  }
})
const user = ref('admin')
const pass = ref('')
const err = ref('')
const loading = ref(false)
const httpsHint = ref('')
const tlsPending = ref(false)

const loginTitle = computed(() => t('login.titleTpl', { name: displayName.value }))

async function submit() {
  err.value = ''
  loading.value = true
  try {
    await api.login(user.value, pass.value)
    const h = await api.health().catch(() => ({}))
    if (h.setup_required) {
      router.push('/setup')
    } else {
      router.push(route.query.redirect || '/')
    }
  } catch (e) {
    err.value = e.message || t('login.failed')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="min-h-screen flex items-center justify-center bg-gradient-to-br from-slate-700 to-pfsense-nav relative">
    <div class="absolute top-4 right-4">
      <LanguageSwitcher />
    </div>
    <form class="card w-full max-w-md p-8" @submit.prevent="submit">
      <h1 class="text-xl font-semibold text-pfsense-nav mb-1">{{ loginTitle }}</h1>
      <p class="text-sm text-slate-500 mb-3">{{ t('login.subtitle') }}</p>
      <p
        v-if="httpsHint"
        class="text-sm mb-3 p-2 rounded"
        :class="tlsPending ? 'bg-amber-50 text-amber-800' : 'bg-blue-50 text-blue-800'"
      >
        {{ httpsHint }}
      </p>
      <label class="block text-sm mb-1">{{ t('login.username') }}</label>
      <input v-model="user" class="input-field mb-4" autocomplete="username" />
      <label class="block text-sm mb-1">{{ t('login.password') }}</label>
      <input v-model="pass" type="password" class="input-field mb-4" autocomplete="current-password" />
      <p v-if="err" class="text-red-600 text-sm mb-3">{{ err }}</p>
      <button type="submit" class="btn-primary w-full" :disabled="loading">
        {{ loading ? t('login.submitting') : t('login.submit') }}
      </button>
    </form>
  </div>
</template>
