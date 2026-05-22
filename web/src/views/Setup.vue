<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { api } from '@/api/client'

const router = useRouter()
const step = ref(0)
const steps = ['欢迎', '管理员', '网卡', 'NAT（可选）', '完成']
const loading = ref(false)
const err = ref('')
const ifaces = ref([])

const form = ref({
  admin_user: 'admin',
  admin_pass: '',
  admin_pass2: '',
  dev_lan: '',
  dev_wan: '',
  policy_routes: '10.0.0.0/8',
  shared_ip: '',
  hostname: 'qosnat2',
  enable_dhcp: false,
  apply_dataplane: true,
})

const progress = computed(() => Math.round(((step.value + 1) / steps.length) * 100))

onMounted(async () => {
  try {
    const st = await api.setup.status()
    if (st.setup_complete) {
      router.replace('/')
      return
    }
    const res = await api.setup.interfaces()
    ifaces.value = res.interfaces || []
    if (!form.value.dev_lan && ifaces.value.length) {
      const up = ifaces.value.filter((i) => i.up)
      if (up.length >= 1) form.value.dev_lan = up[0].name
      if (up.length >= 2) form.value.dev_wan = up[1].name
    }
  } catch (e) {
    err.value = e.message || '无法连接 API'
  }
})

function next() {
  err.value = ''
  if (step.value === 1) {
    if (form.value.admin_pass.length < 8) {
      err.value = '密码至少 8 位'
      return
    }
    if (form.value.admin_pass !== form.value.admin_pass2) {
      err.value = '两次密码不一致'
      return
    }
  }
  if (step.value === 2) {
    if (!form.value.dev_lan || !form.value.dev_wan) {
      err.value = '请选择 LAN 与 WAN 网卡'
      return
    }
    if (form.value.dev_lan === form.value.dev_wan) {
      err.value = 'LAN 与 WAN 不能相同'
      return
    }
  }
  if (step.value < steps.length - 1) step.value++
}

function back() {
  err.value = ''
  if (step.value > 0) step.value--
}

async function finish() {
  err.value = ''
  loading.value = true
  try {
    const routes = form.value.policy_routes
      .split(/[\n,]+/)
      .map((s) => s.trim())
      .filter(Boolean)
    const shared = form.value.shared_ip.trim() ? [form.value.shared_ip.trim()] : []
    await api.setup.complete({
      admin_user: form.value.admin_user,
      admin_pass: form.value.admin_pass,
      dev_lan: form.value.dev_lan,
      dev_wan: form.value.dev_wan,
      policy_routes: routes.length ? routes : ['10.0.0.0/8'],
      shared_ips: shared,
      hostname: form.value.hostname,
      enable_dhcp: form.value.enable_dhcp,
      apply_dataplane: form.value.apply_dataplane,
    })
    router.replace('/')
  } catch (e) {
    err.value = e.message || '设置失败'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="min-h-screen bg-gradient-to-br from-slate-800 to-pfsense-nav flex items-center justify-center p-4">
    <div class="card w-full max-w-xl p-8 shadow-xl">
      <div class="mb-3">
        <h1 class="text-2xl font-semibold text-pfsense-nav">qosnat2 初始设置</h1>
        <p class="text-sm text-slate-500 mt-1">完成引导后才会加载 NAT、QoS 与防火墙规则。远程部署可直接访问本向导，无需 token。</p>
        <div class="mt-4 h-2 bg-slate-200 rounded-full overflow-hidden">
          <div class="h-full bg-blue-600 transition-all" :style="{ width: progress + '%' }" />
        </div>
        <p class="text-xs text-slate-400 mt-2">步骤 {{ step + 1 }} / {{ steps.length }}：{{ steps[step] }}</p>
      </div>

      <div v-if="step === 0" class="space-y-3 text-sm text-slate-600">
        <p>欢迎使用 qosnat2。当前仅启动了 Web 管理界面，数据面尚未生效。</p>
        <ul class="list-disc pl-5 space-y-1">
          <li>创建管理员账号与密码</li>
          <li>选择内网（LAN）与外网（WAN）物理接口</li>
          <li>可选配置 SNAT 公网 IP 与策略网段</li>
        </ul>
        <p class="text-xs text-slate-400">流程参考 AdGuard Home 首次安装向导。</p>
      </div>

      <div v-else-if="step === 1" class="space-y-3">
        <div>
          <label class="block text-sm mb-1">管理员用户名</label>
          <input v-model="form.admin_user" class="input-field" autocomplete="username" />
        </div>
        <div>
          <label class="block text-sm mb-1">密码（至少 8 位）</label>
          <input v-model="form.admin_pass" type="password" class="input-field" autocomplete="new-password" />
        </div>
        <div>
          <label class="block text-sm mb-1">确认密码</label>
          <input v-model="form.admin_pass2" type="password" class="input-field" autocomplete="new-password" />
        </div>
      </div>

      <div v-else-if="step === 2" class="space-y-3">
        <div>
          <label class="block text-sm mb-1">内网接口 (LAN)</label>
          <select v-model="form.dev_lan" class="input-field">
            <option value="">— 选择 —</option>
            <option v-for="i in ifaces" :key="'l-' + i.name" :value="i.name">
              {{ i.name }} {{ i.up ? '(UP)' : '' }} {{ i.addrs?.[0] || '' }}
            </option>
          </select>
        </div>
        <div>
          <label class="block text-sm mb-1">外网接口 (WAN)</label>
          <select v-model="form.dev_wan" class="input-field">
            <option value="">— 选择 —</option>
            <option v-for="i in ifaces" :key="'w-' + i.name" :value="i.name">
              {{ i.name }} {{ i.up ? '(UP)' : '' }} {{ i.addrs?.[0] || '' }}
            </option>
          </select>
        </div>
        <div>
          <label class="block text-sm mb-1">主机名</label>
          <input v-model="form.hostname" class="input-field" />
        </div>
        <label class="flex items-center gap-2 text-sm">
          <input v-model="form.enable_dhcp" type="checkbox" />
          引导完成后启用 DHCP（dnsmasq，监听 LAN）
        </label>
      </div>

      <div v-else-if="step === 3" class="space-y-3">
        <div>
          <label class="block text-sm mb-1">策略路由网段（逗号或换行，默认 10.0.0.0/8）</label>
          <textarea v-model="form.policy_routes" class="input-field h-20" />
        </div>
        <div>
          <label class="block text-sm mb-1">共享 SNAT 公网 IP（可选，留空则使用 WAN 口当前 IPv4）</label>
          <input v-model="form.shared_ip" class="input-field" placeholder="留空 = WAN 口 IP" />
        </div>
        <label class="flex items-center gap-2 text-sm">
          <input v-model="form.apply_dataplane" type="checkbox" />
          立即应用 sysctl / TC / nft（推荐）
        </label>
      </div>

      <div v-else class="space-y-2 text-sm text-slate-600">
        <p><strong>用户：</strong>{{ form.admin_user }}</p>
        <p><strong>LAN：</strong>{{ form.dev_lan }} · <strong>WAN：</strong>{{ form.dev_wan }}</p>
        <p><strong>策略网段：</strong>{{ form.policy_routes }}</p>
        <p v-if="form.shared_ip"><strong>共享 IP：</strong>{{ form.shared_ip }}</p>
        <p class="text-xs text-amber-700 bg-amber-50 border border-amber-200 rounded p-2">
          点击「完成设置」后将加载数据面；未填共享 IP 时将自动使用 WAN 口 IPv4 做 SNAT。
        </p>
      </div>

      <p v-if="err" class="text-red-600 text-sm mt-4">{{ err }}</p>

      <div class="flex justify-between mt-8 gap-3">
        <button v-if="step > 0" type="button" class="btn-secondary" :disabled="loading" @click="back">上一步</button>
        <span v-else />
        <button
          v-if="step < steps.length - 1"
          type="button"
          class="btn-primary"
          :disabled="loading"
          @click="next"
        >
          下一步
        </button>
        <button v-else type="button" class="btn-primary" :disabled="loading" @click="finish">
          {{ loading ? '应用中…' : '完成设置' }}
        </button>
      </div>
    </div>
  </div>
</template>
