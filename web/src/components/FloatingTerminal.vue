<script setup>
import { nextTick, onBeforeUnmount, ref, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import { WebLinksAddon } from '@xterm/addon-web-links'
import '@xterm/xterm/css/xterm.css'

const props = defineProps({
  open: { type: Boolean, default: false },
  shell: { type: String, default: '' },
  title: { type: String, default: '' },
})

const emit = defineEmits(['close'])

const { t } = useI18n()
const containerRef = ref(null)
const status = ref('idle')
const errMsg = ref('')

const termRef = shallowRef(null)
const fitRef = shallowRef(null)
let ws = null
let resizeObserver = null

function terminalWsUrl() {
  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const q = props.shell ? `?shell=${encodeURIComponent(props.shell)}` : ''
  return `${proto}//${window.location.host}/api/v1/diagnostics/terminal${q}`
}

function sendResize() {
  if (!ws || ws.readyState !== WebSocket.OPEN || !fitRef.value || !termRef.value) return
  const { cols, rows } = termRef.value
  ws.send(JSON.stringify({ type: 'resize', cols, rows }))
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
  status.value = 'idle'
}

function connect() {
  disconnect()
  if (!containerRef.value) return
  status.value = 'connecting'
  errMsg.value = ''

  const term = new Terminal({
    cursorBlink: true,
    fontSize: 13,
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
      errMsg.value =
        ev.reason ||
        (ev.code === 1006
          ? t('diagnostics.terminal.connectFailed')
          : t('diagnostics.terminal.closed', { code: ev.code }))
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

function reconnect() {
  connect()
}

function close() {
  disconnect()
  emit('close')
}

watch(
  () => props.open,
  async (v) => {
    if (v) {
      await nextTick()
      connect()
    } else {
      disconnect()
    }
  },
)

onBeforeUnmount(disconnect)
</script>

<template>
  <Teleport to="body">
    <div
      v-if="open"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/40"
      role="dialog"
      aria-modal="true"
      @click.self="close"
    >
      <div class="card w-full max-w-4xl shadow-xl overflow-hidden flex flex-col max-h-[90vh]" @click.stop>
        <div class="flex items-center justify-between gap-2 px-4 py-3 border-b border-slate-200">
          <div>
            <h3 class="font-semibold text-sm">
              {{ title || t('diagnostics.terminal.title') }}
            </h3>
            <p class="text-xs text-slate-500 mt-0.5">
              <span
                class="inline-flex items-center gap-1.5"
                :class="{
                  'text-amber-600': status === 'connecting',
                  'text-emerald-600': status === 'connected',
                  'text-slate-500': status === 'closed' || status === 'idle',
                  'text-red-600': status === 'error',
                }"
              >
                <span
                  class="w-1.5 h-1.5 rounded-full"
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
            </p>
          </div>
          <div class="flex items-center gap-2">
            <button
              v-if="status !== 'connecting' && status !== 'connected'"
              type="button"
              class="btn-secondary text-xs"
              @click="reconnect"
            >
              {{ t('diagnostics.terminal.reconnect') }}
            </button>
            <button type="button" class="text-slate-400 hover:text-slate-600 text-xl leading-none px-1" @click="close">
              ×
            </button>
          </div>
        </div>
        <p v-if="errMsg" class="text-red-600 text-xs px-4 py-2">{{ errMsg }}</p>
        <div class="bg-slate-900 p-2 flex-1 min-h-0">
          <div ref="containerRef" class="floating-terminal-host" tabindex="0" />
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.floating-terminal-host {
  width: 100%;
  height: min(60vh, 480px);
  min-height: 20rem;
}

.floating-terminal-host :deep(.xterm) {
  height: 100%;
}

.floating-terminal-host :deep(.xterm-viewport) {
  overflow-y: auto !important;
}
</style>
