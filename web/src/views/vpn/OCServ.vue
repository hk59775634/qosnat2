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
const camouflageSecret = ref('')
const dnsText = ref('')
const routesText = ref('')
const noRoutesText = ref('')

const isRadius = computed(() => cfg.value?.auth_method === 'radius')

const defaultAdvanced = () => ({
  try_mtu_discovery: true,
  isolate_workers: true,
  dtls_legacy: true,
  tcp: true,
  udp: true,
  deny_roaming: false,
  cisco_client_compat: true,
  cisco_svc_client_compat: false,
  client_bypass_protocol: false,
  compression: false,
  keepalive: true,
  dpd: true,
  mobile_dpd: true,
  predictable_ips: false,
  ping_leases: false,
  use_occtl: false,
  rekey: true,
  switch_to_tcp: true,
  camouflage: false,
  max_same_clients: 2,
  keepalive_sec: 32400,
  dpd_sec: 90,
  mobile_dpd_sec: 1800,
  cookie_timeout: 300,
  rekey_time: 172800,
  rekey_method: 'ssl',
  auth_timeout: 240,
  switch_to_tcp_timeout: 25,
  rate_limit_ms: 100,
  log_level: 2,
  max_ban_score: 80,
  ban_time: 300,
  ban_reset_time: 1200,
  server_stats_reset_time: 604800,
  rx_data_per_sec: 0,
  tx_data_per_sec: 0,
  camouflage_realm: '',
  default_domain: '',
  config_per_group: '',
  cert_user_oid: '0.9.2342.19200300.100.1.1',
  tls_priorities: 'NORMAL:%SERVER_PRECEDENCE:%COMPAT:-VERS-SSL3.0:-VERS-TLS1.0:-VERS-TLS1.1',
})

const featureToggles = [
  { key: 'tcp', label: 'TCP', hint: 'tcp-port' },
  { key: 'udp', label: 'UDP / DTLS', hint: 'udp-port' },
  { key: 'try_mtu_discovery', label: 'MTU 探测', hint: 'try-mtu-discovery' },
  { key: 'isolate_workers', label: '隔离 worker', hint: 'isolate-workers' },
  { key: 'dtls_legacy', label: 'DTLS 旧版兼容', hint: 'dtls-legacy' },
  { key: 'cisco_client_compat', label: 'Cisco 兼容', hint: 'cisco-client-compat' },
  { key: 'cisco_svc_client_compat', label: 'Cisco SVC 兼容', hint: 'cisco-svc-client-compat' },
  { key: 'client_bypass_protocol', label: '客户端绕过协议', hint: 'client-bypass-protocol' },
  { key: 'deny_roaming', label: '禁止漫游', hint: 'deny-roaming' },
  { key: 'compression', label: '压缩', hint: 'compression' },
  { key: 'keepalive', label: 'Keepalive', hint: 'keepalive' },
  { key: 'dpd', label: 'DPD', hint: 'dpd' },
  { key: 'mobile_dpd', label: '移动端 DPD', hint: 'mobile-dpd' },
  { key: 'switch_to_tcp', label: 'UDP 切 TCP', hint: 'switch-to-tcp-timeout' },
  { key: 'rekey', label: '会话重密钥', hint: 'rekey-time' },
  { key: 'predictable_ips', label: '可预测 IP', hint: 'predictable-ips' },
  { key: 'ping_leases', label: 'Ping 租约', hint: 'ping-leases' },
  { key: 'use_occtl', label: 'occtl', hint: 'use-occtl' },
  { key: 'camouflage', label: '伪装站点', hint: 'camouflage' },
]

const numericAdvanced = [
  { key: 'max_same_clients', label: '同用户最大会话', min: 1 },
  { key: 'rate_limit_ms', label: '连接限速间隔（ms）', min: 0 },
  { key: 'log_level', label: '日志级别', min: 0, max: 9 },
  { key: 'max_ban_score', label: '封禁分数阈值', min: 1 },
  { key: 'ban_time', label: '封禁时长（秒）', min: 1 },
  { key: 'ban_reset_time', label: '封禁分数重置（秒）', min: 1 },
  { key: 'server_stats_reset_time', label: '统计清零周期（秒）', min: 60 },
  { key: 'keepalive_sec', label: 'Keepalive（秒）', min: 60, show: () => cfg.value?.advanced?.keepalive },
  { key: 'dpd_sec', label: 'DPD（秒）', min: 10, show: () => cfg.value?.advanced?.dpd },
  { key: 'mobile_dpd_sec', label: '移动端 DPD（秒）', min: 60, show: () => cfg.value?.advanced?.mobile_dpd },
  { key: 'cookie_timeout', label: 'Cookie 超时（秒）', min: 60 },
  { key: 'auth_timeout', label: '认证超时（秒）', min: 30 },
  { key: 'rekey_time', label: 'Rekey 间隔（秒）', min: 3600, show: () => cfg.value?.advanced?.rekey },
  { key: 'switch_to_tcp_timeout', label: '切 TCP 超时（秒）', min: 5, show: () => cfg.value?.advanced?.switch_to_tcp },
  { key: 'rx_data_per_sec', label: '下行限速 B/s（0=不限）', min: 0 },
  { key: 'tx_data_per_sec', label: '上行限速 B/s（0=不限）', min: 0 },
]

function listToText(arr) {
  return (arr || []).join('\n')
}

function textToList(s) {
  return String(s || '')
    .split(/[\n,]+/)
    .map((x) => x.trim())
    .filter(Boolean)
}

function ensureDefaults() {
  if (!cfg.value.auth_method) cfg.value.auth_method = 'plain'
  if (!cfg.value.radius) {
    cfg.value.radius = { auth_port: 1812, acct_port: 1813, groupconfig: true, stats_report_time: 360 }
  }
  cfg.value.advanced = { ...defaultAdvanced(), ...(cfg.value.advanced || {}) }
  if (!cfg.value.server_cert_path) cfg.value.server_cert_path = '/etc/ocserv/certs/server-cert.pem'
  if (!cfg.value.server_key_path) cfg.value.server_key_path = '/etc/ocserv/certs/server-key.pem'
  if (!cfg.value.socket_file) cfg.value.socket_file = '/var/run/ocserv-socket'
  dnsText.value = listToText(cfg.value.dns)
  routesText.value = listToText(cfg.value.routes)
  noRoutesText.value = listToText(cfg.value.no_routes)
}

function buildBody() {
  const body = { ...cfg.value }
  body.dns = textToList(dnsText.value)
  body.routes = textToList(routesText.value)
  body.no_routes = textToList(noRoutesText.value)
  if (body.advanced?.camouflage && camouflageSecret.value) {
    body.advanced = { ...body.advanced, camouflage_secret: camouflageSecret.value }
  }
  return body
}

async function load() {
  const d = await api.get('/api/v1/vpn/ocserv')
  cfg.value = d.config || {}
  ensureDefaults()
  status.value = d.status || {}
  installScript.value = d.install_script || ''
  users.value = (d.config?.users || []).map((u) => ({ ...u }))
  radiusSecret.value = ''
  camouflageSecret.value = ''
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
    const body = buildBody()
    if (!body.advanced?.tcp && !body.advanced?.udp) {
      err.value = 'TCP 与 UDP 至少启用一项'
      return
    }
    if (isRadius.value) {
      body.users = []
      if (radiusSecret.value) body.radius = { ...body.radius, secret: radiusSecret.value }
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
    ok.value = '用户已添加'
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
    <PageHeader title="OpenConnect (ocserv)" subtitle="AnyConnect 兼容 SSL VPN" />
    <p v-if="err" class="text-sm text-red-600 mb-2">{{ err }}</p>
    <p v-if="ok" class="text-sm text-green-700 mb-2">{{ ok }}</p>

    <div class="card p-4 mb-4 space-y-2">
      <div class="flex flex-wrap gap-4 text-sm">
        <span>已安装: <strong>{{ status?.installed ? '是' : '否' }}</strong></span>
        <span>运行中: <strong>{{ status?.active ? '是' : '否' }}</strong></span>
        <span>RADIUS: <strong>{{ status?.radius_linked ? '是' : '否' }}</strong></span>
      </div>
      <button type="button" class="btn-secondary text-sm" :disabled="installing" @click="runInstall">
        {{ installing ? '安装中…' : '从源码安装' }}
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
          <label class="flex items-center gap-2"><input v-model="cfg.auth_method" type="radio" value="plain" /> 本地</label>
          <label class="flex items-center gap-2"><input v-model="cfg.auth_method" type="radio" value="radius" /> RADIUS</label>
        </div>
      </div>

      <div v-if="isRadius" class="border rounded-lg p-4 space-y-3 bg-slate-50/50">
        <h3 class="text-sm font-medium">RADIUS</h3>
        <label class="block text-sm">服务器 <input v-model="cfg.radius.server" class="input w-full mt-1" /></label>
        <div class="grid grid-cols-2 gap-4">
          <label class="text-sm">认证端口 <input v-model.number="cfg.radius.auth_port" type="number" class="input w-full mt-1" /></label>
          <label class="text-sm">计费端口 <input v-model.number="cfg.radius.acct_port" type="number" class="input w-full mt-1" /></label>
        </div>
        <label class="text-sm">共享密钥 <input v-model="radiusSecret" type="password" class="input w-full mt-1" placeholder="留空保持已保存" /></label>
        <label class="text-sm">NAS-Identifier <input v-model="cfg.radius.nas_identifier" class="input w-full mt-1" /></label>
        <label class="flex gap-2 text-sm"><input v-model="cfg.radius.groupconfig" type="checkbox" /> groupconfig</label>
        <label class="flex gap-2 text-sm"><input v-model="cfg.radius.acct_enabled" type="checkbox" /> RADIUS 计费</label>
        <label v-if="cfg.radius.acct_enabled" class="text-sm">stats-report-time（秒）
          <input v-model.number="cfg.radius.stats_report_time" type="number" class="input w-full mt-1 max-w-xs" />
        </label>
      </div>

      <div class="grid grid-cols-2 gap-4">
        <label class="text-sm">TCP 端口 <input v-model.number="cfg.tcp_port" type="number" class="input w-full mt-1" :disabled="!cfg.advanced?.tcp" /></label>
        <label class="text-sm">UDP 端口 <input v-model.number="cfg.udp_port" type="number" class="input w-full mt-1" :disabled="!cfg.advanced?.udp" /></label>
        <label class="text-sm">地址池 <input v-model="cfg.ipv4_network" class="input w-full mt-1" /></label>
        <label class="text-sm">掩码 <input v-model="cfg.ipv4_netmask" class="input w-full mt-1" /></label>
        <label class="text-sm">TUN 设备 <input v-model="cfg.device" class="input w-full mt-1" /></label>
        <label class="text-sm">最大客户端 <input v-model.number="cfg.max_clients" type="number" class="input w-full mt-1" /></label>
        <label class="text-sm col-span-2">Socket 文件 <input v-model="cfg.socket_file" class="input w-full mt-1" /></label>
      </div>

      <div class="grid grid-cols-1 sm:grid-cols-3 gap-4">
        <label class="text-sm">DNS（每行一个）<textarea v-model="dnsText" class="input w-full mt-1 font-mono text-xs" rows="3" /></label>
        <label class="text-sm">路由 route（每行一个）<textarea v-model="routesText" class="input w-full mt-1 font-mono text-xs" rows="3" placeholder="default" /></label>
        <label class="text-sm">排除 no-route<textarea v-model="noRoutesText" class="input w-full mt-1 font-mono text-xs" rows="3" placeholder="192.168.5.0/255.255.255.0" /></label>
      </div>

      <div class="border rounded-lg p-4 space-y-3">
        <h3 class="text-sm font-medium">TLS / 证书</h3>
        <label class="flex gap-2 text-sm"><input v-model="cfg.use_qosnat_tls" type="checkbox" /> 使用 qosnat2 TLS（复制到下方路径）</label>
        <div class="grid grid-cols-2 gap-4">
          <label class="text-sm">server-cert 路径 <input v-model="cfg.server_cert_path" class="input w-full mt-1" /></label>
          <label class="text-sm">server-key 路径 <input v-model="cfg.server_key_path" class="input w-full mt-1" /></label>
          <label class="text-sm col-span-2">ca-cert 路径（可选）<input v-model="cfg.ca_cert_path" class="input w-full mt-1" placeholder="留空不启用客户端证书" /></label>
        </div>
        <label class="text-sm">cert-user-oid <input v-model="cfg.advanced.cert_user_oid" class="input w-full mt-1 font-mono text-xs" /></label>
        <label class="text-sm">tls-priorities <input v-model="cfg.advanced.tls_priorities" class="input w-full mt-1 font-mono text-xs" /></label>
      </div>

      <div class="border rounded-lg">
        <button type="button" class="w-full flex justify-between px-4 py-3 text-sm font-medium" @click="showAdvanced = !showAdvanced">
          <span>高级配置</span>
          <span class="text-slate-400">{{ showAdvanced ? '收起' : '展开' }}</span>
        </button>
        <div v-show="showAdvanced" class="px-4 pb-4 space-y-4 border-t">
          <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3 pt-3">
            <label v-for="f in featureToggles" :key="f.key" class="flex gap-2 text-sm p-2 border rounded cursor-pointer">
              <input v-model="cfg.advanced[f.key]" type="checkbox" class="mt-0.5" />
              <span><span class="font-medium">{{ f.label }}</span><span class="block text-xs text-slate-500">{{ f.hint }}</span></span>
            </label>
          </div>
          <div v-if="cfg.advanced.camouflage" class="grid grid-cols-2 gap-4 p-3 bg-amber-50/50 rounded border border-amber-100">
            <label class="text-sm col-span-2">camouflage_secret <input v-model="camouflageSecret" type="password" class="input w-full mt-1" placeholder="留空保持已保存" /></label>
            <label class="text-sm col-span-2">camouflage_realm <input v-model="cfg.advanced.camouflage_realm" class="input w-full mt-1" placeholder="Restricted Content" /></label>
          </div>
          <div class="grid grid-cols-2 sm:grid-cols-3 gap-4">
            <label v-for="n in numericAdvanced" v-show="!n.show || n.show()" :key="n.key" class="text-sm">
              {{ n.label }}
              <input v-model.number="cfg.advanced[n.key]" type="number" class="input w-full mt-1" :min="n.min" />
            </label>
          </div>
          <div class="grid grid-cols-2 gap-4">
            <label class="text-sm">rekey-method
              <select v-model="cfg.advanced.rekey_method" class="input w-full mt-1">
                <option value="ssl">ssl</option>
                <option value="new-tunnel">new-tunnel</option>
              </select>
            </label>
            <label class="text-sm">default-domain <input v-model="cfg.advanced.default_domain" class="input w-full mt-1" /></label>
            <label class="text-sm col-span-2">config-per-group 目录 <input v-model="cfg.advanced.config_per_group" class="input w-full mt-1" placeholder="/etc/ocserv/config-per-group/" /></label>
          </div>
        </div>
      </div>

      <div class="flex gap-2">
        <button type="button" class="btn-primary" @click="save">保存</button>
        <button type="button" class="btn-secondary" :disabled="!status?.installed" @click="apply">保存并应用</button>
      </div>
    </div>

    <div v-if="cfg && !isRadius" class="card p-4 mt-4">
      <h3 class="font-medium mb-2">本地用户</h3>
      <div class="grid grid-cols-3 gap-2 mb-3 text-sm">
        <input v-model="userForm.username" class="input" placeholder="用户名" />
        <input v-model="userForm.password" type="password" class="input" placeholder="密码" />
        <input v-model="userForm.comment" class="input" placeholder="备注" />
      </div>
      <button type="button" class="btn-secondary text-sm mb-3" @click="addUser">添加</button>
      <table class="w-full text-sm">
        <tbody>
          <tr v-for="u in users" :key="u.username" class="border-t">
            <td class="py-2">{{ u.username }}</td>
            <td>{{ u.comment }}</td>
            <td><button type="button" class="text-red-600" @click="delUser(u.username)">删除</button></td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
