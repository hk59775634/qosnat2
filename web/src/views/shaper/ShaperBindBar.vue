<script setup>
defineProps({
  bindDevice: { type: String, default: '' },
  devLan: { type: String, default: '' },
  devWan: { type: String, default: '' },
  attached: { type: Array, default: () => [] },
})
</script>

<template>
  <div class="card p-3 mb-4 text-sm bg-slate-50 border border-slate-200">
    <div class="flex flex-wrap gap-x-6 gap-y-1">
      <span>
        默认整形网卡：
        <strong class="font-mono text-blue-800">{{ bindDevice || devLan }}</strong>
        <span v-if="bindDevice === devLan" class="text-slate-500">（内网 DEV_LAN）</span>
      </span>
      <span class="text-slate-500">LAN <span class="font-mono">{{ devLan }}</span></span>
      <span class="text-slate-500">WAN <span class="font-mono">{{ devWan }}</span></span>
      <span v-if="attached?.length" class="text-slate-600">
        BPF 已附加：
        <span class="font-mono">{{ attached.join(', ') }}</span>
      </span>
    </div>
  </div>
</template>
