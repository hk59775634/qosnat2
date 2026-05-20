<script setup>
import { onMounted, ref } from 'vue'
import { api } from '@/api/client'

const routes = ref([])
const shared = ref([])
const newCidr = ref('10.0.0.0/8')
const newIP = ref('')
const msg = ref('')
const err = ref('')

async function load() {
  routes.value = await api.policyRoutes.list()
  shared.value = await api.sharedIPs.list()
}

async function addRoute() {
  err.value = ''
  try {
    await api.policyRoutes.add(newCidr.value)
    msg.value = '已添加策略网段'
    await load()
  } catch (e) {
    err.value = e.message
  }
}

async function delRoute(cidr) {
  if (!confirm(`删除策略网段 ${cidr}?`)) return
  await api.policyRoutes.del(cidr)
  await load()
}

async function addIP() {
  err.value = ''
  try {
    await api.sharedIPs.add(newIP.value)
    msg.value = '已添加共享 IP'
    newIP.value = ''
    await load()
  } catch (e) {
    err.value = e.message
  }
}

onMounted(load)
</script>

<template>
  <div>
    <h2 class="text-xl font-semibold mb-4">Outbound NAT</h2>
    <p v-if="msg" class="text-green-700 text-sm mb-2">{{ msg }}</p>
    <p v-if="err" class="text-red-600 text-sm mb-2">{{ err }}</p>

    <div class="grid lg:grid-cols-2 gap-6">
      <section class="card p-4">
        <h3 class="font-medium mb-3">策略路由网段 (SNAT 范围)</h3>
        <ul class="mb-4 space-y-1">
          <li v-for="c in routes" :key="c" class="flex justify-between items-center text-sm font-mono">
            {{ c }}
            <button type="button" class="text-red-600 text-xs" @click="delRoute(c)">删除</button>
          </li>
        </ul>
        <div class="flex gap-2">
          <input v-model="newCidr" class="input-field" placeholder="10.0.0.0/8" />
          <button type="button" class="btn-primary shrink-0" @click="addRoute">添加</button>
        </div>
      </section>

      <section class="card p-4">
        <h3 class="font-medium mb-3">共享公网 IP 池</h3>
        <ul class="mb-4 space-y-1">
          <li v-for="ip in shared" :key="ip" class="font-mono text-sm">{{ ip }}</li>
        </ul>
        <div class="flex gap-2">
          <input v-model="newIP" class="input-field" placeholder="157.15.107.249" />
          <button type="button" class="btn-primary shrink-0" @click="addIP">添加</button>
        </div>
      </section>
    </div>
  </div>
</template>
