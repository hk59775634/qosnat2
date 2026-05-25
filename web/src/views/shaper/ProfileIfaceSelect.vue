<script setup>
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const model = defineModel({ type: String, default: '' })
const props = defineProps({
  interfaces: { type: Array, default: () => [] },
  defaultDevice: { type: String, default: '' },
  label: { type: String, default: '' },
  compact: { type: Boolean, default: false },
})
</script>

<template>
  <div>
    <label class="text-xs text-slate-600">{{ label || t('shaper.ifaceSelect') }}</label>
    <select v-model="model" class="input-field font-mono" :class="compact ? 'mt-0.5' : 'mt-1'">
      <option :value="''">{{ t('shaper.ifaceDefault', { dev: defaultDevice }) }}</option>
      <option v-for="iface in interfaces" :key="iface.name" :value="iface.name">
        {{ iface.name }}
        <template v-if="iface.addrs?.length"> — {{ iface.addrs.join(', ') }}</template>
        {{ iface.up ? '' : ' (down)' }}
      </option>
    </select>
  </div>
</template>
