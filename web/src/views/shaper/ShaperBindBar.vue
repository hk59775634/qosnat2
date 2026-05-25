<script setup>
import { useI18n } from 'vue-i18n'

defineProps({
  bindDevice: { type: String, default: '' },
  devLan: { type: String, default: '' },
  devWan: { type: String, default: '' },
  attached: { type: Array, default: () => [] },
  embedded: { type: Boolean, default: false },
})

const { t } = useI18n()
</script>

<template>
  <div
    :class="
      embedded
        ? 'text-xs sm:text-sm'
        : 'card card-body mb-2 text-xs sm:text-sm bg-slate-50'
    "
  >
    <div class="flex flex-wrap gap-x-4 gap-y-0.5">
      <span>
        {{ t('shaper.bindBar') }}
        <strong class="font-mono text-blue-800">{{ bindDevice || devLan }}</strong>
      </span>
      <span class="text-slate-500">LAN <span class="font-mono">{{ devLan }}</span></span>
      <span class="text-slate-500">WAN <span class="font-mono">{{ devWan }}</span></span>
      <span v-if="attached?.length" class="text-slate-600">
        BPF：<span class="font-mono">{{ attached.join(', ') }}</span>
      </span>
    </div>
  </div>
</template>
