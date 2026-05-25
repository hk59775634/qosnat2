import { createI18n } from 'vue-i18n'
import en from '@/locales/en'
import zh from '@/locales/zh'

const LOCALE_KEY = 'qosnat2.locale'

export function getSavedLocale() {
  try {
    const v = localStorage.getItem(LOCALE_KEY)
    if (v === 'zh' || v === 'en') return v
  } catch {
    /* ignore */
  }
  return 'en'
}

export function saveLocale(locale) {
  try {
    localStorage.setItem(LOCALE_KEY, locale)
  } catch {
    /* ignore */
  }
}

const i18n = createI18n({
  legacy: false,
  locale: getSavedLocale(),
  fallbackLocale: 'en',
  messages: { en, zh },
})

export default i18n
