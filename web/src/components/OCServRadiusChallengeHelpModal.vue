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

const flowSteps = tm('ocserv.radiusChallengeHelpFlowSteps')
const challengeSections = tm('ocserv.radiusChallengeHelpChallengeSections')
const phase2Sections = tm('ocserv.radiusChallengeHelpPhase2Sections')
const pitfalls = tm('ocserv.radiusChallengeHelpPitfalls')
</script>

<template>
  <button type="button" :class="buttonClass" @click="open = true">
    {{ t('ocserv.radiusChallengeHelpBtn') }}
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
            <h3 class="font-medium text-slate-900">{{ t('ocserv.radiusChallengeHelpTitle') }}</h3>
            <p class="text-xs text-slate-500 mt-1">{{ t('ocserv.radiusChallengeHelpIntro') }}</p>
          </div>
          <button type="button" class="text-slate-500 hover:text-slate-800 text-xl leading-none" @click="close">
            ×
          </button>
        </div>

        <div class="overflow-y-auto px-4 py-3 space-y-4 text-sm">
          <p class="text-sky-800 bg-sky-50 border border-sky-200 rounded-md px-3 py-2 text-xs">
            {{ t('ocserv.radiusChallengeHelpScope') }}
          </p>

          <section class="space-y-2">
            <h4 class="font-medium text-slate-800">{{ t('ocserv.radiusChallengeHelpFlowTitle') }}</h4>
            <ol class="list-decimal list-inside space-y-2 text-xs text-slate-700">
              <li v-for="(item, i) in flowSteps" :key="i">
                <span class="font-medium text-slate-800">{{ item.step }}</span>
                <span class="text-slate-600"> — {{ item.desc }}</span>
              </li>
            </ol>
          </section>

          <section v-for="(sec, i) in challengeSections" :key="'c-' + i" class="space-y-2">
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
            <h4 class="font-medium text-slate-800">{{ t('ocserv.radiusChallengeHelpPhase2Title') }}</h4>
            <p class="text-slate-600 text-xs">{{ t('ocserv.radiusChallengeHelpPhase2Intro') }}</p>
            <div
              v-for="(sec, i) in phase2Sections"
              :key="'p2-' + i"
              class="overflow-x-auto border border-slate-200 rounded-md"
            >
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
            <h4 class="font-medium text-slate-800">{{ t('ocserv.radiusChallengeHelpExampleTitle') }}</h4>
            <p class="text-xs text-slate-500">{{ t('ocserv.radiusChallengeHelpExampleNote') }}</p>
            <h5 class="text-xs font-medium text-slate-700">{{ t('ocserv.radiusChallengeHelpExampleChallengeLabel') }}</h5>
            <pre class="text-xs font-mono bg-slate-900 text-slate-100 rounded-md p-3 overflow-x-auto whitespace-pre">{{ t('ocserv.radiusChallengeHelpExampleChallenge') }}</pre>
            <h5 class="text-xs font-medium text-slate-700 pt-1">{{ t('ocserv.radiusChallengeHelpExampleAcceptLabel') }}</h5>
            <pre class="text-xs font-mono bg-slate-50 border border-slate-200 rounded-md p-3 overflow-x-auto whitespace-pre">{{ t('ocserv.radiusChallengeHelpExampleAccept') }}</pre>
          </section>

          <section class="space-y-2">
            <h4 class="font-medium text-slate-800">{{ t('ocserv.radiusChallengeHelpPitfallsTitle') }}</h4>
            <ul class="list-disc list-inside text-xs text-slate-700 space-y-1">
              <li v-for="(line, i) in pitfalls" :key="i">{{ line }}</li>
            </ul>
          </section>
        </div>

        <div class="px-4 py-3 border-t border-slate-200 flex justify-end">
          <button type="button" class="btn-secondary text-sm" @click="close">{{ t('common.close') }}</button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
