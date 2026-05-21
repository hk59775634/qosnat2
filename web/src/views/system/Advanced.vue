<script setup>
import { computed, onMounted, ref } from 'vue'
import { api } from '@/api/client'

const data = ref(null)
const overrides = ref({})
const appValues = ref({})
const err = ref('')
const ok = ref('')
const saving = ref(false)
const loading = ref(true)

const sysctlCategories = computed(() => {
  const cats = new Set((data.value?.catalog || []).map((e) => e.category))
  return [...cats]
})

const appCategories = computed(() => {
  const cats = new Set((data.value?.app_catalog || []).map((e) => e.category))
  return [...cats]
})

function sysctlRows(cat) {
  return (data.value?.catalog || []).filter((e) => e.category === cat)
}

function appRows(cat) {
  return (data.value?.app_catalog || []).filter((e) => e.category === cat)
}

function effectiveValue(key) {
  return data.value?.effective?.[key] ?? '—'
}

function liveValue(key) {
  const v = data.value?.live?.[key]
  return v === '' || v == null ? '—' : v
}

function recommendedValue(key) {
  return data.value?.recommended?.sysctl?.[key] ?? ''
}

function overrideValue(key) {
  if (key in overrides.value) return overrides.value[key]
  return data.value?.saved?.[key] ?? ''
}

function setOverride(key, val) {
  overrides.value = { ...overrides.value, [key]: val }
}

function clearOverride(key) {
  const next = { ...overrides.value }
  delete next[key]
  overrides.value = next
}

function appVal(key) {
  return appValues.value[key]
}

function setApp(key, val) {
  appValues.value = { ...appValues.value, [key]: val }
}

function syncFromResponse() {
  overrides.value = { ...(data.value?.saved || {}) }
  appValues.value = { ...(data.value?.app || {}) }
}

async function load() {
  loading.value = true
  err.value = ''
  try {
    data.value = await api.system.tuning.get()
    syncFromResponse()
  } catch (e) {
    data.value = null
    err.value =
      e.status === 404
        ? '接口不存在：请重新编译并部署 qosnatd（需包含 /api/v1/system/tuning）'
        : e.message || '加载失败'
  } finally {
    loading.value = false
  }
}

function buildBody(applyNow, applyRecommended = false) {
  const sysctl = {}
  for (const [k, v] of Object.entries(overrides.value)) {
    if (v !== undefined && v !== null && String(v).trim() !== '') {
      sysctl[k] = String(v).trim()
    }
  }
  return {
    sysctl,
    app: { ...appValues.value },
    apply: applyNow,
    apply_recommended: applyRecommended,
  }
}

async function save(applyNow) {
  err.value = ''
  ok.value = ''
  saving.value = true
  try {
    await api.system.tuning.put(buildBody(applyNow))
    ok.value = applyNow ? '已保存并应用到内核/网卡' : '已保存'
    await load()
  } catch (e) {
    err.value = e.message
  } finally {
    saving.value = false
  }
}

async function applyHardwareRecommend() {
  err.value = ''
  ok.value = ''
  saving.value = true
  try {
    const res = await api.system.tuning.put({ ...buildBody(true), apply_recommended: true })
    const b = data.value?.recommended?.memory_budget
    const extra = b
      ? `；conntrack max ${b.conntrack_max?.toLocaleString()} / buckets ${b.conntrack_buckets?.toLocaleString()}（优化内存 ${b.optimization_mb}MB）`
      : ''
    ok.value = `已按当前硬件（${data.value?.hardware_tier_label || res.tier || ''}）填入推荐值并应用${extra}`
    await load()
  } catch (e) {
    err.value = e.message
  } finally {
    saving.value = false
  }
}

function fillRecommendedToForm() {
  if (!data.value?.recommended) return
  const rec = data.value.recommended
  overrides.value = { ...(rec.sysctl || {}) }
  appValues.value = {
    'shaper.leaf': rec.qos?.leaf || 'fq_codel',
    'shaper.idle_timeout_sec': rec.qos?.idle_timeout_sec ?? 300,
    'system.txqueuelen_lan': rec.txqueuelen_lan ?? 0,
    'system.txqueuelen_wan': rec.txqueuelen_wan ?? 0,
    'system.rps_lan': !!rec.rps_lan,
    'system.rps_wan': !!rec.rps_wan,
    'system.perf_preset': !!rec.perf_preset,
  }
  ok.value = '已填入推荐值到表单，请点击「保存并应用」生效'
}

onMounted(load)
</script>

<template>
  <div class="page-stack">
    <h2 class="text-lg font-semibold mb-1">高级设置 · 系统优化</h2>
    <p class="page-hint mb-2">
      QoS/NAT 网关相关的内核 sysctl、连接跟踪、TCP、网卡队列与 HTB 叶子队列等。「按硬件重新推荐」将总内存的
      <strong>50%</strong> 划为优化专用预算，其中约 <strong>75%</strong> 用于 conntrack（按约 384 字节/连接估算
      <code class="text-xs">max</code> 与 <code class="text-xs">buckets=max/4</code>，2 的幂）。16GB 内存示例：
      max≈1677 万、buckets≈419 万。
    </p>

    <p v-if="loading" class="text-sm text-slate-500">加载中…</p>
    <p v-if="ok" class="text-green-700 text-sm mb-2">{{ ok }}</p>
    <p v-if="err" class="text-red-600 text-sm mb-2">{{ err }}</p>

    <div v-if="!loading && !data && err" class="card card-body max-w-xl">
      <p class="text-sm text-slate-700 mb-3">无法加载配置。若刚升级代码，请执行部署脚本重启服务。</p>
      <button type="button" class="btn-primary text-sm" @click="load">重试</button>
    </div>

    <div v-if="data" class="space-y-3">
      <div class="card card-body bg-blue-50/80 border-blue-100">
        <h3 class="font-medium text-slate-800 mb-2">本机硬件与推荐档位</h3>
        <p class="text-sm text-slate-700">
          CPU <strong>{{ data.hardware?.cpus ?? '—' }}</strong> 核 · 内存
          <strong>{{ data.hardware?.mem_mb ?? '—' }}</strong> MB · 检测档位
          <strong>{{ data.hardware_tier_label }}</strong>
          <span v-if="data.tuning_auto_applied" class="text-green-700 ml-2">（已自动应用推荐）</span>
          <span v-if="data.tuning_tier" class="text-slate-500 ml-1">保存档位: {{ data.tuning_tier }}</span>
        </p>
        <p
          v-if="data.recommended?.memory_budget"
          class="text-xs text-slate-600 mt-2 font-mono bg-white/80 rounded px-2 py-1.5"
        >
          优化专用 {{ data.recommended.memory_budget.optimization_mb }} MB（总内存 50%）· conntrack 预算
          {{ data.recommended.memory_budget.conntrack_mem_mb }} MB → max
          {{ data.recommended.memory_budget.conntrack_max?.toLocaleString() }} · buckets
          {{ data.recommended.memory_budget.conntrack_buckets?.toLocaleString() }}
          （≈{{ data.recommended.memory_budget.bytes_per_entry }} B/连接）
        </p>
        <p class="text-xs text-slate-500 mt-2">
          LAN <span class="font-mono">{{ data.dev_lan || '—' }}</span> · WAN
          <span class="font-mono">{{ data.dev_wan || '—' }}</span> · 配置
          <span class="font-mono">{{ data.conf_path }}</span>
        </p>
        <div class="flex flex-wrap gap-2 mt-3">
          <button type="button" class="btn-secondary text-sm" @click="fillRecommendedToForm">填入推荐值</button>
          <button type="button" class="btn-primary text-sm" :disabled="saving" @click="applyHardwareRecommend">
            按硬件重新推荐并应用
          </button>
        </div>
      </div>

      <div v-for="cat in appCategories" :key="'app-' + cat" class="card p-4">
        <h3 class="font-medium text-slate-800 mb-3">{{ cat }}</h3>
        <div class="space-y-4">
          <div v-for="row in appRows(cat)" :key="row.key" class="grid sm:grid-cols-2 gap-2 text-sm border-b border-slate-100 pb-3">
            <div>
              <div class="font-mono text-xs text-slate-600">{{ row.key }}</div>
              <div class="text-slate-500 text-xs mt-0.5">{{ row.description }}</div>
            </div>
            <div>
              <select
                v-if="row.type === 'select'"
                :value="appVal(row.key)"
                class="input-field font-mono text-xs"
                @change="setApp(row.key, $event.target.value)"
              >
                <option v-for="opt in row.options" :key="opt" :value="opt">{{ opt }}</option>
              </select>
              <label v-else-if="row.type === 'bool'" class="flex items-center gap-2 mt-2">
                <input
                  type="checkbox"
                  :checked="!!appVal(row.key)"
                  @change="setApp(row.key, $event.target.checked)"
                />
                启用
              </label>
              <input
                v-else
                :value="appVal(row.key)"
                type="number"
                :min="row.min"
                :max="row.max"
                class="input-field font-mono text-xs"
                @input="setApp(row.key, Number($event.target.value))"
              />
              <p
                v-if="row.key.startsWith('system.txqueuelen') && data.live_txqueuelen_lan"
                class="text-xs text-slate-400 mt-1"
              >
                当前内核 txqueuelen — LAN {{ data.live_txqueuelen_lan }} / WAN
                {{ data.live_txqueuelen_wan || '—' }}
              </p>
            </div>
          </div>
        </div>
      </div>

      <div v-for="cat in sysctlCategories" :key="cat" class="card card-body overflow-x-auto">
        <h3 class="font-medium text-slate-800 mb-1">{{ cat }}</h3>
        <p class="text-xs text-slate-500 mb-3">sysctl · 留空「手动覆盖」则使用默认 + 高性能预设合并结果</p>
        <table class="w-full text-sm min-w-[640px]">
          <thead>
            <tr class="text-left text-xs text-slate-500 border-b">
              <th class="pb-2 pr-2">参数</th>
              <th class="pb-2 pr-2">说明</th>
              <th class="pb-2 pr-2">生效</th>
              <th class="pb-2 pr-2">内核</th>
              <th class="pb-2 pr-2">推荐</th>
              <th class="pb-2">覆盖</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in sysctlRows(cat)" :key="row.key" class="border-b border-slate-50">
              <td class="py-2 pr-2 font-mono text-[11px] align-top max-w-[10rem] break-all">{{ row.key }}</td>
              <td class="py-2 pr-2 text-slate-600 align-top text-xs max-w-[12rem]">{{ row.description }}</td>
              <td class="py-2 pr-2 font-mono text-[11px] align-top">{{ effectiveValue(row.key) }}</td>
              <td class="py-2 pr-2 font-mono text-[11px] align-top text-slate-500">{{ liveValue(row.key) }}</td>
              <td class="py-2 pr-2 font-mono text-[11px] align-top text-blue-700">
                {{ recommendedValue(row.key) || '—' }}
              </td>
              <td class="py-2 align-top min-w-[7rem]">
                <input
                  :value="overrideValue(row.key)"
                  type="text"
                  class="input-field font-mono text-xs"
                  :placeholder="row.performance || row.default"
                  @input="setOverride(row.key, $event.target.value)"
                />
                <button
                  v-if="overrideValue(row.key)"
                  type="button"
                  class="text-xs text-slate-500 mt-0.5 block"
                  @click="clearOverride(row.key)"
                >
                  清除
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="flex flex-wrap gap-3 sticky bottom-0 bg-slate-100/90 py-2">
        <button type="button" class="btn-secondary" :disabled="saving" @click="save(false)">仅保存</button>
        <button type="button" class="btn-primary" :disabled="saving" @click="save(true)">
          {{ saving ? '处理中…' : '保存并应用' }}
        </button>
        <button type="button" class="text-sm text-slate-600" :disabled="saving" @click="load">刷新</button>
      </div>
    </div>
  </div>
</template>
