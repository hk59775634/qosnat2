<script setup>
defineProps({
  title: String,
  description: String,
  ok: String,
  err: String,
  warn: String,
})
</script>

<template>
  <div class="page-header-block">
    <div class="mb-2">
      <h2 class="text-lg font-semibold text-slate-900 leading-tight">{{ title }}</h2>
      <p v-if="description" class="text-xs text-slate-500 mt-0.5 leading-snug">{{ description }}</p>
      <div v-else-if="$slots.description" class="text-xs text-slate-500 mt-0.5 leading-snug">
        <slot name="description" />
      </div>
    </div>
    <div
      v-if="ok || err || warn"
      class="page-alerts"
      role="status"
    >
      <p v-if="ok" class="page-alerts__ok">{{ ok }}</p>
      <p v-if="warn" class="page-alerts__warn">{{ warn }}</p>
      <p v-if="err" class="page-alerts__err">{{ err }}</p>
    </div>
  </div>
</template>
