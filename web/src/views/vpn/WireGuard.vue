<script setup>
import { onMounted, ref } from 'vue'
import { api } from '@/api/client'

const cfg = ref(null)
const status = ref(null)
const peers = ref([])
const err = ref('')
const ok = ref('')
function defaultPeerForm() {
  return {
    name: '',
    allowed_ips: '10.200.0.10/32',
    private_key: '',
    public_key: '',
    endpoint: '',
    persistent_keepalive: 25,
    rate: { down: '8mbit', up: '8mbit' },
  }
}

const peerForm = ref(defaultPeerForm())
const serverEndpoint = ref('')

async function load() {
  const d = await api.get('/api/v1/vpn/wireguard')
  cfg.value = d.config
  status.value = d.status
  peers.value = (d.config?.peers || []).map((p) => ({
    ...p,
    rate: p.rate || { down: '', up: '' },
  }))
  serverEndpoint.value = d.config?.server_endpoint || ''
}

function ensurePeerRate(p) {
  if (!p.rate) p.rate = { down: '', up: '' }
}

async function genKeys() {
  err.value = ''
  ok.value = ''
  try {
    const kp = await api.post('/api/v1/vpn/wireguard/keys', {})
    if (!cfg.value) await load()
    cfg.value.private_key = kp.private_key
    cfg.value.public_key = kp.public_key
    ok.value = '已生成服务端密钥'
  } catch (e) {
    err.value = e.message
  }
}

async function save() {
  err.value = ''
  ok.value = ''
  try {
    cfg.value.server_endpoint = serverEndpoint.value
    cfg.value.peers = peers.value
    await api.put('/api/v1/vpn/wireguard', cfg.value)
    ok.value = '配置已保存'
    if (cfg.value.enabled) {
      try {
        await api.post('/api/v1/vpn/wireguard/apply', {})
        ok.value = '已保存并应用'
      } catch (e) {
        err.value = `保存成功，但 wg-quick 应用失败: ${e.message}`
      }
    } else {
      try {
        await api.post('/api/v1/vpn/wireguard/apply', {})
      } catch {
        /* down 时接口可能本就不存在 */
      }
      ok.value = '已保存（WireGuard 未启用）'
    }
    await load()
  } catch (e) {
    err.value = e.message
  }
}

async function genPeerKeys() {
  err.value = ''
  try {
    const kp = await api.post('/api/v1/vpn/wireguard/keys', {})
    peerForm.value.private_key = kp.private_key
    peerForm.value.public_key = kp.public_key
    ok.value = '已生成 Peer 密钥对（客户端私钥 + 公钥）'
  } catch (e) {
    err.value = e.message
  }
}

async function addPeer() {
  err.value = ''
  ok.value = ''
  try {
    if (!peerForm.value.name?.trim()) {
      err.value = '请填写 Peer 名称'
      return
    }
    const body = {
      name: peerForm.value.name.trim(),
      allowed_ips: [String(peerForm.value.allowed_ips || '').trim()].filter(Boolean),
      persistent_keepalive: peerForm.value.persistent_keepalive,
      rate: peerForm.value.rate,
    }
    const priv = String(peerForm.value.private_key || '').trim()
    const pub = String(peerForm.value.public_key || '').trim()
    if (priv) body.private_key = priv
    if (pub) body.public_key = pub
    if (peerForm.value.endpoint?.trim()) {
      body.endpoint = peerForm.value.endpoint.trim()
    }
    await api.post('/api/v1/vpn/wireguard/peers', body)
    peerForm.value = defaultPeerForm()
    await load()
    ok.value = 'Peer 已添加'
  } catch (e) {
    err.value = e.message
  }
}

async function delPeer(name) {
  err.value = ''
  try {
    await api.del(`/api/v1/vpn/wireguard/peers?name=${encodeURIComponent(name)}`)
    await load()
    ok.value = '已删除 Peer'
  } catch (e) {
    err.value = e.message
  }
}

function downloadConf(name) {
  window.open(`/api/v1/vpn/wireguard/peers/${encodeURIComponent(name)}/conf`, '_blank')
}

onMounted(load)
</script>

<template>
  <div>
    <h2 class="text-xl font-semibold mb-4">WireGuard</h2>
    <p v-if="ok" class="text-green-700 text-sm mb-2">{{ ok }}</p>
    <p v-if="err" class="text-red-600 text-sm mb-2">{{ err }}</p>
    <p class="text-sm text-slate-600 mb-4">
      Peer 限速按隧道 IP（AllowedIPs 中第一个 IPv4）写入 <code class="text-xs">host_exact</code>，在
      <code class="text-xs">wg0</code> + IFB 上 HTB 整形；下行/上行相对服务端视角。
    </p>

    <div v-if="cfg" class="space-y-6">
      <section class="card p-4">
        <h3 class="font-medium mb-3">服务端</h3>
        <div class="grid sm:grid-cols-2 gap-3 text-sm">
          <label class="flex items-center gap-2">
            <input v-model="cfg.enabled" type="checkbox" /> 启用
          </label>
          <div>
            <span class="text-slate-500">状态</span>
            {{ status?.installed ? (status?.up ? '运行中' : '已安装') : '未安装 wg' }}
          </div>
          <div>
            <label class="text-xs text-slate-500">接口</label>
            <input v-model="cfg.interface" class="input-field" />
          </div>
          <div>
            <label class="text-xs text-slate-500">监听端口</label>
            <input v-model.number="cfg.listen_port" type="number" class="input-field" />
          </div>
          <div class="sm:col-span-2">
            <label class="text-xs text-slate-500">隧道地址</label>
            <input v-model="cfg.address" class="input-field font-mono" />
          </div>
          <div class="sm:col-span-2">
            <label class="text-xs text-slate-500">客户端 Endpoint（公网 IP:端口）</label>
            <input v-model="serverEndpoint" class="input-field font-mono" placeholder="157.15.107.249:51820" />
          </div>
        </div>
        <div class="flex gap-2 mt-4">
          <button type="button" class="btn-secondary" @click="genKeys">生成服务端密钥</button>
          <button type="button" class="btn-primary" @click="save">保存并 wg-quick apply</button>
        </div>
        <p class="text-xs text-slate-400 mt-2 font-mono truncate">公钥: {{ cfg.public_key || '—' }}</p>
      </section>

      <section class="card p-4">
        <h3 class="font-medium mb-3">添加 Peer</h3>
        <p class="text-xs text-slate-500 mb-3">
          可手动填写客户端私钥/公钥，或点「自动生成密钥对」；仅填公钥可导入已有客户端（无法下载 conf）。
        </p>
        <div class="grid sm:grid-cols-2 gap-3 text-sm max-w-3xl">
          <div>
            <label class="text-xs text-slate-500">名称 *</label>
            <input v-model="peerForm.name" class="input-field" placeholder="client-1" />
          </div>
          <div>
            <label class="text-xs text-slate-500">隧道地址 (AllowedIPs)</label>
            <input v-model="peerForm.allowed_ips" class="input-field font-mono text-xs" placeholder="10.200.0.10/32" />
          </div>
          <div>
            <label class="text-xs text-slate-500">下行限速</label>
            <input v-model="peerForm.rate.down" class="input-field font-mono text-xs" placeholder="8mbit" />
          </div>
          <div>
            <label class="text-xs text-slate-500">上行限速</label>
            <input v-model="peerForm.rate.up" class="input-field font-mono text-xs" placeholder="8mbit" />
          </div>
          <div>
            <label class="text-xs text-slate-500">Keepalive (秒)</label>
            <input v-model.number="peerForm.persistent_keepalive" type="number" class="input-field" />
          </div>
          <div>
            <label class="text-xs text-slate-500">Endpoint（可选，客户端源地址）</label>
            <input v-model="peerForm.endpoint" class="input-field font-mono text-xs" placeholder="203.0.113.50:51820" />
          </div>
          <div class="sm:col-span-2">
            <label class="text-xs text-slate-500">客户端私钥 (PrivateKey)</label>
            <textarea
              v-model="peerForm.private_key"
              class="input-field font-mono text-xs min-h-[4rem]"
              placeholder="留空则添加时自动生成，或仅填公钥导入已有设备"
              spellcheck="false"
            />
          </div>
          <div class="sm:col-span-2">
            <label class="text-xs text-slate-500">客户端公钥 (PublicKey，服务端 Peer 必填)</label>
            <textarea
              v-model="peerForm.public_key"
              class="input-field font-mono text-xs min-h-[4rem]"
              placeholder="可只填公钥；若已填私钥可留空由服务端推导"
              spellcheck="false"
            />
          </div>
        </div>
        <div class="flex flex-wrap gap-2 mt-4">
          <button type="button" class="btn-secondary" @click="genPeerKeys">自动生成密钥对</button>
          <button type="button" class="btn-primary" @click="addPeer">添加 Peer</button>
        </div>
      </section>

      <section class="card table-wrap p-4">
        <table class="data w-full">
          <thead>
            <tr>
              <th>名称</th>
              <th>公钥</th>
              <th>AllowedIPs</th>
              <th>下行</th>
              <th>上行</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="p in peers" :key="p.name">
              <td>{{ p.name }}</td>
              <td class="font-mono text-xs max-w-xs truncate">{{ p.public_key }}</td>
              <td class="font-mono text-xs">{{ (p.allowed_ips || []).join(', ') }}</td>
              <td>
                <input
                  v-model="p.rate.down"
                  class="input-field w-20 text-xs font-mono"
                  placeholder="8mbit"
                  @focus="ensurePeerRate(p)"
                />
              </td>
              <td>
                <input
                  v-model="p.rate.up"
                  class="input-field w-20 text-xs font-mono"
                  placeholder="8mbit"
                  @focus="ensurePeerRate(p)"
                />
              </td>
              <td class="whitespace-nowrap">
                <button type="button" class="text-blue-600 text-xs mr-2" @click='downloadConf(p.name)'>下载 conf</button>
                <button type="button" class="text-red-600 text-xs" @click='delPeer(p.name)'>删除</button>
              </td>
            </tr>
          </tbody>
        </table>
      </section>
    </div>
  </div>
</template>
