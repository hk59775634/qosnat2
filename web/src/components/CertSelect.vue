<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/api/client'

const props = defineProps({
  modelValue: { type: String, default: '' },
  allowInherit: { type: Boolean, default: false },
  allowQosnatTls: { type: Boolean, default: false },
  allowOtherSource: { type: Boolean, default: false },
  useQosnatTls: { type: Boolean, default: false },
  disabled: { type: Boolean, default: false },
})

const emit = defineEmits(['update:modelValue', 'update:useQosnatTls'])

const { t } = useI18n()
const list = ref([])
const loading = ref(false)

const mode = computed({
  get() {
    if (props.allowQosnatTls && props.useQosnatTls && !props.modelValue) return '__qosnat__'
    if (props.modelValue) return props.modelValue
    if (props.allowOtherSource) return '__other__'
    if (props.allowInherit) return ''
    return '__custom__'
  },
  set(v) {
    if (v === '__qosnat__') {
      emit('update:modelValue', '')
      emit('update:useQosnatTls', true)
      return
    }
    if (v === '__other__' || v === '__custom__' || v === '') {
      emit('update:useQosnatTls', false)
      emit('update:modelValue', '')
      return
    }
    emit('update:useQosnatTls', false)
    emit('update:modelValue', v)
  },
})

async function load() {
  loading.value = true
  try {
    const data = await api.system.certificates.list()
    list.value = data.certificates || []
  } catch {
    list.value = []
  } finally {
    loading.value = false
  }
}

function labelFor(c) {
  const parts = [c.name || c.id]
  if (c.domains?.length) parts.push(c.domains.join(', '))
  if (c.not_after) parts.push(c.not_after.slice(0, 10))
  if (c.type === 'acme') parts.push('ACME')
  return parts.join(' · ')
}

onMounted(load)
watch(() => props.modelValue, () => {})
defineExpose({ reload: load })
</script>

<template>
  <div class="space-y-2">
    <label class="text-sm block">
      <span>{{ t('certificates.selectLabel') }}</span>
      <select v-model="mode" class="input w-full mt-1" :disabled="disabled || loading">
        <option v-if="allowInherit" value="">{{ t('certificates.inheritGlobal') }}</option>
        <option v-if="allowQosnatTls" value="__qosnat__">{{ t('certificates.useQosnatTls') }}</option>
        <option v-if="allowOtherSource" value="__other__">{{ t('certificates.otherSource') }}</option>
        <option v-if="!allowOtherSource" value="__custom__">{{ t('certificates.customPaths') }}</option>
        <option v-for="c in list" :key="c.id" :value="c.id">{{ labelFor(c) }}</option>
      </select>
    </label>
    <p v-if="loading" class="text-xs text-slate-500">{{ t('common.loading') }}</p>
    <p v-else-if="!list.length" class="text-xs text-slate-500">
      {{ t('certificates.emptyHint') }}
      <router-link to="/system/certificates" class="text-blue-600 hover:underline">{{ t('certificates.openManager') }}</router-link>
    </p>
  </div>
</template>
