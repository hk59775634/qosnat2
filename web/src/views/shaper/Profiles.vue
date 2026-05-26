<script setup>
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api, bpsLabel } from '@/api/client'

const { t } = useI18n()
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
    ok.value = t('shaper.profiles.tcApplied')
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
      ? t('shaper.profiles.profileAdded', { cidr: res.cidr })
      : t('shaper.profiles.profileUpdated', { cidr: res.cidr })
    await load()
  } catch (e) {
    err.value = e.message
  }
}

async function remove(cidr) {
  if (!confirm(t('shaper.profiles.confirmDelete', { cidr }))) return
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
    ok.value = t('shaper.profiles.orderSaved')
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
  <div class="page-stack">
    <PageHeader :title="t('shaper.profiles.title')" :description="t('shaper.profiles.description')" />

    <div class="card card-body text-sm space-y-2">
      <ShaperBindBar
        embedded
        :bind-device="bindDevice"
        :dev-lan="devLan"
        :dev-wan="devWan"
        :attached="attached"
      />
      <div class="border-t border-slate-100 pt-2 flex flex-wrap gap-2 items-end">
        <div class="min-w-[7rem]">
          <label class="text-xs text-slate-500">{{ t('shaper.profiles.leafQdisc') }}</label>
          <select v-model="tcLeaf" class="input-field mt-0.5">
            <option value="fq_codel">fq_codel</option>
            <option value="cake">cake</option>
          </select>
        </div>
        <div>
          <label class="text-xs text-slate-500" :title="t('shaper.profiles.fqFlowsTitle')">{{ t('shaper.profiles.fqFlows') }}</label>
          <input v-model.number="tcFlows" type="number" min="0" class="input-field mt-0.5 w-20" />
        </div>
        <div>
          <label class="text-xs text-slate-500" :title="t('shaper.profiles.fqFlowsTitle')">{{ t('shaper.profiles.fqQuantum') }}</label>
          <input v-model.number="tcQuantum" type="number" min="0" class="input-field mt-0.5 w-20" />
        </div>
        <button type="button" class="btn-secondary" :disabled="tcSaving" @click="saveTC">
          {{ tcSaving ? t('shaper.profiles.applying') : t('shaper.profiles.rebuildRoot') }}
        </button>
      </div>
    </div>

    <form class="card card-body grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-7 gap-2 items-end" @submit.prevent="submit">
      <div class="lg:col-span-2">
        <label class="text-xs text-slate-600">{{ t('shaper.profiles.cidrLabel') }}</label>
        <input v-model="form.cidr" class="input-field mt-0.5 font-mono" :placeholder="t('shaper.profiles.cidrPh')" />
      </div>
      <div>
        <label class="text-xs text-slate-600">{{ t('shaper.profiles.colDown') }}</label>
        <input v-model="form.down" class="input-field mt-0.5" placeholder="8mbit" />
      </div>
      <div>
        <label class="text-xs text-slate-600">{{ t('shaper.profiles.colUp') }}</label>
        <input v-model="form.up" class="input-field mt-0.5" placeholder="8mbit" />
      </div>
      <div class="sm:col-span-2 lg:col-span-1">
        <ProfileIfaceSelect
          v-model="form.device"
          :interfaces="interfaces"
          :default-device="bindDevice || devLan"
          compact
        />
      </div>
      <div>
        <label class="text-xs text-slate-600">{{ t('shaper.profiles.mask') }}</label>
        <input v-model.number="form.mask" type="number" min="1" max="32" class="input-field mt-0.5 w-full" />
      </div>
      <div class="flex flex-col gap-1">
        <button type="submit" class="btn-primary w-full">{{ t('shaper.profiles.add') }}</button>
        <p v-if="ok" class="text-green-700 text-xs truncate" :title="ok">{{ ok }}</p>
        <p v-if="err" class="text-red-600 text-xs truncate" :title="err">{{ err }}</p>
      </div>
    </form>

    <p class="page-hint">
      <i18n-t keypath="shaper.profiles.helpPrefix" tag="span">
        <template #ex24><code class="bg-slate-100 px-1 rounded">10.0.0.0/24</code></template>
        <template #ex32><code class="bg-slate-100 px-1 rounded">x.x.x.x/32</code></template>
      </i18n-t>
    </p>

    <p v-if="savingOrder" class="text-xs text-slate-500">{{ t('security.firewall.savingOrder') }}</p>

    <div class="card table-wrap card-body !p-2">
      <table class="data w-full">
        <thead>
          <tr>
            <th class="w-7"></th>
            <th class="w-10">ID</th>
            <th>{{ t('shaper.profiles.colIface') }}</th>
            <th>{{ t('shaper.profiles.cidrLabel') }}</th>
            <th>{{ t('shaper.profiles.colDown') }}</th>
            <th>{{ t('shaper.profiles.colUp') }}</th>
            <th class="w-24 text-right">{{ t('common.actions') }}</th>
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
            <td class="text-slate-400 text-center select-none text-xs" :title="t('shaper.profiles.dragSort')">⋮⋮</td>
            <td class="text-center font-mono text-slate-500">{{ p.id }}</td>
            <td class="font-mono text-xs">{{ p.device || bindDevice || devLan }}</td>
            <td class="font-mono">{{ p.cidr }}</td>
            <td>{{ bpsLabel(p.down_bps) }}</td>
            <td>{{ bpsLabel(p.up_bps) }}</td>
            <td class="text-right whitespace-nowrap">
              <button type="button" class="btn-danger" :title="t('common.delete')" @click="remove(p.cidr)">
                {{ t('common.delete') }}
              </button>
            </td>
          </tr>
          <tr v-if="!profiles.length">
            <td colspan="7" class="text-center text-slate-400 py-3">{{ t('shaper.profiles.noProfiles') }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
