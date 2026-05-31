<script setup>
import { onBeforeUnmount, onMounted, ref, shallowRef } from 'vue'
import { useI18n } from 'vue-i18n'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { WebLinksAddon } from '@xterm/addon-web-links'
import '@xterm/xterm/css/xterm.css'
import { api } from '@/api/client'
import PageHeader from '@/components/PageHeader.vue'

const { t } = useI18n()

const containerRef = ref(null)
const status = ref('connecting')
const errMsg = ref('')
const enabled = ref(false)
const checked = ref(false)

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
  if (!enabled.value) return
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
            ? t('diagnostics.terminal.disabled')
            : t('diagnostics.terminal.closed', { code: ev.code }))
      }
      status.value = 'error'
    } else if (status.value === 'connected') {
      term.writeln('')
      term.writeln(`\r\n\x1b[33m[${t('diagnostics.terminal.sessionEnded')}]\x1b[0m`)
      status.value = 'closed'
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

function reconnect() {
  connect()
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
    connect()
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
          'text-slate-500': status === 'closed',
          'text-red-600': status === 'error',
        }"
      >
        <span
          class="w-2 h-2 rounded-full"
          :class="{
            'bg-amber-500 animate-pulse': status === 'connecting',
            'bg-emerald-500': status === 'connected',
            'bg-slate-400': status === 'closed',
            'bg-red-500': status === 'error',
          }"
        />
        {{ t(`diagnostics.terminal.status.${status}`) }}
      </span>
      <button
        v-if="status !== 'connecting'"
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

    <div v-if="enabled" class="card terminal-card overflow-hidden">
      <div ref="containerRef" class="terminal-host" tabindex="0" />
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
