<script setup>
import { onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'

const { t } = useI18n()
const data = ref(null)
let timer

async function load() {
  data.value = await api.ifaceQueues()
}

onMounted(() => {
  load()
  timer = setInterval(load, 5000)
})
onUnmounted(() => clearInterval(timer))
</script>

<template>
  <div class="page-stack">
    <PageHeader :title="t('network.queues.title')" :description="t('nav.rssQueues')" />

    <div v-if="data" class="grid lg:grid-cols-2 gap-4">
      <section v-for="(iface, key) in { lan: data.lan, wan: data.wan }" :key="key" class="card p-4">
        <h3 class="font-medium mb-2 uppercase text-slate-500">{{ key }}</h3>
        <p class="text-sm font-mono mb-2">{{ iface?.device }}</p>
        <p class="text-sm">
          {{ t('network.queues.channel') }}: {{ iface?.channels || '—' }} · RX: {{ iface?.rx_queues }} · TX: {{ iface?.tx_queues }}
        </p>
        <div class="table-wrap mt-3 max-h-48 overflow-auto">
          <table class="data w-full text-xs">
            <thead>
              <tr><th>IRQ</th><th>{{ t('network.queues.count') }}</th></tr>
            </thead>
            <tbody>
              <tr v-for="irq in iface?.irq_lines?.slice(0, 16) || []" :key="irq.irq">
                <td>{{ irq.irq }}</td>
                <td>{{ irq.count }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </div>
  </div>
</template>
