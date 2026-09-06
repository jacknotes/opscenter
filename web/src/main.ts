import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import { i18n } from './i18n'
import { initSession } from './utils/session'
import { forceReflow } from './directives/forceReflow'

// Element Plus 全量样式：vite.config 中 resolver 已关闭按组件 style/css 导入，
// 统一在此引入，避免运行时按需发现新依赖触发重预构建（dev 模式点页面卡住的根因）
import 'element-plus/dist/index.css'

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
// el-table 水平滚动修复（对齐 v1 v-force-reflow）
app.directive('force-reflow', forceReflow)
app.mount('#app')
