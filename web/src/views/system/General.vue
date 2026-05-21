<script setup>
import { onMounted, ref } from 'vue'
import { api } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'

const cfg = ref(null)
const form = ref({ hostname: '', new_password: '', current_password: '' })
const err = ref('')
const ok = ref('')

async function load() {
  cfg.value = await api.system.general.get()
  form.value.hostname = cfg.value.hostname || ''
}

async function save() {
  err.value = ''
  ok.value = ''
  try {
    await api.system.general.put({
      hostname: form.value.hostname,
      new_password: form.value.new_password || undefined,
      current_password: form.value.current_password || undefined,
    })
    ok.value = '已保存'
    form.value.new_password = ''
    form.value.current_password = ''
    await load()
  } catch (e) {
    err.value = e.message
  }
}

onMounted(load)
</script>

<template>
  <div>
    <PageHeader title="常规设置" description="主机名与管理员密码（需输入当前密码才能修改口令）" />
    <p v-if="ok" class="text-green-700 text-sm mb-2">{{ ok }}</p>
    <p v-if="err" class="text-red-600 text-sm mb-2">{{ err }}</p>
    <div v-if="cfg" class="card p-4 max-w-lg space-y-4">
      <p class="text-sm text-slate-600">
        管理员 <span class="font-mono">{{ cfg.admin_user }}</span> · LAN
        <span class="font-mono">{{ cfg.dev_lan }}</span> · WAN
        <span class="font-mono">{{ cfg.dev_wan }}</span>
      </p>
      <div>
        <label class="text-xs text-slate-500">主机名</label>
        <input v-model="form.hostname" class="input-field mt-1 font-mono" />
      </div>
      <div>
        <label class="text-xs text-slate-500">新密码（至少 8 位，留空不修改）</label>
        <input v-model="form.new_password" type="password" class="input-field mt-1" autocomplete="new-password" />
      </div>
      <div>
        <label class="text-xs text-slate-500">当前密码（修改口令时必填）</label>
        <input v-model="form.current_password" type="password" class="input-field mt-1" autocomplete="current-password" />
      </div>
      <button type="button" class="btn-primary" @click="save">保存</button>
    </div>
  </div>
</template>
