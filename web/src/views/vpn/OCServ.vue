<script setup>
import { onMounted, ref } from 'vue'
import { api } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'

const cfg = ref(null)
const status = ref(null)
const installScript = ref('')
const users = ref([])
const err = ref('')
const ok = ref('')
const installing = ref(false)

const userForm = ref({ username: '', password: '', comment: '' })

async function load() {
  const d = await api.get('/api/v1/vpn/ocserv')
  cfg.value = d.config || {}
  status.value = d.status || {}
  installScript.value = d.install_script || ''
  users.value = (d.config?.users || []).map((u) => ({ ...u }))
}

async function runInstall() {
  err.value = ''
  ok.value = ''
  installing.value = true
  try {
    const r = await api.post('/api/v1/vpn/ocserv/install', {})
    ok.value = r.message || '已在后台编译安装，请数分钟后刷新状态'
  } catch (e) {
    err.value = e.message
  } finally {
    installing.value = false
  }
}

async function save() {
  err.value = ''
  ok.value = ''
  try {
    cfg.value.users = users.value
    await api.put('/api/v1/vpn/ocserv', cfg.value)
    ok.value = '配置已保存'
    await load()
  } catch (e) {
    err.value = e.message
  }
}

async function apply() {
  err.value = ''
  ok.value = ''
  try {
    await save()
    await api.post('/api/v1/vpn/ocserv/apply', {})
    ok.value = cfg.value.enabled ? '已应用并启动 ocserv' : '已停止 ocserv'
    await load()
  } catch (e) {
    err.value = e.message
  }
}

async function addUser() {
  err.value = ''
  try {
    await api.post('/api/v1/vpn/ocserv/users', userForm.value)
    userForm.value = { username: '', password: '', comment: '' }
    ok.value = '用户已添加（保存并 Apply 后写入 ocpasswd）'
    await load()
  } catch (e) {
    err.value = e.message
  }
}

async function delUser(name) {
  if (!confirm(`删除用户 ${name}?`)) return
  try {
    await api.del(`/api/v1/vpn/ocserv/users?username=${encodeURIComponent(name)}`)
    await load()
  } catch (e) {
    err.value = e.message
  }
}

onMounted(load)
</script>

<template>
  <div>
    <PageHeader title="OpenConnect (ocserv)" subtitle="AnyConnect 兼容 SSL VPN；需先源码安装 ocserv" />
    <p v-if="err" class="text-sm text-red-600 mb-2">{{ err }}</p>
    <p v-if="ok" class="text-sm text-green-700 mb-2">{{ ok }}</p>

    <div class="card p-4 mb-4 space-y-2">
      <div class="flex flex-wrap gap-4 text-sm">
        <span>已安装: <strong>{{ status?.installed ? '是' : '否' }}</strong></span>
        <span>运行中: <strong>{{ status?.active ? '是' : '否' }}</strong></span>
        <span v-if="status?.version" class="text-slate-500">{{ status.version }}</span>
      </div>
      <p class="text-xs text-slate-500">
        源码安装: <code class="bg-slate-100 px-1">{{ installScript }}</code>
        或 UI「从源码安装」（需 qosnatd 以 root 运行）
      </p>
      <button type="button" class="btn-secondary text-sm" :disabled="installing" @click="runInstall">
        {{ installing ? '安装中…' : '从源码安装（后台）' }}
      </button>
    </div>

    <div v-if="cfg" class="card p-4 space-y-4">
      <label class="flex items-center gap-2">
        <input v-model="cfg.enabled" type="checkbox" />
        启用 ocserv
      </label>
      <div class="grid grid-cols-2 gap-4">
        <label class="block text-sm">TCP 端口 <input v-model.number="cfg.tcp_port" type="number" class="input w-full mt-1" /></label>
        <label class="block text-sm">UDP 端口 <input v-model.number="cfg.udp_port" type="number" class="input w-full mt-1" /></label>
        <label class="block text-sm">地址池网络 <input v-model="cfg.ipv4_network" class="input w-full mt-1" placeholder="10.250.0.0" /></label>
        <label class="block text-sm">掩码 <input v-model="cfg.ipv4_netmask" class="input w-full mt-1" placeholder="255.255.255.0" /></label>
      </div>
      <label class="flex items-center gap-2 text-sm">
        <input v-model="cfg.use_qosnat_tls" type="checkbox" />
        使用 qosnat2 HTTPS 证书（/etc/qosnat2/tls.crt）
      </label>
      <div class="flex gap-2">
        <button type="button" class="btn-primary" @click="save">保存配置</button>
        <button type="button" class="btn-secondary" :disabled="!status?.installed" @click="apply">保存并应用</button>
      </div>
    </div>

    <div class="card p-4 mt-4">
      <h3 class="font-medium mb-2">VPN 用户</h3>
      <div class="grid grid-cols-3 gap-2 mb-3 text-sm">
        <input v-model="userForm.username" class="input" placeholder="用户名" />
        <input v-model="userForm.password" type="password" class="input" placeholder="密码" />
        <input v-model="userForm.comment" class="input" placeholder="备注" />
      </div>
      <button type="button" class="btn-secondary text-sm mb-3" @click="addUser">添加用户</button>
      <table class="w-full text-sm">
        <thead><tr class="text-left text-slate-500"><th>用户</th><th>备注</th><th></th></tr></thead>
        <tbody>
          <tr v-for="u in users" :key="u.username" class="border-t">
            <td class="py-2">{{ u.username }}</td>
            <td>{{ u.comment }}</td>
            <td><button type="button" class="text-red-600" @click="delUser(u.username)">删除</button></td>
          </tr>
        </tbody>
      </table>
      <p class="text-xs text-slate-500 mt-2">客户端：Cisco AnyConnect / openconnect CLI，连接 <code>https://&lt;WAN&gt;:{{ cfg?.tcp_port || 443 }}</code></p>
    </div>
  </div>
</template>
