<script setup>
import { nextTick, onBeforeUnmount, onMounted, ref, shallowRef } from 'vue'
import { useI18n } from 'vue-i18n'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { WebLinksAddon } from '@xterm/addon-web-links'
import '@xterm/xterm/css/xterm.css'
import { api } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'

const { t } = useI18n()

const containerRef = ref(null)
const status = ref('idle')
const errMsg = ref('')
const enabled = ref(false)
const checked = ref(false)
const grantModalOpen = ref(false)
const grantPassword = ref('')
const grantModalErr = ref('')
const grantSubmitting = ref(false)
const grantPasswordRef = ref(null)
const granted = ref(false)

const termRef = shallowRef(null)
const fitRef = shallowRef(null)
let ws = null
let resizeObserver = null

function terminalWsUrl() {
  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${proto}//${window.location.host}/api/v1/diagnostics/terminal`
}

function sendResize() {
  if (!ws || ws.readyState !== WebSocket.OPEN || !fitRef.value || !termRef.value) return
  const { cols, rows } = termRef.value
  ws.send(JSON.stringify({ type: 'resize', cols, rows }))
}

function connect() {
  if (!enabled.value || !granted.value) return
  disconnect()
  status.value = 'connecting'
  errMsg.value = ''

  const term = new Terminal({
    cursorBlink: true,
    fontSize: 14,
    fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace',
    theme: {
      background: '#0f172a',
      foreground: '#e2e8f0',
      cursor: '#38bdf8',
      selectionBackground: '#334155',
    },
  })
  const fitAddon = new FitAddon()
  term.loadAddon(fitAddon)
  term.loadAddon(new WebLinksAddon())
  term.open(containerRef.value)
  fitAddon.fit()

  termRef.value = term
  fitRef.value = fitAddon

  ws = new WebSocket(terminalWsUrl())
  ws.binaryType = 'arraybuffer'

  ws.onopen = () => {
    status.value = 'connected'
    sendResize()
    term.focus()
  }

  ws.onmessage = (ev) => {
    if (typeof ev.data === 'string') {
      term.write(ev.data)
    } else if (ev.data instanceof ArrayBuffer) {
      term.write(new Uint8Array(ev.data))
    }
  }

  ws.onerror = () => {
    if (status.value !== 'connected') {
      errMsg.value = t('diagnostics.terminal.connectFailed')
      status.value = 'error'
    }
  }

  ws.onclose = (ev) => {
    if (status.value === 'connecting') {
      if (ev.code === 1006) {
        errMsg.value = t('diagnostics.terminal.connectFailed')
      } else if (ev.code === 1000) {
        errMsg.value = t('diagnostics.terminal.closed', { code: ev.code })
      } else {
        errMsg.value =
          ev.reason ||
          (ev.code === 403
            ? t('diagnostics.terminal.grantRequired')
            : t('diagnostics.terminal.closed', { code: ev.code }))
      }
      status.value = 'error'
      granted.value = false
    } else if (status.value === 'connected') {
      term.writeln('')
      term.writeln(`\r\n\x1b[33m[${t('diagnostics.terminal.sessionEnded')}]\x1b[0m`)
      status.value = 'closed'
      granted.value = false
    }
  }

  term.onData((data) => {
    if (ws?.readyState === WebSocket.OPEN) ws.send(data)
  })

  resizeObserver = new ResizeObserver(() => {
    fitAddon.fit()
    sendResize()
  })
  resizeObserver.observe(containerRef.value)
}

function disconnect() {
  resizeObserver?.disconnect()
  resizeObserver = null
  if (ws) {
    ws.onclose = null
    ws.close()
    ws = null
  }
  termRef.value?.dispose()
  termRef.value = null
  fitRef.value = null
}

function openGrantModal() {
  grantModalErr.value = ''
  grantPassword.value = ''
  grantModalOpen.value = true
  nextTick(() => grantPasswordRef.value?.focus())
}

function closeGrantModal() {
  if (grantSubmitting.value) return
  grantModalOpen.value = false
  grantPassword.value = ''
  grantModalErr.value = ''
}

async function confirmGrant() {
  grantModalErr.value = ''
  if (!grantPassword.value) {
    grantModalErr.value = t('diagnostics.terminal.grantRequired')
    return
  }
  grantSubmitting.value = true
  try {
    await api.diagnostics.terminalGrant({ current_password: grantPassword.value })
    granted.value = true
    grantModalOpen.value = false
    grantPassword.value = ''
    await nextTick()
    connect()
  } catch (e) {
    grantModalErr.value = e.data?.error || e.message
  } finally {
    grantSubmitting.value = false
  }
}

function reconnect() {
  openGrantModal()
}

onMounted(async () => {
  try {
    const h = await api.health()
    enabled.value = !!h.diagnostics_terminal_enabled
  } catch {
    enabled.value = false
  }
  checked.value = true
  if (enabled.value) {
    status.value = 'idle'
    openGrantModal()
  } else {
    status.value = 'error'
    errMsg.value = t('diagnostics.terminal.disabled')
  }
})
onBeforeUnmount(disconnect)
</script>

<template>
  <div class="page-stack terminal-page">
    <PageHeader
      :title="t('diagnostics.terminal.title')"
      :description="t('diagnostics.terminal.description')"
    />

    <div
      v-if="enabled"
      class="rounded-lg border border-red-300 bg-red-50 text-red-900 text-sm p-3 mb-3"
      role="alert"
    >
      <p class="font-semibold">{{ t('diagnostics.terminal.dangerTitle') }}</p>
      <p class="text-xs mt-1 text-red-800">{{ t('diagnostics.terminal.dangerBody') }}</p>
    </div>

    <div
      v-else-if="checked"
      class="rounded-lg border border-amber-200 bg-amber-50 text-amber-900 text-sm p-3 mb-3"
    >
      {{ t('diagnostics.terminal.disabled') }}
    </div>

    <div v-if="enabled" class="card card-body mb-3 flex flex-wrap items-center gap-3 text-sm">
      <span
        class="inline-flex items-center gap-2"
        :class="{
          'text-amber-600': status === 'connecting',
          'text-emerald-600': status === 'connected',
          'text-slate-500': status === 'closed' || status === 'idle',
          'text-red-600': status === 'error',
        }"
      >
        <span
          class="w-2 h-2 rounded-full"
          :class="{
            'bg-amber-500 animate-pulse': status === 'connecting',
            'bg-emerald-500': status === 'connected',
            'bg-slate-400': status === 'closed' || status === 'idle',
            'bg-red-500': status === 'error',
          }"
        />
        {{
          status === 'idle'
            ? t('diagnostics.terminal.status.closed')
            : t(`diagnostics.terminal.status.${status}`)
        }}
      </span>
      <button
        v-if="status !== 'connecting' && status !== 'connected'"
        type="button"
        class="btn-secondary text-xs"
        @click="reconnect"
      >
        {{ t('diagnostics.terminal.reconnect') }}
      </button>
      <p class="text-xs text-slate-500 flex-1 min-w-[12rem]">
        {{ t('diagnostics.terminal.hint') }}
      </p>
    </div>

    <p v-if="errMsg" class="text-red-600 text-sm mb-2">{{ errMsg }}</p>

    <div v-if="enabled && granted" class="card terminal-card overflow-hidden">
      <div ref="containerRef" class="terminal-host" tabindex="0" />
    </div>

    <div
      v-if="grantModalOpen"
      class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4"
      role="dialog"
      aria-modal="true"
    >
      <div class="card w-full max-w-md p-6 shadow-xl">
        <h2 class="text-lg font-semibold text-red-700">{{ t('diagnostics.terminal.grantModalTitle') }}</h2>
        <p class="text-sm text-slate-600 mt-2">{{ t('diagnostics.terminal.grantModalBody') }}</p>
        <label class="block mt-4 text-sm font-medium">
          {{ t('diagnostics.terminal.grantPasswordLabel') }}
          <input
            ref="grantPasswordRef"
            v-model="grantPassword"
            type="password"
            autocomplete="current-password"
            class="input-field mt-1 w-full"
            :disabled="grantSubmitting"
            @keyup.enter="confirmGrant"
          />
        </label>
        <p v-if="grantModalErr" class="text-sm text-red-600 mt-2">{{ grantModalErr }}</p>
        <div class="flex justify-end gap-2 mt-6">
          <button type="button" class="btn-secondary" :disabled="grantSubmitting" @click="closeGrantModal">
            {{ t('common.cancel') }}
          </button>
          <button type="button" class="btn-primary" :disabled="grantSubmitting" @click="confirmGrant">
            {{ grantSubmitting ? t('common.processing') : t('diagnostics.terminal.grantConfirm') }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.terminal-page {
  min-height: calc(100vh - 8rem);
}

.terminal-card {
  flex: 1;
  min-height: 28rem;
  background: #0f172a;
  padding: 0.5rem;
}

.terminal-host {
  width: 100%;
  height: min(70vh, 640px);
  min-height: 24rem;
}

.terminal-host :deep(.xterm) {
  height: 100%;
}

.terminal-host :deep(.xterm-viewport) {
  overflow-y: auto !important;
}
</style>
