<script setup>
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useNotificationsPollOnMount } from '@/composables/useNotifications'

const { t } = useI18n()
const router = useRouter()
const { items, unread, open, refresh, dismiss, dismissAll } = useNotificationsPollOnMount()

function levelClass(level) {
  switch (level) {
    case 'success':
      return 'border-l-green-500 bg-green-50'
    case 'warn':
      return 'border-l-amber-500 bg-amber-50'
    case 'error':
      return 'border-l-red-500 bg-red-50'
    default:
      return 'border-l-blue-500 bg-blue-50'
  }
}

function go(link) {
  if (!link) return
  const path = link.startsWith('#') ? link.slice(1) : link
  open.value = false
  router.push(path)
}

function toggle() {
  open.value = !open.value
  if (open.value) refresh()
}
</script>

<template>
  <div class="relative">
    <button
      type="button"
      class="relative text-sm text-blue-100 hover:text-white px-2 py-1 rounded hover:bg-white/10"
      :aria-label="t('notifications.title')"
      @click="toggle"
    >
      <svg class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6 6 0 10-12 0v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9"
        />
      </svg>
      <span
        v-if="unread > 0"
        class="absolute -top-0.5 -right-0.5 min-w-[1.1rem] h-[1.1rem] px-1 rounded-full bg-red-500 text-white text-[10px] leading-tight flex items-center justify-center font-medium"
      >
        {{ unread > 9 ? '9+' : unread }}
      </span>
    </button>

    <div
      v-if="open"
      class="fixed inset-0 z-40"
      aria-hidden="true"
      @click="open = false"
    />
    <div
      v-if="open"
      class="absolute right-0 top-full mt-2 z-50 w-[min(24rem,calc(100vw-2rem))] max-h-[min(24rem,70vh)] overflow-hidden rounded-lg shadow-xl border border-slate-200 bg-white text-slate-800 flex flex-col"
    >
      <div class="flex items-center justify-between px-3 py-2 border-b border-slate-100 bg-slate-50">
        <span class="text-sm font-semibold">{{ t('notifications.title') }}</span>
        <button
          v-if="items.length"
          type="button"
          class="text-xs text-slate-500 hover:text-slate-800"
          @click="dismissAll"
        >
          {{ t('notifications.clearAll') }}
        </button>
      </div>
      <ul class="overflow-y-auto flex-1 p-2 space-y-2">
        <li v-if="!items.length" class="text-xs text-slate-500 text-center py-6">
          {{ t('notifications.empty') }}
        </li>
        <li
          v-for="n in items"
          :key="n.id"
          class="rounded border-l-4 p-2 text-xs shadow-sm"
          :class="levelClass(n.level)"
        >
          <div class="flex justify-between gap-2 items-start">
            <p class="font-medium text-slate-800">{{ n.title }}</p>
            <button
              type="button"
              class="text-slate-400 hover:text-slate-700 shrink-0"
              :aria-label="t('notifications.dismiss')"
              @click="dismiss(n.id)"
            >
              ×
            </button>
          </div>
          <p class="text-slate-600 mt-1 whitespace-pre-wrap">{{ n.message }}</p>
          <div class="mt-2 flex gap-2">
            <button
              v-if="n.link"
              type="button"
              class="text-blue-600 hover:underline"
              @click="go(n.link)"
            >
              {{ t('notifications.view') }}
            </button>
            <span class="text-slate-400">{{ n.created_at?.slice(0, 16)?.replace('T', ' ') }}</span>
          </div>
        </li>
      </ul>
    </div>
  </div>
</template>
