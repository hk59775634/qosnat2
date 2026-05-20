<script setup>
defineProps({
  title: String,
  description: String,
})
</script>

<template>
  <div class="mb-6">
    <h2 class="text-xl font-semibold text-slate-900">{{ title }}</h2>
    <p v-if="description" class="text-sm text-slate-600 mt-1">{{ description }}</p>
  </div>
</template>
