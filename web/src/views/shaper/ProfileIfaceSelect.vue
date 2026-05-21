<script setup>
const model = defineModel({ type: String, default: '' })
defineProps({
  interfaces: { type: Array, default: () => [] },
  defaultDevice: { type: String, default: '' },
  label: { type: String, default: '网卡' },
  compact: { type: Boolean, default: false },
})
</script>

<template>
  <div>
    <label class="text-xs text-slate-600">{{ label }}</label>
    <select v-model="model" class="input-field font-mono" :class="compact ? 'mt-0.5' : 'mt-1'">
      <option :value="''">默认（{{ defaultDevice }}）</option>
      <option v-for="iface in interfaces" :key="iface.name" :value="iface.name">
        {{ iface.name }}
        <template v-if="iface.addrs?.length"> — {{ iface.addrs.join(', ') }}</template>
        {{ iface.up ? '' : ' (down)' }}
      </option>
    </select>
  </div>
</template>
