<script setup>
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { saveLocale } from '@/i18n'

const { locale } = useI18n()

const current = computed({
  get: () => locale.value,
  set: (v) => {
    locale.value = v
    saveLocale(v)
    document.documentElement.lang = v === 'zh' ? 'zh-CN' : 'en'
  },
})

function setLang(code) {
  current.value = code
}
</script>

<template>
  <div class="flex items-center rounded-md border border-white/20 overflow-hidden text-xs">
    <button
      type="button"
      class="px-2 py-1 transition-colors"
      :class="current === 'en' ? 'bg-white/20 text-white' : 'text-blue-100 hover:text-white hover:bg-white/10'"
      @click="setLang('en')"
    >
      EN
    </button>
    <button
      type="button"
      class="px-2 py-1 transition-colors border-l border-white/20"
      :class="current === 'zh' ? 'bg-white/20 text-white' : 'text-blue-100 hover:text-white hover:bg-white/10'"
      @click="setLang('zh')"
    >
      中文
    </button>
  </div>
</template>
