<script setup>
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { GITHUB_REPO_SLUG, GITHUB_REPO_URL } from '@/config/project'

const props = defineProps({
  /** sidebar | inline | footer */
  variant: { type: String, default: 'inline' },
  /** 仅图标（侧栏收起时） */
  iconOnly: { type: Boolean, default: false },
})

const { t } = useI18n()

const label = computed(() => {
  if (props.iconOnly) return t('common.githubRepoTitle', { repo: GITHUB_REPO_SLUG })
  return props.variant === 'footer' ? t('common.githubViewRepo') : GITHUB_REPO_SLUG
})
</script>

<template>
  <a
    :href="GITHUB_REPO_URL"
    target="_blank"
    rel="noopener noreferrer"
    class="github-project-link"
    :class="[
      variant === 'sidebar' ? 'github-project-link--sidebar' : '',
      variant === 'footer' ? 'github-project-link--footer' : '',
      variant === 'inline' ? 'github-project-link--inline' : '',
      iconOnly ? 'github-project-link--icon-only' : '',
    ]"
    :title="t('common.githubRepoTitle', { repo: GITHUB_REPO_SLUG })"
  >
    <svg class="github-project-link__icon" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
      <path
        d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.18.82.63-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.51-1.04 2.18-.82 2.18-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0 0 16 8c0-4.42-3.58-8-8-8Z"
      />
    </svg>
    <span v-if="!iconOnly" class="github-project-link__text">
      <span v-if="variant === 'sidebar'" class="github-project-link__hint">{{ t('common.githubOn') }}</span>
      {{ label }}
    </span>
  </a>
</template>

<style scoped>
.github-project-link {
  @apply inline-flex items-center gap-2 transition-colors no-underline;
}

.github-project-link__icon {
  @apply w-4 h-4 shrink-0;
}

.github-project-link--inline {
  @apply text-blue-100 hover:text-white text-xs;
}

.github-project-link--inline .github-project-link__icon {
  @apply w-3.5 h-3.5 opacity-90;
}

.github-project-link--sidebar {
  @apply w-full px-3 py-2.5 rounded-md text-sm text-slate-300 hover:bg-slate-700/80 hover:text-white border border-slate-700/80;
}

.github-project-link--sidebar .github-project-link__hint {
  @apply block text-[10px] uppercase tracking-wide text-slate-500;
}

.github-project-link--sidebar .github-project-link__text {
  @apply flex flex-col items-start gap-0.5 font-mono text-xs leading-tight;
}

.github-project-link--icon-only {
  @apply justify-center p-2.5 rounded-md text-slate-400 hover:bg-slate-700/80 hover:text-white border border-transparent;
}

.github-project-link--icon-only .github-project-link__icon {
  @apply w-5 h-5;
}

.github-project-link--footer {
  @apply text-slate-500 hover:text-pfsense-nav text-sm;
}
</style>
