import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
import './assets/global.css'
import './assets/theme-dark.css'
import './assets/theme-light.css'
import zhCn from 'element-plus/dist/locale/zh-cn.mjs'
import { forceReflow } from './directives/forceReflow'

import App from './App.vue'
import router from './router'
import { useAppStore } from './stores/app'

const app = createApp(App)

app.use(createPinia())
app.use(router)
app.use(ElementPlus, { locale: zhCn })
app.directive('force-reflow', forceReflow)

// 从 localStorage 读取主题偏好并应用
const appStore = useAppStore()
appStore.applyTheme()

app.mount('#app')
