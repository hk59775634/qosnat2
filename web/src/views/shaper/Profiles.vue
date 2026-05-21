<script setup>
import { onMounted, ref } from 'vue'
import { api, bpsLabel } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'
import ShaperBindBar from '@/views/shaper/ShaperBindBar.vue'
import ProfileIfaceSelect from '@/views/shaper/ProfileIfaceSelect.vue'

const profiles = ref([])
const interfaces = ref([])
const bindDevice = ref('')
const devLan = ref('')
const devWan = ref('')
const attached = ref([])
const form = ref({ cidr: '10.0.0.0/16', down: '8mbit', up: '8mbit', mask: 32, device: '' })
const err = ref('')
const ok = ref('')
const dragIdx = ref(null)
const savingOrder = ref(false)
const tcLeaf = ref('fq_codel')
const tcFlows = ref(0)
const tcQuantum = ref(0)
const tcSaving = ref(false)

async function load() {
  const d = await api.shaper.profiles()
  profiles.value = d.profiles || []
  interfaces.value = d.interfaces || []
  bindDevice.value = d.bind_device || d.dev_lan || ''
  devLan.value = d.dev_lan || ''
  devWan.value = d.dev_wan || ''
  attached.value = d.attached_devices || []
  tcLeaf.value = d.leaf || 'fq_codel'
  tcFlows.value = d.fq_flows || 0
  tcQuantum.value = d.fq_quantum || 0
}

async function saveTC() {
  tcSaving.value = true
  err.value = ''
  try {
    await api.shaper.tc.put({
      leaf: tcLeaf.value,
      fq_flows: tcFlows.value,
      fq_quantum: tcQuantum.value,
      apply: true,
    })
    ok.value = 'TC 叶子 qdisc 已应用'
    await load()
  } catch (e) {
    err.value = e.message
  } finally {
    tcSaving.value = false
  }
}

async function submit() {
  err.value = ''
  ok.value = ''
  try {
    const res = await api.shaper.wizard(form.value)
    ok.value = res.added
      ? `已添加模板 ${res.cidr}`
      : `已更新模板 ${res.cidr}`
    await load()
  } catch (e) {
    err.value = e.message
  }
}

async function remove(cidr) {
  if (!confirm(`删除模板 ${cidr}?`)) return
  await api.shaper.delProfile(cidr)
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
  const arr = [...profiles.value]
  const [item] = arr.splice(dragIdx.value, 1)
  arr.splice(targetIdx, 0, item)
  dragIdx.value = null
  savingOrder.value = true
  err.value = ''
  try {
    const res = await api.shaper.reorderProfiles(arr.map((p) => p.cidr))
    profiles.value = res.profiles || arr
    bindDevice.value = res.bind_device || bindDevice.value
    ok.value = '排序已保存（id 已更新）'
  } catch (e) {
    err.value = e.message
    await load()
  } finally {
    savingOrder.value = false
  }
}

onMounted(load)
</script>

<template>
  <div>
    <PageHeader
      title="QoS 策略"
      description="PCQ 网段模板 profile_lpm · id 越小优先级越高"
    />
    <ShaperBindBar
      :bind-device="bindDevice"
      :dev-lan="devLan"
      :dev-wan="devWan"
      :attached="attached"
    />

    <div class="card p-4 mb-6 max-w-2xl text-sm">
      <h3 class="font-medium mb-3">全局 TC（HTB 叶子）</h3>
      <div class="flex flex-wrap gap-3 items-end">
        <div>
          <label class="text-xs text-slate-500">叶子 qdisc</label>
          <select v-model="tcLeaf" class="input-field mt-1">
            <option value="fq_codel">fq_codel</option>
            <option value="fq">fq</option>
            <option value="cake">cake</option>
          </select>
        </div>
        <div>
          <label class="text-xs text-slate-500">fq flows（0=默认）</label>
          <input v-model.number="tcFlows" type="number" min="0" class="input-field mt-1 w-28" />
        </div>
        <div>
          <label class="text-xs text-slate-500">fq quantum</label>
          <input v-model.number="tcQuantum" type="number" min="0" class="input-field mt-1 w-28" />
        </div>
        <button type="button" class="btn-secondary" :disabled="tcSaving" @click="saveTC">
          {{ tcSaving ? '应用中…' : '保存并重建根 qdisc' }}
        </button>
      </div>
    </div>
    <p class="text-sm text-slate-600 mb-4">
      向网段模板 <strong>添加</strong>规则（不覆盖已有策略网段与默认 profile）。单主机请填
      <code class="text-xs bg-slate-100 px-1 rounded">10.0.18.83/32</code>；相同 CIDR 再次提交则更新速率。
      列表按 <strong>id</strong> 排序（拖动 ⋮⋮ 调整优先级）；数据面按 LPM 最长前缀匹配。
    </p>

    <form class="card p-6 max-w-lg space-y-4 mb-6" @submit.prevent="submit">
      <div>
        <label class="text-sm">主网段 CIDR</label>
        <input v-model="form.cidr" class="input-field mt-1 font-mono" placeholder="10.0.0.0/16" />
      </div>
      <div class="grid grid-cols-2 gap-4">
        <div>
          <label class="text-sm">下行</label>
          <input v-model="form.down" class="input-field mt-1" placeholder="8mbit" />
        </div>
        <div>
          <label class="text-sm">上行</label>
          <input v-model="form.up" class="input-field mt-1" placeholder="8mbit" />
        </div>
      </div>
      <ProfileIfaceSelect
        v-model="form.device"
        :interfaces="interfaces"
        :default-device="bindDevice || devLan"
      />
      <div>
        <label class="text-sm">主机掩码</label>
        <input v-model.number="form.mask" type="number" min="1" max="32" class="input-field mt-1 w-24" />
      </div>
      <p v-if="ok" class="text-green-700 text-sm">{{ ok }}</p>
      <p v-if="err" class="text-red-600 text-sm">{{ err }}</p>
      <button type="submit" class="btn-primary">添加模板</button>
    </form>

    <p v-if="savingOrder" class="text-sm text-slate-500 mb-2">正在保存顺序…</p>

    <div class="card table-wrap p-4">
      <h3 class="font-medium mb-3 text-sm">已添加的网段模板</h3>
      <table class="data w-full text-sm">
        <thead>
          <tr>
            <th class="w-8"></th>
            <th class="w-14">ID</th>
            <th>网卡</th>
            <th>CIDR</th>
            <th>下行</th>
            <th>上行</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="(p, idx) in profiles"
            :key="p.cidr"
            draggable="true"
            class="cursor-grab active:cursor-grabbing hover:bg-slate-50"
            :class="{ 'opacity-50': dragIdx === idx }"
            @dragstart="onDragStart(idx)"
            @dragover="onDragOver"
            @drop="onDrop(idx)"
          >
            <td class="text-slate-400 text-center select-none" title="拖动排序">⋮⋮</td>
            <td class="text-center font-mono text-xs text-slate-500">{{ p.id }}</td>
            <td class="font-mono text-xs">{{ p.device || bindDevice || devLan }}</td>
            <td class="font-mono">{{ p.cidr }}</td>
            <td>{{ bpsLabel(p.down_bps) }}</td>
            <td>{{ bpsLabel(p.up_bps) }}</td>
            <td>
              <button type="button" class="text-red-600 text-xs" @click="remove(p.cidr)">删除</button>
            </td>
          </tr>
          <tr v-if="!profiles.length">
            <td colspan="7" class="text-center text-slate-400 py-6">暂无网段模板</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
