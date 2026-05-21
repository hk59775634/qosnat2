<script setup>
import { onMounted, ref } from 'vue'
import { api } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'

const rules = ref([])
const devLan = ref('')
const devWan = ref('')
const showRendered = ref(false)
const rendered = ref('')
const err = ref('')
const ok = ref('')
const form = ref({
  chain: 'forward',
  action: 'drop',
  iif: '',
  oif: '',
  proto: '',
  src_addr: '',
  dst_addr: '',
  src_alias: '',
  dst_alias: '',
  dst_port: 0,
  comment: '',
  enabled: true,
})

async function load() {
  const d = await api.firewall.rules.list()
  rules.value = d.rules || []
  devLan.value = d.dev_lan || ''
  devWan.value = d.dev_wan || ''
  rendered.value = d.rendered || ''
}

async function add() {
  err.value = ''
  try {
    await api.firewall.rules.add({ ...form.value })
    ok.value = '规则已添加并应用 nft'
    await load()
  } catch (e) {
    err.value = e.message
  }
}

async function remove(id) {
  if (!confirm('删除规则？')) return
  await api.firewall.rules.del(id)
  await load()
}

onMounted(load)
</script>

<template>
  <div class="page-stack">
    <PageHeader
      title="防火墙规则"
      description="自定义 forward/input 过滤规则（插入 established 之后）。复杂场景请直接编辑生成的 nft 文件。"
    />
    <p v-if="ok" class="text-green-700 text-sm mb-2">{{ ok }}</p>
    <p v-if="err" class="text-red-600 text-sm mb-2">{{ err }}</p>

    <div class="card card-body mb-0 space-y-3 text-sm">
      <h3 class="font-medium">添加规则</h3>
      <div class="grid sm:grid-cols-2 gap-3">
        <div>
          <label class="text-xs text-slate-500">链</label>
          <select v-model="form.chain" class="input-field mt-1">
            <option value="forward">forward</option>
            <option value="input">input</option>
          </select>
        </div>
        <div>
          <label class="text-xs text-slate-500">动作</label>
          <select v-model="form.action" class="input-field mt-1">
            <option value="accept">accept</option>
            <option value="drop">drop</option>
            <option value="reject">reject</option>
          </select>
        </div>
        <div>
          <label class="text-xs text-slate-500">入接口 iif</label>
          <input v-model="form.iif" class="input-field mt-1 font-mono" :placeholder="devWan" />
        </div>
        <div>
          <label class="text-xs text-slate-500">出接口 oif</label>
          <input v-model="form.oif" class="input-field mt-1 font-mono" :placeholder="devLan" />
        </div>
        <div>
          <label class="text-xs text-slate-500">协议</label>
          <input v-model="form.proto" class="input-field mt-1" placeholder="tcp / udp" />
        </div>
        <div>
          <label class="text-xs text-slate-500">目标端口</label>
          <input v-model.number="form.dst_port" type="number" class="input-field mt-1" />
        </div>
        <div>
          <label class="text-xs text-slate-500">源 Alias</label>
          <input v-model="form.src_alias" class="input-field mt-1 font-mono" placeholder="lan_hosts" />
        </div>
        <div class="sm:col-span-2">
          <label class="text-xs text-slate-500">源地址 CIDR（与 alias 二选一）</label>
          <input v-model="form.src_addr" class="input-field mt-1 font-mono" />
        </div>
        <div class="sm:col-span-2">
          <label class="text-xs text-slate-500">目标地址 CIDR</label>
          <input v-model="form.dst_addr" class="input-field mt-1 font-mono" />
        </div>
      </div>
      <label class="flex items-center gap-2">
        <input v-model="form.enabled" type="checkbox" /> 启用
      </label>
      <button type="button" class="btn-primary" @click="add">添加并应用</button>
    </div>

    <div class="card overflow-x-auto mb-4">
      <table class="data w-full text-sm">
        <thead>
          <tr>
            <th>链</th><th>动作</th><th>匹配</th><th>备注</th><th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="r in rules" :key="r.id">
            <td>{{ r.chain }}</td>
            <td>{{ r.action }}</td>
            <td class="font-mono text-xs">
              {{ [r.iif, r.oif, r.proto, r.src_addr, r.dst_addr, r.dst_port || ''].filter(Boolean).join(' ') }}
            </td>
            <td class="text-xs">{{ r.comment || r.id }}</td>
            <td>
              <button type="button" class="text-red-600 text-xs" @click="remove(r.id)">删除</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <button type="button" class="text-sm text-slate-600" @click="showRendered = !showRendered">
      {{ showRendered ? '隐藏' : '显示' }} nft 生成预览
    </button>
    <pre v-if="showRendered" class="mt-2 text-xs bg-slate-50 p-3 rounded overflow-auto max-h-96">{{ rendered }}</pre>
  </div>
</template>
