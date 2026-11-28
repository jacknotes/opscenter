/// <reference types="vite/client" />

/** 构建期由 vite 插件注入的版本号虚拟模块（见 web/build/appVersion.js） */
declare module 'virtual:app-version' {
  export const APP_VERSION: string
}

declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<{}, {}, any>
  export default component
}
