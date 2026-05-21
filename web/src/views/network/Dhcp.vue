<script setup>
import { computed, onMounted, ref } from 'vue'
import { api } from '@/api/client'

const cfg = ref(null)
const status = ref(null)
const interfaces = ref([])
const devLan = ref('')
const devWan = ref('')
const rendered = ref('')
const err = ref('')
const ok = ref('')
const dnsText = ref('')
const staticForm = ref({ mac: '', ip: '', hostname: '', comment: '' })

const bindIface = computed({
  get: () => cfg.value?.interface || devLan.value || '',
  set: (v) => {
    if (cfg.value) cfg.value.interface = v
  },
})

async function load() {
  const d = await api.get('/api/v1/dhcp')
  cfg.value = d.config || {}
  status.value = d.status
  interfaces.value = d.interfaces || []
  devLan.value = d.dev_lan || ''
  devWan.value = d.dev_wan || ''
  rendered.value = d.rendered || ''
  dnsText.value = (cfg.value.dns_servers || []).join('\n')
  if (!cfg.value.interface && devLan.value) {
    cfg.value.interface = devLan.value
  }
}

async function save(applyAfter) {
  err.value = ''
  ok.value = ''
  try {
    cfg.value.dns_servers = dnsText.value
      .split(/[\n,]+/)
      .map((s) => s.trim())
      .filter(Boolean)
    await api.put('/api/v1/dhcp', cfg.value)
    ok.value = '配置已保存'
    if (applyAfter) {
      await api.post('/api/v1/dhcp/apply', {})
      ok.value = cfg.value.enabled ? '已保存并启动 dnsmasq' : '已保存并停止 DHCP'
    }
    await load()
  } catch (e) {
    err.value = e.message
  }
}

function addStatic() {
  if (!staticForm.value.mac || !staticForm.value.ip) {
    err.value = '静态绑定需填写 MAC 与 IP'
    return
  }
  if (!cfg.value.static_leases) cfg.value.static_leases = []
  cfg.value.static_leases.push({ ...staticForm.value })
  staticForm.value = { mac: '', ip: '', hostname: '', comment: '' }
  err.value = ''
}

function removeStatic(i) {
  cfg.value.static_leases.splice(i, 1)
}

function leaseLines() {
  const raw = status.value?.leases_raw || ''
  if (!raw.trim()) return []
  return raw.trim().split('\n').filter(Boolean)
}

onMounted(load)
</script>

<template>
  <div class="page-stack">
    <h2 class="text-xl font-semibold mb-4">DHCP 服务</h2>
    <p class="text-sm text-slate-600 mb-4">
      通过 <strong>dnsmasq</strong> 在指定网卡提供 DHCP，通常绑定内网口
      <span class="font-mono">{{ devLan }}</span>，并排除 WAN
      <span class="font-mono">{{ devWan }}</span>。
      请确保该网卡已配置静态 IP，且 LAN 上无其它 DHCP 服务冲突。
    </p>
    <p v-if="ok" class="text-green-700 text-sm mb-2">{{ ok }}</p>
    <p v-if="err" class="text-red-600 text-sm mb-2">{{ err }}</p>

    <div v-if="cfg" class="card card-body mb-0 space-y-3">
      <label class="flex items-center gap-2 text-sm font-medium">
        <input v-model="cfg.enabled" type="checkbox" /> 启用 DHCP
      </label>

      <div class="grid sm:grid-cols-2 gap-3 text-sm">
        <div>
          <label class="text-xs text-slate-500">绑定网卡（监听接口）</label>
          <select v-model="bindIface" class="input-field font-mono">
            <option v-for="iface in interfaces" :key="iface.name" :value="iface.name">
              {{ iface.name }}
              <template v-if="iface.addrs?.length"> — {{ iface.addrs.join(', ') }}</template>
              {{ iface.up ? '' : ' (down)' }}
            </option>
          </select>
          <p class="text-xs text-slate-400 mt-1">留空时默认使用 DEV_LAN（{{ devLan }}）</p>
        </div>
        <div>
          <label class="text-xs text-slate-500">默认网关 (option 3)</label>
          <input v-model="cfg.router" class="input-field font-mono" placeholder="192.168.1.1" />
        </div>
        <div>
          <label class="text-xs text-slate-500">地址池起始</label>
          <input v-model="cfg.range_start" class="input-field font-mono" />
        </div>
        <div>
          <label class="text-xs text-slate-500">地址池结束</label>
          <input v-model="cfg.range_end" class="input-field font-mono" />
        </div>
        <div>
          <label class="text-xs text-slate-500">子网掩码</label>
          <input v-model="cfg.netmask" class="input-field font-mono" placeholder="255.255.255.0" />
        </div>
        <div>
          <label class="text-xs text-slate-500">租约时间（秒）</label>
          <input v-model.number="cfg.lease_time_sec" type="number" class="input-field" />
        </div>
        <div class="sm:col-span-2">
          <label class="text-xs text-slate-500">DNS 服务器（每行一个或逗号分隔）</label>
          <textarea v-model="dnsText" class="input-field font-mono h-16" rows="2" />
        </div>
        <div>
          <label class="text-xs text-slate-500">域名（可选）</label>
          <input v-model="cfg.domain" class="input-field" />
        </div>
        <label class="flex items-center gap-2 sm:col-span-2 text-sm">
          <input v-model="cfg.authoritative" type="checkbox" /> authoritative（本网段唯一 DHCP）
        </label>
      </div>

      <div class="border-t border-slate-200 pt-4 space-y-3">
        <h3 class="font-medium text-sm">IPv6 / RA（dnsmasq）</h3>
        <label class="flex items-center gap-2 text-sm">
          <input v-model="cfg.ipv6_enabled" type="checkbox" /> 启用 DHCPv6 地址池
        </label>
        <div v-if="cfg.ipv6_enabled" class="grid sm:grid-cols-2 gap-3 text-sm">
          <div class="sm:col-span-2">
            <label class="text-xs text-slate-500">前缀（如 2001:db8::/64）</label>
            <input v-model="cfg.ipv6_prefix" class="input-field mt-1 font-mono" />
          </div>
          <div>
            <label class="text-xs text-slate-500">起始</label>
            <input v-model="cfg.ipv6_start" class="input-field mt-1 font-mono" placeholder="2001:db8::100" />
          </div>
          <div>
            <label class="text-xs text-slate-500">结束</label>
            <input v-model="cfg.ipv6_end" class="input-field mt-1 font-mono" placeholder="2001:db8::200" />
          </div>
        </div>
        <label class="flex items-center gap-2 text-sm">
          <input v-model="cfg.ra_enabled" type="checkbox" /> 路由器通告 (RA)
        </label>
        <div v-if="cfg.ra_enabled" class="text-sm max-w-xs">
          <label class="text-xs text-slate-500">RA 间隔（秒，可选）</label>
          <input v-model.number="cfg.ra_interval_sec" type="number" class="input-field mt-1" />
        </div>
      </div>

      <div class="flex flex-wrap gap-2">
        <button type="button" class="btn-primary" @click="save(true)">保存并应用</button>
        <button type="button" class="btn-secondary" @click="save(false)">仅保存</button>
      </div>

      <p class="text-xs text-slate-500">
        dnsmasq:
        <span :class="status?.active ? 'text-green-700' : 'text-slate-500'">
          {{ status?.installed ? (status.active ? '运行中' : '已停止') : '未安装' }}
        </span>
        <span v-if="status?.config"> · {{ status.config }}</span>
      </p>
    </div>

    <div v-if="cfg" class="grid lg:grid-cols-2 gap-3">
      <section class="card p-4">
        <h3 class="font-medium mb-3">静态租约</h3>
        <div class="grid sm:grid-cols-2 gap-2 text-sm mb-3">
          <input v-model="staticForm.mac" class="input-field font-mono text-xs" placeholder="aa:bb:cc:dd:ee:ff" />
          <input v-model="staticForm.ip" class="input-field font-mono text-xs" placeholder="192.168.1.50" />
          <input v-model="staticForm.hostname" class="input-field text-xs" placeholder="hostname" />
          <button type="button" class="btn-secondary text-xs" @click="addStatic">添加</button>
        </div>
        <table class="data w-full text-xs">
          <thead>
            <tr><th>MAC</th><th>IP</th><th></th></tr>
          </thead>
          <tbody>
            <tr v-for="(sl, i) in cfg.static_leases" :key="i">
              <td class="font-mono">{{ sl.mac }}</td>
              <td class="font-mono">{{ sl.ip }}</td>
              <td><button type="button" class="text-red-600" @click="removeStatic(i)">删除</button></td>
            </tr>
            <tr v-if="!cfg.static_leases?.length">
              <td colspan="3" class="text-center text-slate-400 py-3">无</td>
            </tr>
          </tbody>
        </table>
      </section>

      <section class="card p-4">
        <h3 class="font-medium mb-3">当前租约（dnsmasq.leases）</h3>
        <pre class="text-xs font-mono bg-slate-50 p-2 rounded overflow-auto max-h-48">{{ leaseLines().join('\n') || '（无）' }}</pre>
        <h3 class="font-medium mt-4 mb-2 text-sm">生成的配置预览</h3>
        <pre class="text-xs font-mono bg-slate-50 p-2 rounded overflow-auto max-h-40">{{ rendered || '# 启用后显示' }}</pre>
      </section>
    </div>
  </div>
</template>
