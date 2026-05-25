<script setup>
import { computed, onMounted, ref } from 'vue'
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
const radiusSecret = ref('')

const isRadius = computed(() => cfg.value?.auth_method === 'radius')

function ensureDefaults() {
  if (!cfg.value.auth_method) cfg.value.auth_method = 'plain'
  if (!cfg.value.radius) {
    cfg.value.radius = {
      auth_port: 1812,
      acct_port: 1813,
      groupconfig: true,
      stats_report_time: 360,
    }
  }
}

async function load() {
  const d = await api.get('/api/v1/vpn/ocserv')
  cfg.value = d.config || {}
  ensureDefaults()
  status.value = d.status || {}
  installScript.value = d.install_script || ''
  users.value = (d.config?.users || []).map((u) => ({ ...u }))
  radiusSecret.value = ''
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
    const body = { ...cfg.value }
    if (isRadius.value) {
      body.users = []
      if (radiusSecret.value) {
        body.radius = { ...body.radius, secret: radiusSecret.value }
      }
    } else {
      body.users = users.value
    }
    await api.put('/api/v1/vpn/ocserv', body)
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
    <PageHeader title="OpenConnect (ocserv)" subtitle="AnyConnect 兼容 SSL VPN；需先源码安装 ocserv（含 RADIUS/radcli）" />
    <p v-if="err" class="text-sm text-red-600 mb-2">{{ err }}</p>
    <p v-if="ok" class="text-sm text-green-700 mb-2">{{ ok }}</p>

    <div class="card p-4 mb-4 space-y-2">
      <div class="flex flex-wrap gap-4 text-sm">
        <span>已安装: <strong>{{ status?.installed ? '是' : '否' }}</strong></span>
        <span>运行中: <strong>{{ status?.active ? '是' : '否' }}</strong></span>
        <span>RADIUS 支持: <strong>{{ status?.radius_linked ? '是' : '否' }}</strong></span>
        <span v-if="status?.version" class="text-slate-500">{{ status.version }}</span>
      </div>
      <p v-if="status?.installed && !status?.radius_linked" class="text-xs text-amber-700">
        当前 ocserv 未链接 radcli，无法使用 RADIUS。请重新运行源码安装脚本（需 libradcli-dev）。
      </p>
      <p class="text-xs text-slate-500">
        源码安装: <code class="bg-slate-100 px-1">{{ installScript }}</code>
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

      <div>
        <span class="text-sm font-medium">认证方式</span>
        <div class="flex gap-4 mt-2 text-sm">
          <label class="flex items-center gap-2">
            <input v-model="cfg.auth_method" type="radio" value="plain" />
            本地用户（ocpasswd）
          </label>
          <label class="flex items-center gap-2">
            <input v-model="cfg.auth_method" type="radio" value="radius" />
            RADIUS
          </label>
        </div>
      </div>

      <div v-if="isRadius" class="border border-slate-200 rounded-lg p-4 space-y-3 bg-slate-50/50">
        <h3 class="text-sm font-medium">RADIUS 服务器</h3>
        <div class="grid grid-cols-2 gap-4">
          <label class="block text-sm col-span-2">
            服务器地址
            <input v-model="cfg.radius.server" class="input w-full mt-1" placeholder="192.168.1.10 或 radius.example.com" />
          </label>
          <label class="block text-sm">
            认证端口
            <input v-model.number="cfg.radius.auth_port" type="number" class="input w-full mt-1" />
          </label>
          <label class="block text-sm">
            计费端口
            <input v-model.number="cfg.radius.acct_port" type="number" class="input w-full mt-1" />
          </label>
          <label class="block text-sm col-span-2">
            共享密钥
            <input v-model="radiusSecret" type="password" class="input w-full mt-1" placeholder="留空则保持已保存的密钥" />
          </label>
          <label class="block text-sm col-span-2">
            NAS-Identifier（可选）
            <input v-model="cfg.radius.nas_identifier" class="input w-full mt-1" placeholder="qosnat2" />
          </label>
        </div>
        <label class="flex items-center gap-2 text-sm">
          <input v-model="cfg.radius.groupconfig" type="checkbox" />
          groupconfig（从 RADIUS 下发 DNS/路由/地址等）
        </label>
        <label class="flex items-center gap-2 text-sm">
          <input v-model="cfg.radius.acct_enabled" type="checkbox" />
          启用 RADIUS 计费（acct）
        </label>
        <label v-if="cfg.radius.acct_enabled" class="block text-sm">
          计费上报间隔（秒）
          <input v-model.number="cfg.radius.stats_report_time" type="number" class="input w-full mt-1 max-w-xs" />
        </label>
        <p class="text-xs text-slate-500">
          配置写入 <code>/etc/radcli/</code>。FreeRADIUS 需在 acct_unique 中去掉 NAS-Port 依赖（ocserv 不发送该属性）。
        </p>
      </div>

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

    <div v-if="cfg && !isRadius" class="card p-4 mt-4">
      <h3 class="font-medium mb-2">本地 VPN 用户</h3>
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
    </div>

    <p v-if="cfg" class="text-xs text-slate-500 mt-4">
      客户端：Cisco AnyConnect / openconnect CLI，连接 <code>https://&lt;WAN&gt;:{{ cfg?.tcp_port || 443 }}</code>
    </p>
  </div>
</template>
