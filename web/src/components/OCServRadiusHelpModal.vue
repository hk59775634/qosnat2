<script setup>
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'

defineProps({
  buttonClass: { type: String, default: 'text-blue-600 text-sm underline' },
})

const { t, tm } = useI18n()
const open = ref(false)

function close() {
  open.value = false
}

const sections = tm('ocserv.radiusHelpSections')
</script>

<template>
  <button type="button" :class="buttonClass" @click="open = true">
    {{ t('ocserv.radiusHelpBtn') }}
  </button>

  <Teleport to="body">
    <div
      v-if="open"
      class="fixed inset-0 z-[200] flex items-center justify-center p-4 bg-black/40"
      @click.self="close"
    >
      <div
        role="dialog"
        aria-modal="true"
        class="bg-white rounded-lg shadow-xl max-w-3xl w-full max-h-[85vh] flex flex-col"
        @click.stop
      >
        <div class="flex items-start justify-between gap-3 px-4 py-3 border-b border-slate-200">
          <div>
            <h3 class="font-medium text-slate-900">{{ t('ocserv.radiusHelpTitle') }}</h3>
            <p class="text-xs text-slate-500 mt-1">{{ t('ocserv.radiusHelpIntro') }}</p>
          </div>
          <button type="button" class="text-slate-500 hover:text-slate-800 text-xl leading-none" @click="close">
            ×
          </button>
        </div>

        <div class="overflow-y-auto px-4 py-3 space-y-4 text-sm">
          <p class="text-amber-800 bg-amber-50 border border-amber-200 rounded-md px-3 py-2 text-xs">
            {{ t('ocserv.radiusHelpGroupconfig') }}
          </p>
          <p class="text-slate-600 text-xs">{{ t('ocserv.radiusHelpRateNote') }}</p>

          <section v-for="(sec, i) in sections" :key="i" class="space-y-2">
            <h4 class="font-medium text-slate-800">{{ sec.title }}</h4>
            <div class="overflow-x-auto border border-slate-200 rounded-md">
              <table class="data w-full text-xs">
                <thead>
                  <tr>
                    <th class="whitespace-nowrap">{{ t('ocserv.radiusHelpColAttr') }}</th>
                    <th>{{ t('ocserv.radiusHelpColDesc') }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="(row, j) in sec.rows" :key="j">
                    <td class="font-mono text-blue-900 align-top whitespace-nowrap">{{ row.attr }}</td>
                    <td class="text-slate-700">{{ row.desc }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </section>

          <section class="space-y-2">
            <h4 class="font-medium text-slate-800">{{ t('ocserv.radiusHelpExampleTitle') }}</h4>
            <pre class="text-xs font-mono bg-slate-900 text-slate-100 rounded-md p-3 overflow-x-auto whitespace-pre">{{ t('ocserv.radiusHelpExampleFreeradius') }}</pre>
            <pre class="text-xs font-mono bg-slate-50 border border-slate-200 rounded-md p-3 overflow-x-auto whitespace-pre">{{ t('ocserv.radiusHelpExampleRate') }}</pre>
          </section>
        </div>

        <div class="px-4 py-3 border-t border-slate-200 flex justify-end">
          <button type="button" class="btn-secondary text-sm" @click="close">{{ t('common.close') }}</button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
