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
const showAdvanced = ref(false)

const userForm = ref({ username: '', password: '', comment: '' })
const radiusSecret = ref('')

const isRadius = computed(() => cfg.value?.auth_method === 'radius')

const defaultAdvanced = () => ({
  try_mtu_discovery: true,
  isolate_workers: true,
  dtls_legacy: true,
  tcp: true,
  udp: true,
  deny_roaming: false,
  cisco_client_compat: true,
  compression: false,
  keepalive: true,
  dpd: true,
  mobile_dpd: true,
  predictable_ips: false,
  ping_leases: false,
  use_occtl: false,
  rekey: true,
  switch_to_tcp: true,
  max_same_clients: 2,
  keepalive_sec: 32400,
  dpd_sec: 90,
  mobile_dpd_sec: 1800,
  cookie_timeout: 300,
  rekey_time: 172800,
  auth_timeout: 240,
  switch_to_tcp_timeout: 25,
})

const featureToggles = [
  { key: 'tcp', label: 'TCP', hint: '监听 tcp-port，主通道' },
  { key: 'udp', label: 'UDP / DTLS', hint: '监听 udp-port，低延迟' },
  { key: 'try_mtu_discovery', label: 'MTU 探测', hint: 'try-mtu-discovery' },
  { key: 'isolate_workers', label: '隔离 worker', hint: 'isolate-workers，提升安全' },
  { key: 'dtls_legacy', label: 'DTLS 旧版兼容', hint: 'AnyConnect 老客户端' },
  { key: 'cisco_client_compat', label: 'Cisco 客户端兼容', hint: 'cisco-client-compat' },
  { key: 'deny_roaming', label: '禁止漫游', hint: 'deny-roaming' },
  { key: 'compression', label: '压缩', hint: 'compression（通常关闭）' },
  { key: 'keepalive', label: 'Keepalive', hint: '长连接保活' },
  { key: 'dpd', label: 'DPD', hint: '断线检测' },
  { key: 'mobile_dpd', label: '移动端 DPD', hint: 'mobile-dpd' },
  { key: 'switch_to_tcp', label: 'UDP 切 TCP', hint: 'switch-to-tcp-timeout' },
  { key: 'rekey', label: '会话重密钥', hint: 'rekey-time / ssl' },
  { key: 'predictable_ips', label: '可预测 IP', hint: 'predictable-ips' },
  { key: 'ping_leases', label: 'Ping 租约', hint: 'ping-leases' },
  { key: 'use_occtl', label: 'occtl 控制', hint: 'use-occtl（本机管理）' },
]

const numericAdvanced = [
  { key: 'max_same_clients', label: '同用户最大会话', min: 1, max: 64 },
  { key: 'keepalive_sec', label: 'Keepalive（秒）', min: 60, show: () => cfg.value?.advanced?.keepalive },
  { key: 'dpd_sec', label: 'DPD（秒）', min: 10, show: () => cfg.value?.advanced?.dpd },
  { key: 'mobile_dpd_sec', label: '移动端 DPD（秒）', min: 60, show: () => cfg.value?.advanced?.mobile_dpd },
  { key: 'cookie_timeout', label: 'Cookie 超时（秒）', min: 60 },
  { key: 'auth_timeout', label: '认证超时（秒）', min: 30 },
  { key: 'rekey_time', label: 'Rekey 间隔（秒）', min: 3600, show: () => cfg.value?.advanced?.rekey },
  { key: 'switch_to_tcp_timeout', label: '切 TCP 超时（秒）', min: 5, show: () => cfg.value?.advanced?.switch_to_tcp },
]

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
  if (!cfg.value.advanced || Object.keys(cfg.value.advanced).length === 0) {
    cfg.value.advanced = defaultAdvanced()
  } else {
    cfg.value.advanced = { ...defaultAdvanced(), ...cfg.value.advanced }
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
    if (!body.advanced?.tcp && !body.advanced?.udp) {
      err.value = 'TCP 与 UDP 至少启用一项'
      return
    }
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
    if (err.value) return
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
            <input v-model="cfg.radius.server" class="input w-full mt-1" placeholder="192.168.1.10" />
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
            <input v-model="cfg.radius.nas_identifier" class="input w-full mt-1" />
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
      </div>

      <div class="grid grid-cols-2 gap-4">
        <label class="block text-sm">TCP 端口 <input v-model.number="cfg.tcp_port" type="number" class="input w-full mt-1" :disabled="!cfg.advanced?.tcp" /></label>
        <label class="block text-sm">UDP 端口 <input v-model.number="cfg.udp_port" type="number" class="input w-full mt-1" :disabled="!cfg.advanced?.udp" /></label>
        <label class="block text-sm">地址池网络 <input v-model="cfg.ipv4_network" class="input w-full mt-1" /></label>
        <label class="block text-sm">掩码 <input v-model="cfg.ipv4_netmask" class="input w-full mt-1" /></label>
        <label class="block text-sm col-span-2">TUN 设备名 <input v-model="cfg.device" class="input w-full mt-1" placeholder="vpns" /></label>
        <label class="block text-sm">最大客户端 <input v-model.number="cfg.max_clients" type="number" class="input w-full mt-1" /></label>
      </div>
      <label class="flex items-center gap-2 text-sm">
        <input v-model="cfg.use_qosnat_tls" type="checkbox" />
        使用 qosnat2 HTTPS 证书（/etc/qosnat2/tls.crt）
      </label>

      <div class="border border-slate-200 rounded-lg">
        <button
          type="button"
          class="w-full flex items-center justify-between px-4 py-3 text-sm font-medium text-left hover:bg-slate-50"
          @click="showAdvanced = !showAdvanced"
        >
          <span>高级配置</span>
          <span class="text-slate-400">{{ showAdvanced ? '收起' : '展开' }}</span>
        </button>
        <div v-show="showAdvanced && cfg.advanced" class="px-4 pb-4 space-y-4 border-t border-slate-100">
          <p class="text-xs text-slate-500 pt-3">开关对应 ocserv.conf 中的功能项；关闭后写入 <code>= false</code> 或省略端口行。</p>
          <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
            <label
              v-for="f in featureToggles"
              :key="f.key"
              class="flex items-start gap-2 text-sm p-2 rounded border border-slate-100 hover:bg-slate-50/80 cursor-pointer"
            >
              <input v-model="cfg.advanced[f.key]" type="checkbox" class="mt-0.5" />
              <span>
                <span class="font-medium">{{ f.label }}</span>
                <span class="block text-xs text-slate-500">{{ f.hint }}</span>
              </span>
            </label>
          </div>
          <div class="grid grid-cols-2 sm:grid-cols-3 gap-4">
            <label
              v-for="n in numericAdvanced"
              v-show="!n.show || n.show()"
              :key="n.key"
              class="block text-sm"
            >
              {{ n.label }}
              <input v-model.number="cfg.advanced[n.key]" type="number" class="input w-full mt-1" :min="n.min" />
            </label>
          </div>
        </div>
      </div>

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
      客户端：Cisco AnyConnect / openconnect，<code>https://&lt;WAN&gt;:{{ cfg?.tcp_port || 443 }}</code>
    </p>
  </div>
</template>
