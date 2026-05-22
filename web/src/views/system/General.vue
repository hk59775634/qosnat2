<script setup>
import { onMounted, ref } from 'vue'
import { api } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'

const cfg = ref(null)
const form = ref({
  hostname: '',
  new_password: '',
  current_password: '',
  tls_enabled: false,
  tls_cert: '',
  tls_key: '',
})
const err = ref('')
const ok = ref('')
const warn = ref('')

async function load() {
  cfg.value = await api.system.general.get()
  form.value.hostname = cfg.value.hostname || ''
  form.value.tls_enabled = cfg.value.tls?.tls_enabled ?? false
  form.value.tls_cert = ''
  form.value.tls_key = ''
}

function readFile(e, field) {
  const f = e.target?.files?.[0]
  if (!f) return
  const reader = new FileReader()
  reader.onload = () => {
    form.value[field] = String(reader.result || '')
  }
  reader.readAsText(f)
  e.target.value = ''
}

async function save() {
  err.value = ''
  ok.value = ''
  warn.value = ''
  try {
    const body = {
      hostname: form.value.hostname,
      new_password: form.value.new_password || undefined,
      current_password: form.value.current_password || undefined,
    }
    const tlsChange =
      form.value.tls_enabled !== (cfg.value?.tls?.tls_enabled ?? false) ||
      form.value.tls_cert.trim() !== '' ||
      form.value.tls_key.trim() !== ''
    if (tlsChange) {
      body.tls_enabled = form.value.tls_enabled
      if (form.value.tls_cert.trim()) body.tls_cert = form.value.tls_cert.trim()
      if (form.value.tls_key.trim()) body.tls_key = form.value.tls_key.trim()
      if (!form.value.current_password) {
        err.value = '修改 HTTPS 设置需填写当前密码'
        return
      }
    }
    const res = await api.system.general.put(body)
    ok.value = '已保存'
    if (res.warning) warn.value = res.warning
    form.value.new_password = ''
    form.value.current_password = ''
    form.value.tls_cert = ''
    form.value.tls_key = ''
    await load()
  } catch (e) {
    err.value = e.message
  }
}

onMounted(load)
</script>

<template>
  <div class="page-stack">
    <PageHeader
      title="常规设置"
      description="主机名、管理员密码、HTTPS（证书粘贴或上传）；修改 HTTPS 需当前密码。"
    />
    <p v-if="ok" class="text-green-700 text-sm mb-2">{{ ok }}</p>
    <p v-if="warn" class="text-amber-700 text-sm mb-2">{{ warn }}</p>
    <p v-if="err" class="text-red-600 text-sm mb-2">{{ err }}</p>

    <div v-if="cfg" class="card card-body space-y-6">
      <section class="space-y-4">
        <h3 class="text-sm font-semibold text-slate-800">基本</h3>
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
          <label class="text-xs text-slate-500">当前密码（改口令或 HTTPS 时必填）</label>
          <input v-model="form.current_password" type="password" class="input-field mt-1" autocomplete="current-password" />
        </div>
      </section>

      <section class="space-y-4 pt-4 border-t border-slate-200">
        <h3 class="text-sm font-semibold text-slate-800">HTTPS</h3>
        <p class="text-xs text-slate-500">
          启用后写入 <span class="font-mono">/etc/qosnat2/tls.crt</span> 与
          <span class="font-mono">tls.key</span>，并更新 <span class="font-mono">env</span> 后重启
          <span class="font-mono">qosnatd</span>。保存后请使用 <span class="font-mono">https://</span> 访问。
        </p>
        <div v-if="cfg.tls" class="text-xs text-slate-600 space-y-1 bg-slate-50 rounded p-3">
          <p>
            运行中:
            <span :class="cfg.tls.tls_active ? 'text-green-700' : 'text-slate-500'">
              {{ cfg.tls.tls_active ? 'HTTPS 已生效' : 'HTTP' }}
            </span>
            · 配置开关: {{ cfg.tls.tls_enabled ? '开' : '关' }}
          </p>
          <p v-if="cfg.tls.has_cert_file">证书: {{ cfg.tls.cert_subject || cfg.tls.cert_path }}</p>
          <p v-if="cfg.tls.cert_not_after">到期: {{ cfg.tls.cert_not_after }}</p>
          <p v-if="!cfg.tls.has_cert_file && form.tls_enabled" class="text-amber-700">尚未上传证书，请粘贴或选择文件后保存</p>
        </div>
        <label class="flex items-center gap-2 text-sm cursor-pointer">
          <input v-model="form.tls_enabled" type="checkbox" class="rounded" />
          启用 HTTPS
        </label>
        <div>
          <label class="text-xs text-slate-500">证书 PEM（含 CERTIFICATE 头尾，可粘贴或上传）</label>
          <textarea
            v-model="form.tls_cert"
            class="input-field mt-1 font-mono text-xs h-28"
            placeholder="-----BEGIN CERTIFICATE-----"
            spellcheck="false"
          />
          <input type="file" accept=".pem,.crt,.cer,.txt" class="text-xs mt-1" @change="readFile($event, 'tls_cert')" />
        </div>
        <div>
          <label class="text-xs text-slate-500">私钥 PEM（含 PRIVATE KEY 头尾，可粘贴或上传）</label>
          <textarea
            v-model="form.tls_key"
            class="input-field mt-1 font-mono text-xs h-28"
            placeholder="-----BEGIN PRIVATE KEY-----"
            spellcheck="false"
          />
          <input type="file" accept=".pem,.key,.txt" class="text-xs mt-1" @change="readFile($event, 'tls_key')" />
        </div>
      </section>

      <button type="button" class="btn-primary" @click="save">保存</button>
    </div>
  </div>
</template>
