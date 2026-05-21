<script setup>
import { computed, onMounted, ref } from 'vue'
import { ApiReference } from '@scalar/api-reference'
import '@scalar/api-reference/style.css'

const specContent = ref('')
const ready = ref(false)
const loadErr = ref('')

onMounted(async () => {
  try {
    const res = await fetch('/openapi.yaml', { credentials: 'same-origin' })
    if (!res.ok) {
      loadErr.value = `无法加载 OpenAPI (${res.status})`
    } else {
      specContent.value = await res.text()
    }
  } catch (e) {
    loadErr.value = e.message || '无法加载 OpenAPI'
  } finally {
    ready.value = true
  }
})

const configuration = computed(() => {
  if (!specContent.value) return null
  return {
    content: specContent.value,
    layout: 'modern',
    showSidebar: true,
    agent: { disabled: true },
    telemetry: false,
    authentication: {
      preferredSecurityScheme: 'sessionAuth',
    },
    persistAuth: true,
    darkMode: false,
    hideDownloadButton: false,
    customCss: `
      .references-sidebar { --refs-sidebar-width: 288px; --scalar-sidebar-width: 288px; }
      .references-layout .t-doc__sidebar.sticky { display: flex !important; }
      @media (max-width: 1023px) {
        .references-layout .t-doc__sidebar.sticky { display: none !important; }
      }
    `,
  }
})
</script>

<template>
  <div class="api-docs-wrap">
    <div class="api-docs-header shrink-0 px-1 pb-2">
      <h2 class="text-xl font-semibold">API 文档 (Scalar)</h2>
      <p class="text-sm text-slate-600 mt-1">
        OpenAPI:
        <a href="/openapi.yaml" class="text-blue-600 hover:underline" target="_blank">openapi.yaml</a>
        — 试用前请先登录以携带 Cookie <code class="text-xs bg-slate-100 px-1 rounded">qosnat_sess</code>。
      </p>
      <p v-if="loadErr" class="text-red-600 text-sm mt-1">{{ loadErr }}</p>
    </div>
    <div v-if="ready && configuration" class="scalar-host">
      <ApiReference :configuration="configuration" />
    </div>
    <p v-else-if="ready" class="text-sm text-slate-500 p-4">OpenAPI 为空，无法渲染文档。</p>
    <p v-else class="text-sm text-slate-500 p-4">加载 API 文档…</p>
  </div>
</template>

<style scoped>
.api-docs-wrap {
  display: flex;
  flex-direction: column;
  height: calc(100vh - 7.5rem);
  min-height: 560px;
  min-width: 0;
}

.scalar-host {
  flex: 1;
  min-height: 0;
  min-width: 0;
  position: relative;
  isolation: isolate;
  border: 1px solid rgb(226 232 240);
  border-radius: 0.5rem;
  overflow: hidden;
  --scalar-sidebar-width: 288px;
  --refs-sidebar-width: 288px;
}

/* 嵌入时 Tailwind lg: 可能未作用于 Scalar 组件，强制桌面侧栏可见 */
.scalar-host :deep(.references-layout.references-sidebar) {
  --refs-sidebar-width: 288px;
}

/* Scalar 使用 Tailwind `hidden lg:flex`，嵌入 qosnat 后 lg: 常不生效，强制显示桌面导航列 */
.scalar-host :deep(.t-doc__sidebar.sticky) {
  display: flex !important;
  flex-direction: column !important;
  width: 288px !important;
  min-width: 288px !important;
  max-height: 100% !important;
  pointer-events: auto !important;
  z-index: 20 !important;
}

/* 移动端抽屉内第二条 sidebar 无 sticky，保持由 Scalar 控制 */
.scalar-host :deep(.t-doc__header .t-doc__sidebar) {
  display: flex !important;
  width: 100% !important;
  min-width: 0 !important;
}

.scalar-host :deep(.scalar-app),
.scalar-host :deep(.references-layout) {
  height: 100% !important;
  min-height: 0 !important;
}

.scalar-host :deep(.t-doc__sidebar) {
  overflow-y: auto !important;
}
</style>
