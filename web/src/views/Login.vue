<script setup>
import { ref, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { api } from '@/api/client'

const router = useRouter()
const route = useRoute()

onMounted(async () => {
  try {
    const h = await api.health()
    if (h.setup_required) router.replace({ name: 'setup' })
    if (h.suggest_https) {
      tlsPending.value = true
      const port = h.admin_port || location.port || '8080'
      const host = location.hostname || 'localhost'
      httpsHint.value = `HTTPS 已配置但尚未生效，请重启 qosnatd 后使用 https://${host}:${port}/`
    } else if (h.tls_active && location.protocol === 'http:') {
      const port = h.admin_port || location.port || '8080'
      const host = location.hostname || 'localhost'
      httpsHint.value = `管理端已启用 HTTPS，建议使用 https://${host}:${port}/ 访问`
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

async function submit() {
  err.value = ''
  loading.value = true
  try {
    await api.login(user.value, pass.value)
    router.push(route.query.redirect || '/')
  } catch (e) {
    err.value = e.message || '登录失败'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="min-h-screen flex items-center justify-center bg-gradient-to-br from-slate-700 to-pfsense-nav">
    <form class="card w-full max-w-md p-8" @submit.prevent="submit">
      <h1 class="text-xl font-semibold text-pfsense-nav mb-1">qosnat2 登录</h1>
      <p class="text-sm text-slate-500 mb-3">管理控制台</p>
      <p
        v-if="httpsHint"
        class="text-sm mb-3 p-2 rounded"
        :class="tlsPending ? 'bg-amber-50 text-amber-800' : 'bg-blue-50 text-blue-800'"
      >
        {{ httpsHint }}
      </p>
      <label class="block text-sm mb-1">用户名</label>
      <input v-model="user" class="input-field mb-4" autocomplete="username" />
      <label class="block text-sm mb-1">密码</label>
      <input v-model="pass" type="password" class="input-field mb-4" autocomplete="current-password" />
      <p v-if="err" class="text-red-600 text-sm mb-3">{{ err }}</p>
      <button type="submit" class="btn-primary w-full" :disabled="loading">
        {{ loading ? '登录中…' : '登录' }}
      </button>
    </form>
  </div>
</template>
