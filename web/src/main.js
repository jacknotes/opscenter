import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { forceReflow } from './directives/forceReflow'

import App from './App.vue'
import router from './router'
import { useAppStore } from './stores/app'
import { validateSession } from './stores/user'

// Element Plus 程序化组件的基础样式（unplugin-vue-components 无法自动导入）
import 'element-plus/theme-chalk/el-message.css'
import 'element-plus/theme-chalk/el-notification.css'
import 'element-plus/theme-chalk/el-message-box.css'

// 全局样式
import './assets/global.css'
import './assets/theme-dark.css'
import './assets/theme-light.css'

const app = createApp(App)

app.use(createPinia())
app.use(router)

// Element Plus 通过 unplugin-vue-components 自动按需注册组件
// 中文语言包通过 App.vue 中的 <el-config-provider> 配置

app.directive('force-reflow', forceReflow)

// 校验浏览器会话：若浏览器曾被关闭（会话 cookie 丢失）但 localStorage 残留 token，
// 清除 token 强制重新登录。需在路由守卫与 store 初始化前执行。
validateSession()

// 从 localStorage 读取主题偏好并应用
const appStore = useAppStore()
appStore.applyTheme()

app.mount('#app')
