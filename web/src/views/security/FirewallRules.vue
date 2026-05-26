<script setup>
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'
import {
  actionMeta,
  builtinRulesForChain,
  formatDestination,
  formatIface,
  formatPort,
  formatProto,
  formatSource,
  mergeChainReorder,
  rulesForChain,
} from '@/lib/firewallRuleDisplay'
import { emptyRuleForm, formToPayload, isRuleMutable, ruleToForm } from '@/lib/firewallRuleForm'

const { t } = useI18n()

const rules = ref([])
const devLan = ref('')
const devWan = ref('')
const adminPort = ref('')
const aliasNames = ref([])
const vpnMeta = ref({})
const activeChain = ref('forward')
const showRendered = ref(false)
const showForm = ref(false)
const rendered = ref('')
const err = ref('')
const ok = ref('')
const dragIdx = ref(null)
const savingOrder = ref(false)
const editing = ref(null)
const formPanel = ref(null)

const chains = [
  { id: 'forward', labelKey: 'security.firewall.tabForward' },
  { id: 'input', labelKey: 'security.firewall.tabInput' },
]

const form = ref(emptyRuleForm())

const userRulesInChain = computed(() => rulesForChain(rules.value, activeChain.value))

const showOutCol = computed(() => activeChain.value === 'forward')

const tableColspan = computed(() => (showOutCol.value ? 12 : 11))

const builtinCtx = computed(() => ({
  adminPort: adminPort.value,
  vpn: vpnMeta.value,
}))

const builtinRows = computed(() =>
  builtinRulesForChain(activeChain.value, devLan.value, devWan.value, builtinCtx.value, t),
)

const customCount = computed(() => userRulesInChain.value.length)

const isInputChain = computed(() => form.value.chain === 'input')

async function load() {
  const d = await api.firewall.rules.list()
  rules.value = d.rules || []
  devLan.value = d.dev_lan || ''
  devWan.value = d.dev_wan || ''
  adminPort.value = d.admin_port || ''
  aliasNames.value = d.alias_names || []
  vpnMeta.value = d.vpn || {}
  rendered.value = d.rendered || ''
}

function openAdd() {
  editing.value = null
  form.value = emptyRuleForm(activeChain.value)
  showForm.value = true
}

function apiError(e) {
  return e?.data?.error || e?.message || String(e)
}

function startEdit(r) {
  if (!isRuleMutable(r)) {
    err.value = t('security.firewall.ruleLocked')
    return
  }
  editing.value = r.id
  form.value = ruleToForm(r, devLan.value, devWan.value)
  showForm.value = true
  requestAnimationFrame(() => {
    formPanel.value?.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
  })
}

function cancelEdit() {
  editing.value = null
  form.value = emptyRuleForm(activeChain.value)
  showForm.value = false
}

function buildPayload() {
  return formToPayload(form.value, devLan.value, devWan.value)
}

async function add() {
  err.value = ''
  try {
    await api.firewall.rules.add(buildPayload())
    ok.value = t('security.firewall.addedOk')
    cancelEdit()
    await load()
  } catch (e) {
    err.value = apiError(e)
  }
}

async function saveEdit() {
  if (!editing.value) return
  err.value = ''
  ok.value = ''
  try {
    await api.firewall.rules.put(editing.value, { ...buildPayload(), id: editing.value })
    ok.value = t('security.firewall.updatedOk')
    cancelEdit()
    await load()
  } catch (e) {
    err.value = apiError(e)
  }
}

async function toggleEnabled(r) {
  if (!isRuleMutable(r)) return
  err.value = ''
  ok.value = ''
  try {
    await api.firewall.rules.put(r.id, { ...r, enabled: !r.enabled })
    ok.value = t('security.firewall.updatedOk')
    await load()
  } catch (e) {
    err.value = apiError(e)
  }
}

async function remove(r) {
  if (!isRuleMutable(r)) {
    err.value = t('security.firewall.ruleLocked')
    return
  }
  if (!confirm(t('security.firewall.confirmDelete'))) return
  err.value = ''
  ok.value = ''
  try {
    await api.firewall.rules.del(r.id)
    if (editing.value === r.id) cancelEdit()
    ok.value = t('security.firewall.deletedOk')
    await load()
  } catch (e) {
    err.value = apiError(e)
  }
}

function onDragStart(idx) {
  dragIdx.value = idx
}

function onDragOver(e) {
  e.preventDefault()
}

async function persistOrder(reorderedSubset) {
  savingOrder.value = true
  err.value = ''
  try {
    const merged = mergeChainReorder(rules.value, activeChain.value, reorderedSubset)
    const res = await api.firewall.rules.reorder(merged.map((r) => r.id))
    rules.value = res.rules || merged
    ok.value = t('security.firewall.orderSaved')
  } catch (e) {
    err.value = e.message
    await load()
  } finally {
    savingOrder.value = false
  }
}

async function onDrop(targetIdx) {
  if (dragIdx.value === null || dragIdx.value === targetIdx) {
    dragIdx.value = null
    return
  }
  const arr = [...userRulesInChain.value]
  const [item] = arr.splice(dragIdx.value, 1)
  arr.splice(targetIdx, 0, item)
  dragIdx.value = null
  await persistOrder(arr)
}

async function moveRule(idx, dir) {
  const j = idx + dir
  if (j < 0 || j >= userRulesInChain.value.length) return
  const arr = [...userRulesInChain.value]
  ;[arr[idx], arr[j]] = [arr[j], arr[idx]]
  await persistOrder(arr)
}

function rowActionLabel(action) {
  const m = actionMeta(action)
  const key = { pass: 'actionPass', block: 'actionBlock', reject: 'actionReject' }[m.badge]
  return t(`security.firewall.${key}`)
}

onMounted(load)
</script>

<template>
  <div class="page-stack fw-page">
    <PageHeader :title="t('security.firewall.title')" :description="t('security.firewall.description')" />

    <p v-if="ok" class="text-green-700 text-sm">{{ ok }}</p>
    <p v-if="err" class="text-red-600 text-sm">{{ err }}</p>

    <!-- 接口角色条（类似 pfSense 接口名） -->
    <div class="fw-iface-bar card card-body flex flex-wrap items-center gap-3 text-sm">
      <span class="text-slate-500">{{ t('security.firewall.ifaceRoles') }}</span>
      <span v-if="devLan" class="fw-iface-pill fw-iface-lan">
        <span class="fw-iface-tag">LAN</span>
        <span class="font-mono">{{ devLan }}</span>
      </span>
      <span v-else class="text-slate-400 text-xs">{{ t('security.firewall.noLan') }}</span>
      <span v-if="devWan" class="fw-iface-pill fw-iface-wan">
        <span class="fw-iface-tag">WAN</span>
        <span class="font-mono">{{ devWan }}</span>
      </span>
      <span class="ml-auto flex flex-wrap gap-2">
        <button type="button" class="btn-primary text-sm" @click="openAdd">
          {{ t('security.firewall.addRule') }}
        </button>
        <button type="button" class="btn-secondary text-sm" @click="showForm = !showForm">
          {{ showForm ? t('security.firewall.hideForm') : t('security.firewall.showForm') }}
        </button>
      </span>
    </div>

    <!-- 链 Tab -->
    <nav class="fw-tabs flex gap-1 border-b border-slate-200">
      <button
        v-for="c in chains"
        :key="c.id"
        type="button"
        class="fw-tab px-4 py-2 text-sm font-medium rounded-t transition-colors"
        :class="
          activeChain === c.id
            ? 'bg-white border border-b-white border-slate-200 text-pfsense-nav -mb-px'
            : 'text-slate-500 hover:text-slate-800 hover:bg-slate-50'
        "
        @click="activeChain = c.id"
      >
        {{ t(c.labelKey) }}
        <span class="ml-1 text-xs opacity-70">({{ rulesForChain(rules, c.id).length }})</span>
      </button>
    </nav>

    <!-- 规则表（pfSense 风格） -->
    <div class="card overflow-hidden border-t-0 rounded-t-none">
      <p v-if="savingOrder" class="text-xs text-slate-500 px-3 py-2 bg-slate-50 border-b">
        {{ t('security.firewall.savingOrder') }}
      </p>

      <div class="table-wrap">
        <table class="data fw-rules-table w-full text-sm">
          <thead>
            <tr>
              <th class="fw-col-drag w-8"></th>
              <th class="w-10 text-center">#</th>
              <th class="w-24">{{ t('security.firewall.colAction') }}</th>
              <th class="w-28">{{ t('security.firewall.colIn') }}</th>
              <th v-if="showOutCol" class="w-28">{{ t('security.firewall.colOut') }}</th>
              <th class="w-16">{{ t('security.firewall.colProto') }}</th>
              <th>{{ t('security.firewall.colSource') }}</th>
              <th class="w-16 text-center">{{ t('security.firewall.colSPort') }}</th>
              <th>{{ t('security.firewall.colDest') }}</th>
              <th class="w-16 text-center">{{ t('security.firewall.colDPort') }}</th>
              <th>{{ t('security.firewall.colDescription') }}</th>
              <th class="w-28 text-right">{{ t('security.firewall.colActions') }}</th>
            </tr>
          </thead>
          <tbody>
            <!-- 系统默认规则 -->
            <tr class="fw-section-row">
              <td :colspan="tableColspan" class="!py-1.5 !bg-slate-100 !text-xs !font-semibold !text-slate-600">
                {{ t('security.firewall.sectionSystem') }}
              </td>
            </tr>
            <tr
              v-for="(br, bi) in builtinRows"
              :key="br.id"
              class="fw-row-system"
            >
              <td class="text-center text-slate-300">—</td>
              <td class="text-center text-xs text-slate-400">{{ bi + 1 }}</td>
              <td>
                <span
                  class="fw-action-badge"
                  :class="actionMeta(br.action).class"
                >
                  {{ rowActionLabel(br.action) }}
                </span>
              </td>
              <td :colspan="showOutCol ? 2 : 1" class="text-xs text-slate-600">{{ br.summary }}</td>
              <td class="text-center text-slate-400">*</td>
              <td class="text-slate-400">*</td>
              <td class="text-center text-slate-400">*</td>
              <td class="text-slate-400">*</td>
              <td class="text-center text-slate-400">*</td>
              <td class="text-xs text-slate-500">
                {{ br.detail || t('security.firewall.systemRule') }}
              </td>
              <td></td>
            </tr>

            <!-- 自定义规则 -->
            <tr class="fw-section-row">
              <td :colspan="tableColspan" class="!py-1.5 !bg-blue-50 !text-xs !font-semibold !text-blue-900">
                {{ t('security.firewall.sectionCustom') }}
                <span class="font-normal text-blue-700">({{ customCount }})</span>
              </td>
            </tr>
            <tr v-if="customCount === 0" class="fw-row-empty">
              <td :colspan="tableColspan" class="text-center text-slate-400 py-6 text-sm">
                {{ t('security.firewall.noCustom') }}
              </td>
            </tr>
            <tr
              v-for="(r, idx) in userRulesInChain"
              :key="r.id || `row-${idx}`"
              class="fw-row-custom"
              :class="{
                'opacity-40': !r.enabled,
                'opacity-50': dragIdx === idx,
                'bg-blue-50/80 ring-1 ring-blue-200': editing === r.id,
              }"
              @dragover="onDragOver"
              @drop="onDrop(idx)"
            >
              <td
                class="text-slate-400 text-center select-none text-xs cursor-grab active:cursor-grabbing"
                draggable="true"
                :title="t('security.firewall.dragHint')"
                @dragstart="onDragStart(idx)"
              >
                ⋮⋮
              </td>
              <td class="text-center text-xs text-slate-500 font-mono">{{ idx + 1 }}</td>
              <td>
                <span class="fw-action-badge" :class="actionMeta(r.action).class">
                  {{ rowActionLabel(r.action) }}
                </span>
              </td>
              <td>
                <span v-if="formatIface(r.iif, devLan, devWan).name !== '—'" class="fw-iface-cell">
                  <span v-if="formatIface(r.iif, devLan, devWan).role" class="fw-mini-tag">{{
                    formatIface(r.iif, devLan, devWan).role
                  }}</span>
                  <span class="font-mono text-xs">{{ formatIface(r.iif, devLan, devWan).name }}</span>
                </span>
                <span v-else class="text-slate-400">*</span>
              </td>
              <td v-if="showOutCol">
                <span v-if="formatIface(r.oif, devLan, devWan).name !== '—'" class="fw-iface-cell">
                  <span v-if="formatIface(r.oif, devLan, devWan).role" class="fw-mini-tag">{{
                    formatIface(r.oif, devLan, devWan).role
                  }}</span>
                  <span class="font-mono text-xs">{{ formatIface(r.oif, devLan, devWan).name }}</span>
                </span>
                <span v-else class="text-slate-400">*</span>
              </td>
              <td class="font-mono text-xs text-center">{{ formatProto(r.proto) }}</td>
              <td>
                <span
                  class="font-mono text-xs"
                  :class="formatSource(r).kind === 'alias' ? 'text-violet-700' : ''"
                >
                  <template v-if="formatSource(r).kind === 'alias'">@{{ formatSource(r).label }}</template>
                  <template v-else>{{ formatSource(r).label }}</template>
                </span>
              </td>
              <td class="text-center font-mono text-xs">{{ formatPort(r.src_port) }}</td>
              <td>
                <span
                  class="font-mono text-xs"
                  :class="formatDestination(r).kind === 'alias' ? 'text-violet-700' : ''"
                >
                  <template v-if="formatDestination(r).kind === 'alias'"
                    >@{{ formatDestination(r).label }}</template
                  >
                  <template v-else>{{ formatDestination(r).label }}</template>
                </span>
              </td>
              <td class="text-center font-mono text-xs">{{ formatPort(r.dst_port) }}</td>
              <td class="text-xs text-slate-700 max-w-[12rem] truncate" :title="r.comment || r.id">
                {{ r.comment || '—' }}
              </td>
              <td class="text-right whitespace-nowrap fw-col-actions">
                <template v-if="isRuleMutable(r)">
                  <button
                    type="button"
                    class="fw-icon-btn"
                    :title="t('security.firewall.moveUp')"
                    @click.stop="moveRule(idx, -1)"
                  >
                    ↑
                  </button>
                  <button
                    type="button"
                    class="fw-icon-btn"
                    :title="t('security.firewall.moveDown')"
                    @click.stop="moveRule(idx, 1)"
                  >
                    ↓
                  </button>
                  <button
                    type="button"
                    class="fw-icon-btn"
                    :class="r.enabled ? 'text-slate-600' : 'text-green-700'"
                    :title="r.enabled ? t('security.firewall.disableRule') : t('security.firewall.enableRule')"
                    @click.stop="toggleEnabled(r)"
                  >
                    {{ r.enabled ? '○' : '●' }}
                  </button>
                  <button type="button" class="fw-icon-btn text-blue-600" @click.stop="startEdit(r)">
                    {{ t('common.edit') }}
                  </button>
                  <button type="button" class="fw-icon-btn text-red-600" @click.stop="remove(r)">
                    {{ t('common.delete') }}
                  </button>
                </template>
                <span
                  v-else
                  class="text-xs text-slate-400"
                  :title="t('security.firewall.ruleLocked')"
                >
                  🔒
                </span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <p class="text-xs text-slate-500 px-3 py-2 border-t bg-slate-50">
        {{ t('security.firewall.orderHint') }}
      </p>
    </div>

    <!-- 添加/编辑表单 -->
    <div
      v-show="showForm"
      ref="formPanel"
      class="card card-body space-y-3 text-sm border-l-4 border-l-blue-500"
    >
      <h3 class="font-medium text-pfsense-nav">
        {{ editing ? t('security.firewall.editRule') : t('security.firewall.addRule') }}
        <span class="text-slate-400 font-normal">({{ activeChain }})</span>
      </h3>
      <div class="grid sm:grid-cols-2 lg:grid-cols-3 gap-3">
        <div>
          <label class="text-xs text-slate-500">{{ t('security.firewall.chain') }}</label>
          <select v-model="form.chain" class="input-field mt-1">
            <option value="forward">forward</option>
            <option value="input">input</option>
          </select>
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('security.firewall.action') }}</label>
          <select v-model="form.action" class="input-field mt-1">
            <option value="accept">{{ t('security.firewall.actionPass') }}</option>
            <option value="drop">{{ t('security.firewall.actionBlock') }}</option>
            <option value="reject">{{ t('security.firewall.actionReject') }}</option>
          </select>
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('security.firewall.inIface') }}</label>
          <select v-model="form.iif_mode" class="input-field mt-1">
            <option value="any">{{ t('security.firewall.optAny') }}</option>
            <option v-if="devLan" value="lan">LAN ({{ devLan }})</option>
            <option v-if="devWan" value="wan">WAN ({{ devWan }})</option>
            <option value="custom">{{ t('security.firewall.optCustomIface') }}</option>
          </select>
          <input
            v-if="form.iif_mode === 'custom'"
            v-model="form.iif_custom"
            class="input-field mt-1 font-mono text-xs"
            :placeholder="t('security.firewall.customIfacePh')"
          />
        </div>
        <div v-if="!isInputChain">
          <label class="text-xs text-slate-500">{{ t('security.firewall.outIface') }}</label>
          <select v-model="form.oif_mode" class="input-field mt-1">
            <option value="any">{{ t('security.firewall.optAny') }}</option>
            <option v-if="devLan" value="lan">LAN ({{ devLan }})</option>
            <option v-if="devWan" value="wan">WAN ({{ devWan }})</option>
            <option value="custom">{{ t('security.firewall.optCustomIface') }}</option>
          </select>
          <input
            v-if="form.oif_mode === 'custom'"
            v-model="form.oif_custom"
            class="input-field mt-1 font-mono text-xs"
            :placeholder="t('security.firewall.customIfacePh')"
          />
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('security.firewall.protocol') }}</label>
          <input v-model="form.proto" class="input-field mt-1" placeholder="tcp / udp / icmp" />
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('security.firewall.srcPort') }}</label>
          <select v-model="form.src_port_mode" class="input-field mt-1">
            <option value="any">{{ t('security.firewall.optAny') }}</option>
            <option value="custom">{{ t('security.firewall.optCustomPort') }}</option>
          </select>
          <input
            v-if="form.src_port_mode === 'custom'"
            v-model="form.src_port_custom"
            type="number"
            min="1"
            max="65535"
            class="input-field mt-1 font-mono"
          />
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('security.firewall.dstPort') }}</label>
          <select v-model="form.dst_port_mode" class="input-field mt-1">
            <option value="any">{{ t('security.firewall.optAny') }}</option>
            <option value="custom">{{ t('security.firewall.optCustomPort') }}</option>
          </select>
          <input
            v-if="form.dst_port_mode === 'custom'"
            v-model="form.dst_port_custom"
            type="number"
            min="1"
            max="65535"
            class="input-field mt-1 font-mono"
          />
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('security.firewall.srcAddr') }}</label>
          <select v-model="form.src_mode" class="input-field mt-1">
            <option value="any">{{ t('security.firewall.optAny') }}</option>
            <option value="cidr">{{ t('security.firewall.optCustomCidr') }}</option>
            <option value="alias">{{ t('security.firewall.optAlias') }}</option>
          </select>
          <input
            v-if="form.src_mode === 'cidr'"
            v-model="form.src_cidr"
            class="input-field mt-1 font-mono text-xs"
            placeholder="10.0.0.0/8"
          />
          <select
            v-if="form.src_mode === 'alias'"
            v-model="form.src_alias"
            class="input-field mt-1 font-mono text-xs"
          >
            <option value="">{{ t('security.firewall.pickAlias') }}</option>
            <option v-for="a in aliasNames" :key="'s-' + a" :value="a">{{ a }}</option>
          </select>
        </div>
        <div>
          <label class="text-xs text-slate-500">{{ t('security.firewall.dstAddr') }}</label>
          <select v-model="form.dst_mode" class="input-field mt-1">
            <option value="any">{{ t('security.firewall.optAny') }}</option>
            <option value="cidr">{{ t('security.firewall.optCustomCidr') }}</option>
            <option value="alias">{{ t('security.firewall.optAlias') }}</option>
          </select>
          <input
            v-if="form.dst_mode === 'cidr'"
            v-model="form.dst_cidr"
            class="input-field mt-1 font-mono text-xs"
          />
          <select
            v-if="form.dst_mode === 'alias'"
            v-model="form.dst_alias"
            class="input-field mt-1 font-mono text-xs"
          >
            <option value="">{{ t('security.firewall.pickAlias') }}</option>
            <option v-for="a in aliasNames" :key="'d-' + a" :value="a">{{ a }}</option>
          </select>
        </div>
        <div class="sm:col-span-2 lg:col-span-3">
          <label class="text-xs text-slate-500">{{ t('security.firewall.colDescription') }}</label>
          <input v-model="form.comment" class="input-field mt-1" />
        </div>
      </div>
      <label class="flex items-center gap-2">
        <input v-model="form.enabled" type="checkbox" />
        {{ t('security.firewall.enabled') }}
      </label>
      <div class="flex flex-wrap gap-2">
        <button v-if="!editing" type="button" class="btn-primary" @click="add">
          {{ t('security.firewall.addApply') }}
        </button>
        <template v-else>
          <button type="button" class="btn-primary" @click="saveEdit">{{ t('security.firewall.saveEdit') }}</button>
          <button type="button" class="btn-secondary" @click="cancelEdit">{{ t('common.cancel') }}</button>
        </template>
      </div>
    </div>

    <button type="button" class="text-sm text-slate-600 hover:text-slate-900" @click="showRendered = !showRendered">
      {{ showRendered ? t('security.firewall.hideNft') : t('security.firewall.showNft') }}
    </button>
    <pre v-if="showRendered" class="text-xs bg-slate-900 text-slate-100 p-4 rounded-lg overflow-auto max-h-96">{{
      rendered
    }}</pre>
  </div>
</template>

<style scoped>
.fw-iface-bar {
  @apply border border-slate-200;
}

.fw-iface-pill {
  @apply inline-flex items-center gap-1.5 px-2 py-1 rounded-md border text-xs;
}

.fw-iface-lan {
  @apply bg-emerald-50 border-emerald-200 text-emerald-900;
}

.fw-iface-wan {
  @apply bg-sky-50 border-sky-200 text-sky-900;
}

.fw-iface-tag {
  @apply font-bold text-[10px] uppercase tracking-wide opacity-80;
}

.fw-rules-table thead th {
  @apply sticky top-0 z-10 whitespace-nowrap;
}

.fw-action-badge {
  @apply inline-block px-2 py-0.5 rounded text-xs font-semibold uppercase tracking-wide;
}

.fw-action-pass {
  @apply bg-green-100 text-green-800 border border-green-200;
}

.fw-action-block {
  @apply bg-red-100 text-red-800 border border-red-200;
}

.fw-action-reject {
  @apply bg-amber-100 text-amber-900 border border-amber-200;
}

.fw-row-system {
  @apply bg-slate-50/80;
}

.fw-row-system td {
  @apply border-b border-slate-100;
}

.fw-row-custom:hover {
  @apply bg-slate-50;
}

.fw-iface-cell {
  @apply inline-flex items-center gap-1;
}

.fw-mini-tag {
  @apply text-[10px] font-bold px-1 rounded bg-slate-200 text-slate-700;
}

.fw-icon-btn {
  @apply text-xs px-1 py-0.5 rounded hover:bg-slate-100;
}

.fw-col-actions {
  @apply sticky right-0 z-[5] bg-white shadow-[-4px_0_6px_-4px_rgba(0,0,0,0.08)];
}

.fw-row-custom:hover .fw-col-actions {
  @apply bg-slate-50;
}

.fw-section-row td {
  @apply border-b border-slate-200;
}
</style>
