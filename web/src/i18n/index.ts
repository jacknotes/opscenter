import { createI18n } from 'vue-i18n'
import zhCN from '../locales/zh-CN.json'

// 仅中文：文案统一收敛到 locales/zh-CN.json，架构上保留多语言扩展能力
export const i18n = createI18n({
  legacy: false,
  locale: 'zh-CN',
  fallbackLocale: 'zh-CN',
  messages: {
    'zh-CN': zhCN,
  },
})
