<script setup>
defineProps({
  title: String,
  description: String,
})
</script>

<template>
  <div class="mb-2">
    <h2 class="text-lg font-semibold text-slate-900 leading-tight">{{ title }}</h2>
    <p v-if="description" class="text-xs text-slate-500 mt-0.5 leading-snug">{{ description }}</p>
  </div>
</template>
