import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import { i18n } from './i18n'
import { initSession } from './utils/session'

// Element Plus 程序化组件的基础样式（unplugin-vue-components 无法自动导入）
import 'element-plus/theme-chalk/el-message.css'
import 'element-plus/theme-chalk/el-notification.css'
import 'element-plus/theme-chalk/el-message-box.css'
import 'element-plus/theme-chalk/el-loading.css'

// 全局样式（令牌 → 全局氛围 + EP 重上色 → 亮色覆盖）
import './styles/tokens.css'
import './styles/index.css'
import './styles/light.css'

// 会话三场景判定（复制标签页保持 / 浏览器重开重新登录）
initSession()

const app = createApp(App)
app.use(createPinia())
app.use(router)
app.use(i18n)
app.mount('#app')
