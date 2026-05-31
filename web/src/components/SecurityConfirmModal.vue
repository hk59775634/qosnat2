<script setup>
import { nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

const props = defineProps({
  open: { type: Boolean, default: false },
  title: { type: String, required: true },
  body: { type: String, default: '' },
  confirmLabel: { type: String, default: '' },
  submitting: { type: Boolean, default: false },
  error: { type: String, default: '' },
})

const emit = defineEmits(['close', 'confirm'])

const { t } = useI18n()
const password = ref('')
const passwordRef = ref(null)

watch(
  () => props.open,
  (open) => {
    if (!open) {
      password.value = ''
      return
    }
    nextTick(() => passwordRef.value?.focus())
  },
)

function onClose() {
  if (props.submitting) return
  emit('close')
}

function onConfirm() {
  if (props.submitting) return
  emit('confirm', password.value)
}
</script>

<template>
  <Teleport to="body">
    <div
      v-if="open"
      class="fixed inset-0 z-[60] flex items-center justify-center p-4 bg-black/40"
      role="presentation"
      @click.self="onClose"
    >
      <div
        class="bg-white rounded-xl shadow-xl w-full max-w-md border border-slate-200"
        role="dialog"
        aria-modal="true"
        aria-labelledby="security-confirm-modal-title"
        @click.stop
      >
        <div class="flex items-center justify-between px-4 py-3 border-b border-slate-100">
          <h3 id="security-confirm-modal-title" class="font-medium text-slate-900">
            {{ title }}
          </h3>
          <button
            type="button"
            class="text-slate-500 hover:text-slate-800 text-xl leading-none px-2"
            :aria-label="t('common.cancel')"
            :disabled="submitting"
            @click="onClose"
          >
            ×
          </button>
        </div>
        <div class="p-4 space-y-3">
          <p v-if="body" class="text-sm text-slate-600">{{ body }}</p>
          <label class="block text-sm text-slate-700">
            {{ t('system.general.confirmPasswordLabel') }}
            <input
              ref="passwordRef"
              v-model="password"
              type="password"
              autocomplete="current-password"
              class="input-field mt-1 w-full"
              :disabled="submitting"
              @keydown.enter.prevent="onConfirm"
            >
          </label>
          <p v-if="error" class="text-sm text-red-600">{{ error }}</p>
          <div class="flex justify-end gap-2 pt-1">
            <button type="button" class="btn-secondary" :disabled="submitting" @click="onClose">
              {{ t('common.cancel') }}
            </button>
            <button type="button" class="btn-primary" :disabled="submitting" @click="onConfirm">
              {{ submitting ? t('common.processing') : (confirmLabel || t('common.confirm')) }}
            </button>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>
