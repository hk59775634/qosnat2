import { createApp } from 'vue'
import App from './App.vue'
import router from './router'
import i18n from './i18n'
import './styles/main.css'

document.documentElement.lang = i18n.global.locale.value === 'zh' ? 'zh-CN' : 'en'

createApp(App).use(i18n).use(router).mount('#app')
