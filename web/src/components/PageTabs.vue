<script setup>
defineProps({
  modelValue: { type: String, required: true },
  tabs: { type: Array, required: true },
})

defineEmits(['update:modelValue'])
</script>

<template>
  <div class="page-tabs" role="tablist">
    <button
      v-for="tab in tabs"
      :key="tab.id"
      type="button"
      role="tab"
      class="page-tabs__btn"
      :class="{ 'page-tabs__btn--active': modelValue === tab.id }"
      :aria-selected="modelValue === tab.id"
      @click="$emit('update:modelValue', tab.id)"
    >
      {{ tab.label }}
    </button>
  </div>
</template>
