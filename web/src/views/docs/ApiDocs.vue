<script setup>
import { ApiReference } from '@scalar/api-reference'
import '@scalar/api-reference/style.css'

const configuration = {
  spec: {
    url: '/openapi.yaml',
  },
  authentication: {
    preferredSecurityScheme: 'sessionAuth',
  },
  darkMode: false,
  layout: 'modern',
  hideDownloadButton: false,
}
</script>

<template>
  <div class="api-docs-wrap -mx-2 sm:-mx-0">
    <div class="px-1 mb-3">
      <h2 class="text-xl font-semibold">API 文档 (Scalar)</h2>
      <p class="text-sm text-slate-600 mt-1">
        OpenAPI:
        <a href="/openapi.yaml" class="text-blue-600 hover:underline" target="_blank">openapi.yaml</a>
        — 试用前请先登录以携带 Cookie <code class="text-xs bg-slate-100 px-1 rounded">qosnat_sess</code>。
      </p>
    </div>
    <div class="scalar-panel rounded-lg border border-slate-200 overflow-hidden min-h-[75vh]">
      <ApiReference :configuration="configuration" />
    </div>
  </div>
</template>

<style scoped>
.api-docs-wrap :deep(.scalar-app) {
  min-height: 75vh;
}
</style>
