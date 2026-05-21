<script setup>
import { onMounted, ref } from 'vue'
import { api } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'

const aliases = ref([])
const form = ref({ name: '', membersText: '', comment: '' })
const err = ref('')

async function load() {
  const d = await api.firewall.aliases.list()
  aliases.value = d.aliases || []
}

async function add() {
  err.value = ''
  const members = form.value.membersText.split(/[\n,]+/).map((s) => s.trim()).filter(Boolean)
  try {
    await api.firewall.aliases.add({
      name: form.value.name,
      type: 'ipv4_addr',
      members,
      comment: form.value.comment,
    })
    form.value = { name: '', membersText: '', comment: '' }
    await load()
  } catch (e) {
    err.value = e.message
  }
}

async function remove(name) {
  if (!confirm(`删除别名 ${name}？`)) return
  await api.firewall.aliases.del(name)
  await load()
}

onMounted(load)
</script>

<template>
  <div>
    <PageHeader
      title="防火墙 Aliases"
      description="nft 地址对象组，可在防火墙规则中通过 src_alias / dst_alias 引用（@alias_名称）。"
    />
    <p v-if="err" class="text-red-600 text-sm mb-2">{{ err }}</p>
    <div class="card p-4 mb-6 max-w-xl space-y-3 text-sm">
      <input v-model="form.name" class="input-field font-mono" placeholder="名称 lan_hosts" />
      <textarea
        v-model="form.membersText"
        class="input-field font-mono text-xs h-24"
        placeholder="成员，每行一个 CIDR&#10;10.0.0.0/8"
      />
      <input v-model="form.comment" class="input-field" placeholder="备注" />
      <button type="button" class="btn-primary" @click="add">添加并应用 nft</button>
    </div>
    <div class="card overflow-x-auto">
      <table class="data w-full text-sm">
        <thead>
          <tr><th>名称</th><th>成员</th><th></th></tr>
        </thead>
        <tbody>
          <tr v-for="a in aliases" :key="a.name">
            <td class="font-mono">{{ a.name }}</td>
            <td class="text-xs font-mono">{{ (a.members || []).join(', ') }}</td>
            <td><button type="button" class="text-red-600 text-xs" @click="remove(a.name)">删除</button></td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
