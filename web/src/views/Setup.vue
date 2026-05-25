<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { api } from '@/api/client'
import LanguageSwitcher from '@/components/LanguageSwitcher.vue'

const router = useRouter()
const { t } = useI18n()
const step = ref(0)
const steps = computed(() => [
  t('setup.steps.welcome'),
  t('setup.steps.admin'),
  t('setup.steps.iface'),
  t('setup.steps.nat'),
  t('setup.steps.done'),
])
const loading = ref(false)
const err = ref('')
const ifaces = ref([])

const form = ref({
  admin_user: 'admin',
  admin_pass: '',
  admin_pass2: '',
  dev_lan: '',
  dev_wan: '',
  policy_routes: '10.0.0.0/8',
  shared_ip: '',
  hostname: 'qosnat2',
  enable_dhcp: false,
  apply_dataplane: true,
})

const progress = computed(() => Math.round(((step.value + 1) / steps.value.length) * 100))

onMounted(async () => {
  try {
    const st = await api.setup.status()
    if (st.setup_complete) {
      router.replace('/')
      return
    }
    const res = await api.setup.interfaces()
    ifaces.value = res.interfaces || []
    if (!form.value.dev_wan && ifaces.value.length) {
      const up = ifaces.value.filter((i) => i.up)
      if (up.length >= 1) form.value.dev_wan = up[0].name
      if (up.length >= 2) form.value.dev_lan = up[1].name
    }
  } catch (e) {
    err.value = e.message || t('setup.apiUnreachable')
  }
})

function next() {
  err.value = ''
  if (step.value === 1) {
    if (form.value.admin_pass.length < 8) {
      err.value = t('setup.passMin')
      return
    }
    if (form.value.admin_pass !== form.value.admin_pass2) {
      err.value = t('setup.passMismatch')
      return
    }
  }
  if (step.value === 2) {
    if (!form.value.dev_wan) {
      err.value = t('setup.wanRequired')
      return
    }
    if (form.value.dev_lan && form.value.dev_lan === form.value.dev_wan) {
      err.value = t('setup.lanWanDiff')
      return
    }
    if (form.value.enable_dhcp && !form.value.dev_lan) {
      err.value = t('setup.dhcpNeedsLan')
      return
    }
  }
  if (step.value < steps.value.length - 1) step.value++
}

function back() {
  err.value = ''
  if (step.value > 0) step.value--
}

async function finish() {
  err.value = ''
  loading.value = true
  try {
    const routes = form.value.policy_routes
      .split(/[\n,]+/)
      .map((s) => s.trim())
      .filter(Boolean)
    const shared = form.value.shared_ip.trim() ? [form.value.shared_ip.trim()] : []
    await api.setup.complete({
      admin_user: form.value.admin_user,
      admin_pass: form.value.admin_pass,
      dev_lan: form.value.dev_lan,
      dev_wan: form.value.dev_wan,
      policy_routes: routes.length ? routes : ['10.0.0.0/8'],
      shared_ips: shared,
      hostname: form.value.hostname,
      enable_dhcp: form.value.enable_dhcp,
      apply_dataplane: form.value.apply_dataplane,
    })
    router.replace('/')
  } catch (e) {
    err.value = e.message || t('setup.setupFailed')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="min-h-screen bg-gradient-to-br from-slate-800 to-pfsense-nav flex items-center justify-center p-4 relative">
    <div class="absolute top-4 right-4">
      <LanguageSwitcher />
    </div>
    <div class="card w-full max-w-xl p-8 shadow-xl">
      <div class="mb-3">
        <h1 class="text-2xl font-semibold text-pfsense-nav">{{ t('setup.title') }}</h1>
        <div class="mt-4 h-2 bg-slate-200 rounded-full overflow-hidden">
          <div class="h-full bg-blue-600 transition-all" :style="{ width: progress + '%' }" />
        </div>
        <p class="text-xs text-slate-400 mt-2">
          {{ t('setup.stepOf', { n: step + 1, m: steps.length }) }}：{{ steps[step] }}
        </p>
      </div>

      <div v-if="step === 0" class="space-y-3 text-sm text-slate-600">
        <p>qosnat2</p>
      </div>

      <div v-else-if="step === 1" class="space-y-3">
        <div>
          <label class="block text-sm mb-1">{{ t('setup.adminUser') }}</label>
          <input v-model="form.admin_user" class="input-field" autocomplete="username" />
        </div>
        <div>
          <label class="block text-sm mb-1">{{ t('setup.password') }}</label>
          <input v-model="form.admin_pass" type="password" class="input-field" autocomplete="new-password" />
        </div>
        <div>
          <label class="block text-sm mb-1">{{ t('setup.confirmPassword') }}</label>
          <input v-model="form.admin_pass2" type="password" class="input-field" autocomplete="new-password" />
        </div>
      </div>

      <div v-else-if="step === 2" class="space-y-3">
        <div>
          <label class="block text-sm mb-1">{{ t('setup.wanIface') }} *</label>
          <select v-model="form.dev_wan" class="input-field">
            <option value="">{{ t('setup.choose') }}</option>
            <option v-for="i in ifaces" :key="'w-' + i.name" :value="i.name">
              {{ i.name }} {{ i.up ? '(UP)' : '' }} {{ i.addrs?.[0] || '' }}
            </option>
          </select>
        </div>
        <div>
          <label class="block text-sm mb-1">{{ t('setup.lanIface') }}</label>
          <select v-model="form.dev_lan" class="input-field">
            <option value="">{{ t('setup.lanLater') }}</option>
            <option v-for="i in ifaces" :key="'l-' + i.name" :value="i.name">
              {{ i.name }} {{ i.up ? '(UP)' : '' }} {{ i.addrs?.[0] || '' }}
            </option>
          </select>
        </div>
        <div>
          <label class="block text-sm mb-1">{{ t('setup.hostname') }}</label>
          <input v-model="form.hostname" class="input-field" />
        </div>
        <label class="flex items-center gap-2 text-sm" :class="{ 'opacity-50': !form.dev_lan }">
          <input v-model="form.enable_dhcp" type="checkbox" :disabled="!form.dev_lan" />
          {{ t('setup.dhcpAfterSetup') }}
        </label>
      </div>

      <div v-else-if="step === 3" class="space-y-3">
        <div>
          <label class="block text-sm mb-1">{{ t('setup.policyCidrs') }}</label>
          <textarea v-model="form.policy_routes" class="input-field h-20" />
        </div>
        <div>
          <label class="block text-sm mb-1">{{ t('setup.sharedIps') }}</label>
          <input v-model="form.shared_ip" class="input-field" :placeholder="t('setup.sharedEmpty')" />
        </div>
        <label class="flex items-center gap-2 text-sm">
          <input v-model="form.apply_dataplane" type="checkbox" />
          {{ t('setup.applyNow') }}
        </label>
      </div>

      <div v-else class="space-y-2 text-sm text-slate-600">
        <p><strong>{{ t('setup.summaryUser') }}：</strong>{{ form.admin_user }}</p>
        <p>
          <strong>{{ t('setup.summaryLan') }}：</strong>{{ form.dev_lan || t('common.notSet') }}
          · <strong>{{ t('setup.summaryWan') }}：</strong>{{ form.dev_wan }}
        </p>
        <p><strong>{{ t('setup.summaryPolicy') }}：</strong>{{ form.policy_routes }}</p>
        <p v-if="form.shared_ip"><strong>{{ t('setup.summaryShared') }}：</strong>{{ form.shared_ip }}</p>
      </div>

      <p v-if="err" class="text-red-600 text-sm mt-4">{{ err }}</p>

      <div class="flex justify-between mt-8 gap-3">
        <button v-if="step > 0" type="button" class="btn-secondary" :disabled="loading" @click="back">
          {{ t('setup.prev') }}
        </button>
        <span v-else />
        <button
          v-if="step < steps.length - 1"
          type="button"
          class="btn-primary"
          :disabled="loading"
          @click="next"
        >
          {{ t('setup.next') }}
        </button>
        <button v-else type="button" class="btn-primary" :disabled="loading" @click="finish">
          {{ loading ? t('setup.applying') : t('setup.finish') }}
        </button>
      </div>
    </div>
  </div>
</template>
