<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { api } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'

const list = ref([])
const interfaces = ref([])
const defaults = ref({})
const devLan = ref('')
const devWan = ref('')
const err = ref('')
const ok = ref('')

const form = ref({
  interface: '',
  ip_version: 'ipv4',
  proto: 'tcp',
  src_addr: '0.0.0.0/0',
  dst_addr: '',
  dst_port: 443,
  redirect_ip: '192.168.1.10',
  redirect_port: 443,
  comment: '',
})

const bindIface = computed({
  get: () => form.value.interface || defaults.value.interface || devWan.value,
  set: (v) => {
    form.value.interface = v
  },
})

const ifaceAddrs = computed(() => {
  const iface = interfaces.value.find((i) => i.name === bindIface.value)
  return iface?.addrs || []
})

const srcAddrPlaceholder = computed(() =>
  form.value.ip_version === 'ipv6' ? '::/0' : '0.0.0.0/0',
)

watch(
  () => form.value.ip_version,
  (ver) => {
    if (ver === 'ipv6') {
      if (form.value.src_addr === '0.0.0.0/0' || !form.value.src_addr) {
        form.value.src_addr = '::/0'
      }
    } else if (form.value.src_addr === '::/0') {
      form.value.src_addr = '0.0.0.0/0'
    }
  },
)

watch(bindIface, (name) => {
  const iface = interfaces.value.find((i) => i.name === name)
  if (iface?.addrs?.length && !form.value.dst_addr) {
    form.value.dst_addr = iface.addrs[0].split('/')[0]
  }
})

async function load() {
  const d = await api.wanForwards.list()
  list.value = d.forwards || []
  interfaces.value = d.interfaces || []
  defaults.value = d.defaults || {}
  devLan.value = d.dev_lan || ''
  devWan.value = d.dev_wan || ''
  if (!form.value.interface) {
    form.value.interface = defaults.value.interface || devWan.value
  }
  if (!form.value.dst_addr && defaults.value.dst_addr) {
    form.value.dst_addr = defaults.value.dst_addr
  }
}

async function add() {
  err.value = ''
  ok.value = ''
  try {
    const body = {
      interface: bindIface.value,
      ip_version: form.value.ip_version,
      proto: form.value.proto,
      src_addr: form.value.src_addr || srcAddrPlaceholder.value,
      dst_addr: form.value.dst_addr || undefined,
      dst_port: form.value.dst_port,
      redirect_ip: form.value.redirect_ip,
      redirect_port: form.value.redirect_port || form.value.dst_port,
      comment: form.value.comment,
    }
    await api.wanForwards.add(body)
    ok.value = '已添加并加载 nft 规则'
    await load()
  } catch (e) {
    err.value = e.message
  }
}

async function remove(f) {
  if (!confirm(`删除转发 ${f.comment || f.id}?`)) return
  err.value = ''
  try {
    await api.wanForwards.del(f.id)
    await load()
  } catch (e) {
    err.value = e.message
  }
}

function fmtRule(f) {
  const dst = f.dst_addr ? `${f.dst_addr}:` : ':'
  return `${f.interface || devWan.value} ${f.ip_version} ${f.proto} ${f.src_addr} → ${dst}${f.dst_port} ⇒ ${f.redirect_ip}:${f.redirect_port}`
}

onMounted(load)
</script>

<template>
  <div class="page-stack">
    <PageHeader
      title="端口转发"
      description="DNAT · 可选接口、协议版本、源/目标与重定向"
    />
    <p class="text-sm text-slate-600 mb-4 -mt-2">
      在指定接口上匹配入站流量并重定向到内网目标。默认绑定 WAN
      <span class="font-mono">{{ devWan }}</span>，源地址默认任意（0.0.0.0/0 或 ::/0）。
    </p>
    <p v-if="ok" class="text-green-700 text-sm mb-2">{{ ok }}</p>
    <p v-if="err" class="text-red-600 text-sm mb-2">{{ err }}</p>

    <div class="card card-body mb-0">
      <h3 class="font-medium mb-3">添加规则</h3>
      <div class="grid sm:grid-cols-2 lg:grid-cols-3 gap-3 text-sm">
        <div>
          <label class="text-xs text-slate-500">接口</label>
          <select v-model="bindIface" class="input-field font-mono">
            <option v-for="iface in interfaces" :key="iface.name" :value="iface.name">
              {{ iface.name }}
              <template v-if="iface.addrs?.length"> — {{ iface.addrs.join(', ') }}</template>
            </option>
          </select>
        </div>
        <div>
          <label class="text-xs text-slate-500">协议版本</label>
          <select v-model="form.ip_version" class="input-field">
            <option value="ipv4">IPv4</option>
            <option value="ipv6">IPv6</option>
          </select>
        </div>
        <div>
          <label class="text-xs text-slate-500">协议</label>
          <select v-model="form.proto" class="input-field">
            <option value="tcp">TCP</option>
            <option value="udp">UDP</option>
            <option value="tcp_udp">TCP + UDP</option>
          </select>
        </div>
        <div>
          <label class="text-xs text-slate-500">源地址 (CIDR)</label>
          <input
            v-model="form.src_addr"
            class="input-field font-mono"
            :placeholder="srcAddrPlaceholder"
          />
        </div>
        <div>
          <label class="text-xs text-slate-500">目标地址</label>
          <select v-model="form.dst_addr" class="input-field font-mono">
            <option value="">任意（不限制 daddr）</option>
            <option v-for="a in ifaceAddrs" :key="a" :value="a.split('/')[0]">
              {{ a }}
            </option>
          </select>
        </div>
        <div>
          <label class="text-xs text-slate-500">目标端口</label>
          <input v-model.number="form.dst_port" type="number" class="input-field" />
        </div>
        <div>
          <label class="text-xs text-slate-500">重定向目标 IP</label>
          <input v-model="form.redirect_ip" class="input-field font-mono" />
        </div>
        <div>
          <label class="text-xs text-slate-500">重定向目标端口</label>
          <input v-model.number="form.redirect_port" type="number" class="input-field" />
        </div>
        <div class="sm:col-span-2 lg:col-span-3">
          <label class="text-xs text-slate-500">描述</label>
          <input v-model="form.comment" class="input-field" placeholder="备注" />
        </div>
      </div>
      <button type="button" class="btn-primary mt-3" @click="add">添加</button>
    </div>

    <div class="card card-body table-wrap">
      <table class="data w-full text-sm">
        <thead>
          <tr>
            <th>接口</th>
            <th>版本</th>
            <th>协议</th>
            <th>匹配</th>
            <th>重定向</th>
            <th>描述</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="f in list" :key="f.id">
            <td class="font-mono text-xs">{{ f.interface || devWan }}</td>
            <td>{{ f.ip_version }}</td>
            <td>{{ f.proto }}</td>
            <td class="font-mono text-xs max-w-xs truncate" :title="fmtRule(f)">
              {{ f.src_addr }} → {{ f.dst_addr || '*' }}:{{ f.dst_port }}
            </td>
            <td class="font-mono text-xs">{{ f.redirect_ip }}:{{ f.redirect_port }}</td>
            <td>{{ f.comment }}</td>
            <td>
              <button type="button" class="text-red-600 text-xs" @click="remove(f)">删除</button>
            </td>
          </tr>
          <tr v-if="!list.length">
            <td colspan="7" class="text-center text-slate-400 py-3">暂无规则</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
