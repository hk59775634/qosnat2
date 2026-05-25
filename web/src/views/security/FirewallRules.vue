<script setup>
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/api/client'

const { t } = useI18n()
import PageHeader from '@/components/PageHeader.vue'

const rules = ref([])
const devLan = ref('')
const devWan = ref('')
const showRendered = ref(false)
const rendered = ref('')
const err = ref('')
const ok = ref('')
const dragIdx = ref(null)
const savingOrder = ref(false)
const editing = ref(null)
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

const emptyForm = () => ({
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
    form.value = emptyForm()
    await load()
  } catch (e) {
    err.value = e.message
  }
}

function startEdit(r) {
  editing.value = r.id
  form.value = {
    chain: r.chain,
    action: r.action,
    iif: r.iif || '',
    oif: r.oif || '',
    proto: r.proto || '',
    src_addr: r.src_addr || '',
    dst_addr: r.dst_addr || '',
    src_alias: r.src_alias || '',
    dst_alias: r.dst_alias || '',
    dst_port: r.dst_port || 0,
    comment: r.comment || '',
    enabled: r.enabled !== false,
  }
}

function cancelEdit() {
  editing.value = null
  form.value = emptyForm()
}

async function saveEdit() {
  if (!editing.value) return
  err.value = ''
  try {
    await api.firewall.rules.put(editing.value, { ...form.value, id: editing.value })
    ok.value = '规则已更新'
    cancelEdit()
    await load()
  } catch (e) {
    err.value = e.message
  }
}

async function remove(id) {
  if (!confirm(t('security.firewall.confirmDelete'))) return
  await api.firewall.rules.del(id)
  if (editing.value === id) cancelEdit()
  await load()
}

function onDragStart(idx) {
  dragIdx.value = idx
}

function onDragOver(e) {
  e.preventDefault()
}

async function onDrop(targetIdx) {
  if (dragIdx.value === null || dragIdx.value === targetIdx) {
    dragIdx.value = null
    return
  }
  const arr = [...rules.value]
  const [item] = arr.splice(dragIdx.value, 1)
  arr.splice(targetIdx, 0, item)
  dragIdx.value = null
  savingOrder.value = true
  err.value = ''
  try {
    const res = await api.firewall.rules.reorder(arr.map((r) => r.id))
    rules.value = res.rules || arr
    ok.value = '规则顺序已保存'
  } catch (e) {
    err.value = e.message
    await load()
  } finally {
    savingOrder.value = false
  }
}

async function moveRule(idx, dir) {
  const j = idx + dir
  if (j < 0 || j >= rules.value.length) return
  const arr = [...rules.value]
  ;[arr[idx], arr[j]] = [arr[j], arr[idx]]
  savingOrder.value = true
  try {
    const res = await api.firewall.rules.reorder(arr.map((r) => r.id))
    rules.value = res.rules || arr
    ok.value = '顺序已更新'
  } catch (e) {
    err.value = e.message
  } finally {
    savingOrder.value = false
  }
}

onMounted(load)
</script>

<template>
  <div class="page-stack">
    <PageHeader
      :title="t('security.firewall.title')"
      :description="t('security.firewall.description')"
    />
    <p v-if="ok" class="text-green-700 text-sm mb-2">{{ ok }}</p>
    <p v-if="err" class="text-red-600 text-sm mb-2">{{ err }}</p>
    <p v-if="savingOrder" class="text-xs text-slate-500">{{ t('security.firewall.savingOrder') }}</p>

    <div class="card card-body mb-0 space-y-3 text-sm">
      <h3 class="font-medium">{{ editing ? t('security.firewall.editRule') : t('security.firewall.addRule') }}</h3>
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
        <div class="sm:col-span-2">
          <label class="text-xs text-slate-500">备注</label>
          <input v-model="form.comment" class="input-field mt-1" />
        </div>
      </div>
      <label class="flex items-center gap-2">
        <input v-model="form.enabled" type="checkbox" /> 启用
      </label>
      <div class="flex flex-wrap gap-2">
        <button v-if="!editing" type="button" class="btn-primary" @click="add">添加并应用</button>
        <template v-else>
          <button type="button" class="btn-primary" @click="saveEdit">保存修改</button>
          <button type="button" class="btn-secondary" @click="cancelEdit">取消</button>
        </template>
      </div>
    </div>

    <div class="card overflow-x-auto mb-4">
      <table class="data w-full text-sm">
        <thead>
          <tr>
            <th class="w-7"></th>
            <th>链</th>
            <th>动作</th>
            <th>匹配</th>
            <th>备注</th>
            <th class="w-32"></th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="(r, idx) in rules"
            :key="r.id"
            draggable="true"
            class="cursor-grab active:cursor-grabbing hover:bg-slate-50"
            :class="{ 'opacity-50': dragIdx === idx, 'bg-blue-50': editing === r.id }"
            @dragstart="onDragStart(idx)"
            @dragover="onDragOver"
            @drop="onDrop(idx)"
          >
            <td class="text-slate-400 text-center select-none text-xs" title="拖动排序">⋮⋮</td>
            <td>{{ r.chain }}</td>
            <td>{{ r.action }}</td>
            <td class="font-mono text-xs">
              {{ [r.iif, r.oif, r.proto, r.src_addr, r.dst_addr, r.dst_port || ''].filter(Boolean).join(' ') }}
            </td>
            <td class="text-xs">{{ r.comment || r.id }}</td>
            <td class="text-right whitespace-nowrap space-x-1">
              <button type="button" class="text-xs text-slate-600" title="上移" @click="moveRule(idx, -1)">↑</button>
              <button type="button" class="text-xs text-slate-600" title="下移" @click="moveRule(idx, 1)">↓</button>
              <button type="button" class="text-xs text-blue-600" @click="startEdit(r)">编辑</button>
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
