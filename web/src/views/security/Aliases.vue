<script setup>
import { onMounted, ref } from 'vue'
import { api } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'

const aliases = ref([])
const form = ref({ name: '', type: 'ipv4_addr', asn: 0, membersText: '', comment: '' })
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
      type: form.value.type,
      asn: form.value.type === 'asn' ? Number(form.value.asn) : undefined,
      members,
      comment: form.value.comment,
    })
    form.value = { name: '', type: 'ipv4_addr', asn: 0, membersText: '', comment: '' }
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
  <div class="page-stack">
    <PageHeader
      title="防火墙 Aliases"
      description="nft 地址对象组；type=asn 时填写 AS 号与前缀成员（P4），规则中仍用 src_alias 引用。"
    />
    <p v-if="err" class="text-red-600 text-sm mb-2">{{ err }}</p>
    <div class="card card-body mb-0 space-y-3 text-sm">
      <input v-model="form.name" class="input-field font-mono" placeholder="名称 lan_hosts" />
      <div class="flex flex-wrap gap-3 items-center">
        <select v-model="form.type" class="input-field w-36">
          <option value="ipv4_addr">ipv4_addr</option>
          <option value="asn">asn</option>
        </select>
        <input
          v-if="form.type === 'asn'"
          v-model.number="form.asn"
          type="number"
          class="input-field w-32"
          placeholder="AS 号 13335"
        />
      </div>
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
          <tr><th>名称</th><th>类型</th><th>ASN</th><th>成员</th><th></th></tr>
        </thead>
        <tbody>
          <tr v-for="a in aliases" :key="a.name">
            <td class="font-mono">{{ a.name }}</td>
            <td>{{ a.type || 'ipv4_addr' }}</td>
            <td>{{ a.asn || '—' }}</td>
            <td class="text-xs font-mono">{{ (a.members || []).join(', ') }}</td>
            <td><button type="button" class="text-red-600 text-xs" @click="remove(a.name)">删除</button></td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
