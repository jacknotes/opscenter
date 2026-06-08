# OpsCenter 前端重构计划

> 版本：v1.0 | 日期：2026-06-08 | 目标：代码架构优化 + UI/UX 提升 + 性能达标

---

## 目录

- [一、现状分析](#一现状分析)
- [二、重构目标与量化指标](#二重构目标与量化指标)
- [三、阶段一：工程化基础设施（第 1-2 周）](#三阶段一工程化基础设施第-1-2-周)
- [四、阶段二：代码架构重构（第 3-5 周）](#四阶段二代码架构重构第-3-5-周)
- [五、阶段三：UI/UX 视觉升级（第 6-8 周）](#五阶段三uiux-视觉升级第-6-8-周)
- [六、阶段四：性能优化（第 9-10 周）](#六阶段四性能优化第-9-10-周)
- [七、阶段五：测试与回归验证（第 11-12 周）](#七阶段五测试与回归验证第-11-12-周)
- [八、风险评估与缓解措施](#八风险评估与缓解措施)
- [九、时间线总览](#九时间线总览)
- [十、附录：设计系统规范](#十附录设计系统规范)

---

## 一、现状分析

### 1.1 技术栈概况

| 维度 | 现状 |
|------|------|
| 框架 | Vue 3.3 + Composition API (`<script setup>`) |
| 构建 | Vite 5，单插件（`@vitejs/plugin-vue`） |
| UI 库 | Element Plus 2.4，全量引入 |
| 状态管理 | Pinia（3 stores）+ 4 composables |
| HTTP | Axios 1.6，按领域分 11 个 API 模块 |
| 图表 | ECharts 6.1 + vue-echarts 8.0 |
| 样式 | 纯 CSS，3 个全局文件（global + dark + light），60+ 自定义 token |
| 类型 | 100% JavaScript，无 TypeScript |
| 代码规范 | 无 ESLint、无 Prettier、无测试框架 |

### 1.2 构建产物分析（当前基线）

| 文件 | 体积 | Gzip |
|------|------|------|
| `index.js`（主包） | **1,047.76 KB** | 346.51 KB |
| `Dashboard.js`（含 ECharts） | **633.70 KB** | 213.78 KB |
| `index.css`（全局样式） | 385.76 KB | 52.59 KB |
| `NginxManage.js` | 27.92 KB | 9.23 KB |
| `LvsManage.js` | 26.01 KB | 8.35 KB |
| `PreprodScale.js` | 21.02 KB | 7.31 KB |
| **总计** | **~2,230 KB** | **~660 KB** |

> ⚠️ 主包超过 1MB，Dashboard 因 ECharts 全量引入达 633KB。两个 chunk 触发 Vite 的 500KB 告警。

### 1.3 核心问题清单

#### 代码架构问题

| # | 问题 | 影响范围 | 严重度 |
|---|------|---------|--------|
| A1 | 超大组件：NginxManage(1662L)、Dashboard(1258L)、LvsManage(1076L) | 可维护性 | 🔴 高 |
| A2 | 预览→执行流程在 LVS/Nginx 中手动实现，未复用 `usePreviewExecute` | 重复代码 | 🟡 中 |
| A3 | 分页逻辑在 5 个页面重复定义（currentPage/pageSize/filtered/paginated） | 重复代码 | 🟡 中 |
| A4 | `formatTime()` 在 OpLog 和 UserManage 中重复实现 | 重复代码 | 🟢 低 |
| A5 | 路由守卫直接读 localStorage，与 Pinia store 双数据源 | 一致性风险 | 🟡 中 |
| A6 | `PreviewPanel.vue` 的 `panelWidth` prop 非响应式 | 功能缺陷 | 🟢 低 |
| A7 | `StreamOutput.vue` 硬编码颜色值，未走主题 token | 主题一致性 | 🟡 中 |
| A8 | `BatchConfirmDialog` 样式在 PreprodScale/LvsManage 中各自实现 | 重复代码 | 🟢 低 |
| A9 | 无 TypeScript，无类型安全 | 可维护性 | 🟡 中 |
| A10 | 无 ESLint/Prettier，代码风格不一致 | 代码质量 | 🟡 中 |

#### UI/UX 问题

| # | 问题 | 严重度 |
|---|------|--------|
| U1 | 移动端仅 768px 单断点，缺少 375px/1024px 适配 | 🟡 中 |
| U2 | 页面切换动效仅有 opacity fade，缺乏层次感 | 🟢 低 |
| U3 | 加载状态统一使用 `v-loading`，无骨架屏 | 🟡 中 |
| U4 | 部分可交互元素缺少 `cursor-pointer` | 🟢 低 |
| U5 | `font-base: 13px` 偏小，移动端可读性不佳 | 🟡 中 |
| U6 | 暗色主题下部分文字对比度接近 WCAG AA 边界（`#94A3B8` on `#141722` ≈ 4.6:1） | 🟡 中 |

---

## 二、重构目标与量化指标

| 指标 | 当前值 | 目标值 | 度量方式 |
|------|--------|--------|---------|
| 主包体积（gzip） | 346 KB | **≤ 250 KB**（-28%） | `vite build` 输出 |
| Dashboard chunk（gzip） | 213 KB | **≤ 120 KB**（-44%） | `vite build` 输出 |
| 总 JS 体积（gzip） | ~660 KB | **≤ 530 KB**（-20%） | `vite build` 输出 |
| LCP | 未测量 | **< 2.5s**（4G 网络） | Lighthouse |
| 组件最大行数 | 1662 行 | **≤ 400 行** | 代码审查 |
| 重复工具函数 | 6 处 | **0 处** | 代码审查 |
| TypeScript 覆盖 | 0% | **核心模块 100%** | 文件计数 |
| 移动端断点 | 1 个 | **3 个**（375/768/1024） | CSS 媒体查询 |

---

## 三、阶段一：工程化基础设施（第 1-2 周）

> 目标：搭建开发规范基座，后续重构有据可依。

### 3.1 引入 ESLint + Prettier

**安装：**
```bash
npm install -D eslint @vue/eslint-config-prettier eslint-plugin-vue prettier
```

**`.eslintrc.cjs` 配置要点：**
- 继承 `plugin:vue/vue3-recommended` + `@vue/eslint-config-prettier`
- 规则：`vue/multi-word-component-names: off`（现有组件不符合）
- 关闭与 Prettier 冲突的格式化规则

**`.prettierrc` 配置：**
```json
{
  "semi": false,
  "singleQuote": true,
  "tabWidth": 2,
  "trailingComma": "es5",
  "printWidth": 120,
  "vueIndentScriptAndStyle": false
}
```

**`package.json` 新增脚本：**
```json
{
  "lint": "eslint --ext .js,.vue src/",
  "lint:fix": "eslint --ext .js,.vue src/ --fix",
  "format": "prettier --write \"src/**/*.{js,vue,css}\""
}
```

**风险：** 首次格式化会产生大量 diff，需在单独 commit 中完成。
**缓解：** 在阶段一初期一次性格式化，后续 commit 保持一致。

### 3.2 引入 TypeScript（渐进式）

**策略：** 不强制一次性迁移，新文件/重构文件使用 `.ts`，现有 `.js` 逐步迁移。

**安装：**
```bash
npm install -D typescript vue-tsc @types/node
```

**配置文件：**
- `tsconfig.json` — 启用 `strict: false`（渐进模式），`allowJs: true`
- `vite.config.js` — 无需修改（Vite 原生支持 TS）

**优先迁移清单：**

| 优先级 | 文件 | 原因 |
|--------|------|------|
| P0 | `api/client.js` → `api/client.ts` | 类型化 API 响应 |
| P0 | `stores/*.js` → `stores/*.ts` | 类型化 store state |
| P0 | `composables/*.js` → `composables/*.ts` | 类型化参数和返回值 |
| P1 | `utils/*.js` → `utils/*.ts` | 类型化工具函数 |
| P1 | `constants.js` → `constants.ts` | 类型化常量 |
| P2 | `views/*.vue` — 添加 `lang="ts"` | 最后迁移 |

**类型定义文件 `src/types/`：**
```
src/types/
  api.d.ts       — API 响应类型（Server, User, OperationLog, LvsVip...）
  store.d.ts     — Store state 类型
  model.d.ts     — 业务模型类型
```

### 3.3 Vite 配置增强

**更新 `vite.config.js`：**

```js
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src'),
    },
  },
  server: {
    host: '0.0.0.0',
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:18080',
        changeOrigin: true,
        ws: true,
      },
    },
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks: {
          'vendor-vue': ['vue', 'vue-router', 'pinia'],
          'vendor-ep': ['element-plus'],
          'vendor-echarts': ['echarts', 'vue-echarts'],
        },
      },
    },
    chunkSizeWarningLimit: 600,
  },
})
```

**效果预估：** 拆分后主包从 1047KB 降至 ~400KB（gzip ~140KB）。

---

## 四、阶段二：代码架构重构（第 3-5 周）

> 目标：消除重复、拆分大组件、统一模式。

### 4.1 目录结构重组

**目标结构：**
```
web/src/
  api/
    client.ts              — Axios 实例 + 拦截器
    auth.ts                — 认证 API
    dashboard.ts           — 仪表盘 API
    k8s.ts                 — K8S API
    lvs.ts                 — LVS API
    nginx.ts               — Nginx API
    preprod.ts             — 预生产 API
    server.ts              — 服务器 API
    user.ts                — 用户 API
    log.ts                 — 日志 API
    index.ts               — 统一导出
  assets/
    global.css
    theme-dark.css
    theme-light.css
  components/
    Layout.vue
    PreviewPanel.vue
    StreamOutput.vue
    common/                 — 🆕 通用业务组件
      BatchConfirmDialog.vue
      EmptyState.vue
      PageCard.vue
      StatusBadge.vue
      StatsChips.vue
    charts/                 — 🆕 图表组件
      PieChart.vue
      TrendChart.vue
      ActivityChart.vue
  composables/
    usePreviewExecute.ts
    useServerSelector.ts
    useSelection.ts
    useOutputCache.ts
    usePagination.ts        — 🆕 统一分页
    useBatchOperation.ts    — 🆕 统一批量操作
    useConfirmDialog.ts     — 🆕 统一确认对话框
  directives/
    forceReflow.js
  router/
    index.ts
  stores/
    app.ts
    user.ts
    websocket.ts
  types/                    — 🆕 类型定义
    api.d.ts
    model.d.ts
  utils/
    format.ts               — 🆕 格式化工具（formatTime 等）
    constants.ts
  views/
    Dashboard/
      index.vue             — 主页面（精简后）
      components/
        StatCards.vue        — 数字卡片
        ModulePies.vue       — 模块饼图
        ActivityChart.vue    — 活动图表
        ProjectStats.vue     — 项目统计
    NginxManage/
      index.vue             — 主页面
      components/
        UpstreamGroup.vue    — 单个 upstream 组
        BatchDialog.vue      — 批量操作对话框
        BackupDialog.vue     — 备份对话框
        ConfigViewer.vue     — 配置查看器
    LvsManage/
      index.vue             — 主页面
      components/
        VipGroup.vue         — VIP 分组
        RsTable.vue          — RealServer 子表格
        SwapDialog.vue       — 切换对话框
        TagDialog.vue        — 标签对话框
    K8sDeploy.vue
    PreprodScale.vue
    OpLog.vue
    ServerManage.vue
    UserManage.vue
    Login.vue
```

### 4.2 新增 Composables

#### 4.2.1 `usePagination` — 统一分页

```ts
// composables/usePagination.ts
import { ref, computed, watch } from 'vue'

interface UsePaginationOptions {
  pageSize?: number
  resetOn?: () => any[]  // 当此数据变化时重置页码
}

export function usePagination<T>(data: () => T[], opts: UsePaginationOptions = {}) {
  const currentPage = ref(1)
  const pageSize = ref(opts.pageSize ?? 20)

  const total = computed(() => data().length)
  const paginated = computed(() => {
    const start = (currentPage.value - 1) * pageSize.value
    return data().slice(start, start + pageSize.value)
  })

  // 数据源变化时重置页码
  if (opts.resetOn) {
    watch(opts.resetOn, () => { currentPage.value = 1 })
  }

  function handlePageChange(page: number) { currentPage.value = page }
  function handleSizeChange(size: number) { pageSize.value = size; currentPage.value = 1 }

  return { currentPage, pageSize, total, paginated, handlePageChange, handleSizeChange }
}
```

**使用示例（ServerManage）：**
```vue
<script setup>
const filteredServers = computed(() => servers.value.filter(/*...*/))
const { currentPage, pageSize, total, paginated, handlePageChange, handleSizeChange } =
  usePagination(filteredServers, { resetOn: () => [searchKeyword.value, statusFilter.value] })
</script>
```

#### 4.2.2 `useBatchOperation` — 统一批量操作

```ts
// composables/useBatchOperation.ts
import { ref, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'

interface UseBatchOptions {
  threshold?: number           // 超过此数量需输入确认文字
  confirmText?: string         // 确认文字（默认 "确认执行"）
}

export function useBatchOperation(opts: UseBatchOptions = {}) {
  const selectedRows = ref<Set<any>>(new Set())
  const selectedCount = computed(() => selectedRows.value.size)

  function toggle(row: any) {
    selectedRows.value.has(row)
      ? selectedRows.value.delete(row)
      : selectedRows.value.add(row)
  }

  function clearAll() { selectedRows.value.clear() }

  async function confirmBatch(actionName: string, onConfirm: () => Promise<void>) {
    if (selectedRows.value.size === 0) {
      ElMessage.warning('请先选择操作项')
      return
    }

    if (opts.threshold && selectedRows.value.size >= opts.threshold) {
      try {
        await ElMessageBox.prompt(
          `当前选择了 ${selectedRows.value.size} 项，输入"${opts.confirmText || '确认执行'}"以继续`,
          '批量操作确认',
          { inputPattern: new RegExp(opts.confirmText || '确认执行'), inputErrorMessage: '输入不正确' }
        )
      } catch { return }
    }

    await onConfirm()
  }

  return { selectedRows, selectedCount, toggle, clearAll, confirmBatch }
}
```

#### 4.2.3 `useConfirmDialog` — 危险操作确认

```ts
// composables/useConfirmDialog.ts
import { ref } from 'vue'
import { ElMessageBox } from 'element-plus'

export function useConfirmDialog() {
  const loading = ref(false)

  async function confirm(options: {
    title: string
    message: string
    type?: 'warning' | 'error' | 'info'
    confirmText?: string
    onConfirm: () => Promise<void>
  }) {
    try {
      await ElMessageBox.confirm(options.message, options.title, {
        type: options.type || 'warning',
        confirmButtonText: options.confirmText || '确定',
        cancelButtonText: '取消',
      })
      loading.value = true
      await options.onConfirm()
    } catch {
      // 用户取消
    } finally {
      loading.value = false
    }
  }

  return { loading, confirm }
}
```

### 4.3 通用业务组件抽取

#### 4.3.1 `EmptyState.vue`

```vue
<!-- components/common/EmptyState.vue -->
<template>
  <div class="empty-state">
    <el-icon class="empty-state-icon" :size="48">
      <component :is="icon" />
    </el-icon>
    <p class="empty-state-text">{{ text }}</p>
    <slot />
  </div>
</template>

<script setup>
defineProps({
  icon: { type: [Object, String], default: 'Document' },
  text: { type: String, default: '暂无数据' },
})
</script>
```

#### 4.3.2 `StatusBadge.vue`

```vue
<!-- components/common/StatusBadge.vue -->
<template>
  <el-tag :type="typeMap[status] || 'info'" size="small" effect="light">
    <span :class="['status-dot', `status-${status}`]" />
    {{ label || status }}
  </el-tag>
</template>

<script setup>
defineProps({
  status: { type: String, required: true },
  label: { type: String, default: '' },
  typeMap: {
    type: Object,
    default: () => ({ up: 'success', down: 'danger', online: 'success', offline: 'danger' }),
  },
})
</script>
```

#### 4.3.3 `PageCard.vue` — 统一页面卡片壳

```vue
<!-- components/common/PageCard.vue -->
<template>
  <el-card class="main-card">
    <template #header>
      <slot name="header" />
    </template>
    <slot />
  </el-card>
</template>
```

### 4.4 超大组件拆分方案

#### 4.4.1 NginxManage.vue（1662 行 → ~300 行）

**拆分方案：**

| 子组件 | 行数估计 | 职责 |
|--------|---------|------|
| `index.vue` | ~300L | 页面容器、数据加载、状态管理 |
| `UpstreamGroup.vue` | ~200L | 单个 upstream 折叠面板 |
| `BatchDialog.vue` | ~150L | 批量上线/下线对话框 |
| `BackupDialog.vue` | ~100L | 备份列表对话框 |
| `ConfigViewer.vue` | ~80L | 配置文件查看器 |

**提取的 composable：**
- `useNginxData.ts` — 服务器/配置/upstream 数据加载和管理
- 复用 `usePreviewExecute` 替代手动预览逻辑
- 复用 `usePagination`（如果适用）

#### 4.4.2 Dashboard.vue（1258 行 → ~200 行）

**拆分方案：**

| 子组件 | 行数估计 | 职责 |
|--------|---------|------|
| `index.vue` | ~200L | 布局容器、数据加载调度 |
| `StatCards.vue` | ~120L | 数字卡片（在线用户、登录成功/失败） |
| `ModulePies.vue` | ~200L | 4 个模块饼图 |
| `ActivityChart.vue` | ~150L | 活动趋势折线图 |
| `ProjectStats.vue` | ~180L | K8S/预生产项目统计表格 |

**提取的 composable：**
- `useDashboardStats.ts` — 统计数据加载与定时刷新
- `useRemoteStats.ts` — 远程状态（LVS/Nginx/K8S/Preprod）

#### 4.4.3 LvsManage.vue（1076 行 → ~350 行）

**拆分方案：**

| 子组件 | 行数估计 | 职责 |
|--------|---------|------|
| `index.vue` | ~350L | 页面容器、主表格 |
| `VipGroup.vue` | ~200L | VIP 分组行 + RS 子表格 |
| `RsTable.vue` | ~150L | RealServer 表格 |
| `SwapDialog.vue` | ~80L | 切换确认对话框 |
| `TagDialog.vue` | ~100L | 标签管理对话框 |

### 4.5 统一工具函数

**新建 `utils/format.ts`：**

```ts
export function formatTime(t: string | Date | number): string {
  return new Date(t).toLocaleString('zh-CN', { timeZone: 'Asia/Shanghai' })
}

export function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

export function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
  return `${Math.floor(ms / 60000)}m ${Math.round((ms % 60000) / 1000)}s`
}
```

**新建 `utils/constants.ts`（从 `constants.js` 迁移 + 扩展）：**

```ts
export const STORAGE_KEYS = {
  TOKEN: 'token',
  ROLE: 'role',
  THEME: 'theme',
  SIDEBAR_COLLAPSED: 'sidebarCollapsed',
} as const

export const MODULE_LABELS: Record<string, string> = {
  lvs: 'LVS',
  nginx: 'Nginx',
  k8s: 'K8S',
  preprod: '预生产',
  server: '服务器',
  user: '用户',
  auth: '认证',
}

export const MODULE_TAG_TYPES: Record<string, string> = {
  lvs: '',
  nginx: 'success',
  k8s: 'warning',
  preprod: 'info',
  server: 'danger',
  user: '',
  auth: 'info',
}

export const PAGE_SIZES = [10, 20, 50, 100] as const
export const DEFAULT_PAGE_SIZE = 20
```

### 4.6 路由守卫与 Store 统一

**问题：** 路由守卫直接读 `localStorage.getItem('token')` 和 `localStorage.getItem('role')`，与 `useUserStore` 双数据源。

**修复方案：**

```ts
// router/index.ts
import { useUserStore } from '@/stores/user'

router.beforeEach((to, from, next) => {
  const userStore = useUserStore()

  if (to.meta.requiresAuth !== false && !userStore.isLoggedIn) {
    return next('/login')
  }

  if (to.meta.admin && !userStore.isAdmin) {
    return next('/dashboard')
  }

  next()
})
```

> **注意：** `useUserStore()` 在 router guard 中调用时，Pinia 必须已 install。需确保 `router.afterEach` 或在 `main.js` 中先 `createPinia()` 再 `use(router)` — 当前代码已是正确顺序。

---

## 五、阶段三：UI/UX 视觉升级（第 6-8 周）

> 目标：基于 Element Plus 设计 Token 标准化，提升视觉层次和交互体验。

### 5.1 设计系统增强

#### 5.1.1 色彩对比度优化

**当前问题：** 暗色主题下 `--text-regular: #94A3B8` on `--bg-card: #141722` 对比度 ≈ 4.6:1，刚好达到 WCAG AA 标准，但在小字号下可读性不足。

**优化方案：**

```css
/* theme-dark.css 调整 */
html.dark {
  --text-primary: #F1F5F9;    /* #E2E8F0 → #F1F5F9，对比度从 11.2:1 → 14.1:1 */
  --text-regular: #A8B8C8;    /* #94A3B8 → #A8B8C8，对比度从 4.6:1 → 5.8:1 */
  --text-secondary: #7A8A9C;  /* #64748B → #7A8A9C，对比度从 3.1:1 → 4.0:1 */
}
```

#### 5.1.2 字号阶梯标准化

**当前：** `--font-base: 13px`，移动端偏小。

**调整：**

```css
:root {
  --font-xs: 11px;     /* 辅助说明、徽章 */
  --font-sm: 12px;     /* 表格次要信息 */
  --font-base: 14px;   /* 正文（原 13px → 14px） */
  --font-md: 15px;     /* 强调正文 */
  --font-lg: 16px;     /* 小标题 */
  --font-xl: 20px;     /* 页面标题 */
  --font-2xl: 24px;    /* 卡片标题 */
  --font-3xl: 32px;    /* 数字展示 */
}
```

#### 5.1.3 间距系统扩展

```css
:root {
  --spacing-3xs: 2px;
  --spacing-2xs: 4px;    /* = xs */
  --spacing-xs: 8px;     /* 原 4px → 8px */
  --spacing-sm: 12px;    /* 原 8px → 12px */
  --spacing-md: 16px;    /* 原 12px → 16px */
  --spacing-lg: 24px;    /* 原 16px → 24px */
  --spacing-xl: 32px;    /* 原 20px → 32px */
  --spacing-2xl: 48px;
}
```

### 5.2 响应式增强

**当前：** 仅 `max-width: 768px` 一个断点。

**新增断点体系：**

```css
/* 手机竖屏 */
@media (max-width: 374px) {
  :root { --font-base: 14px; }
  .el-card__header > div { flex-direction: column; }
  .toolbar { flex-direction: column; }
  .toolbar .el-button { width: 100%; }
  .stat-chip { font-size: var(--font-xs); }
}

/* 手机横屏 / 小平板 */
@media (max-width: 767px) {
  /* 现有的 768px 规则移至此处 */
}

/* 平板 */
@media (min-width: 768px) and (max-width: 1023px) {
  .main-card { max-height: calc(100vh - 120px); }
  .el-table { font-size: var(--font-sm); }
}

/* 小桌面 */
@media (min-width: 1024px) and (max-width: 1439px) {
  /* 默认布局，微调间距 */
}

/* 大桌面 */
@media (min-width: 1440px) {
  .el-card__header > div { gap: 16px; }
  .toolbar { padding: 12px 16px; }
}
```

### 5.3 交互动效

#### 5.3.1 页面切换动效增强

```vue
<!-- App.vue -->
<template>
  <router-view v-slot="{ Component, route }">
    <transition name="page-slide" mode="out-in">
      <component :is="Component" :key="route.path" />
    </transition>
  </router-view>
</template>

<style>
.page-slide-enter-active,
.page-slide-leave-active {
  transition: all 0.2s ease;
}
.page-slide-enter-from {
  opacity: 0;
  transform: translateY(8px);
}
.page-slide-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}
</style>
```

#### 5.3.2 骨架屏加载

**替换 `v-loading` 为骨架屏：**

```vue
<!-- 示例：Dashboard StatCards -->
<template>
  <div class="stat-row">
    <template v-if="loading">
      <div v-for="i in 3" :key="i" class="number-card skeleton-card">
        <el-skeleton :rows="2" animated />
      </div>
    </template>
    <template v-else>
      <div class="number-card number-card--cyan">...</div>
      <!-- ... -->
    </template>
  </div>
</template>
```

#### 5.3.3 列表进入动效

```css
/* 表格行淡入 */
@keyframes rowFadeIn {
  from { opacity: 0; transform: translateX(-8px); }
  to { opacity: 1; transform: translateX(0); }
}

:deep(.el-table__row) {
  animation: rowFadeIn 0.2s ease forwards;
}

/* 交错延迟 */
:deep(.el-table__row:nth-child(1)) { animation-delay: 0ms; }
:deep(.el-table__row:nth-child(2)) { animation-delay: 20ms; }
:deep(.el-table__row:nth-child(3)) { animation-delay: 40ms; }
/* ... 最多到第 10 行 */
```

> **无障碍：** 媒体查询 `@media (prefers-reduced-motion: reduce)` 下禁用所有动效。

### 5.4 StreamOutput 主题化

**当前问题：** `StreamOutput.vue` 硬编码 `#0B0D13`、`#22D3EE`、`#FB7185`。

**修复：** 使用已有的 CSS token：

```css
.terminal-pre {
  background: var(--terminal-bg);
  color: var(--terminal-text);
  /* 移除硬编码颜色 */
}
.terminal-error {
  color: var(--color-danger);
}
.terminal-success {
  color: var(--color-success);
}
```

### 5.5 图标统一

**当前问题：** 部分页面使用内联 SVG path，部分使用 Element Plus Icons。

**方案：** 统一使用 `@element-plus/icons-vue`，移除所有内联 SVG：

```vue
<!-- 替换前 -->
<el-icon><svg viewBox="0 0 1024 1024"><path d="M831.872 340.864..."/></svg></el-icon>

<!-- 替换后 -->
<el-icon><ArrowDown /></el-icon>
```

> Element Plus Icons 是按需引入的（tree-shakable），不影响包体积。

### 5.6 Login 页面优化

**增强视觉效果：**
- 添加品牌 logo 区域
- 输入框添加图标前缀（`User`、`Lock`）
- 登录按钮添加加载动画
- 添加 `prefers-color-scheme` 跟随系统主题

---

## 六、阶段四：性能优化（第 9-10 周）

> 目标：包体积减少 ≥20%，LCP < 2.5s。

### 6.1 ECharts 按需引入

**当前问题：** Dashboard 全量引入 ECharts（633KB chunk）。

**优化方案：**

```ts
// composables/useECharts.ts
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { PieChart, LineChart, BarChart } from 'echarts/charts'
import {
  TitleComponent,
  TooltipComponent,
  LegendComponent,
  GridComponent,
} from 'echarts/components'

use([
  CanvasRenderer,
  PieChart,
  LineChart,
  BarChart,
  TitleComponent,
  TooltipComponent,
  LegendComponent,
  GridComponent,
])
```

**预估效果：** ECharts chunk 从 633KB → ~250KB（gzip 213KB → ~85KB）。

### 6.2 Element Plus 按需引入

**方案 A（推荐）：使用 `unplugin-vue-components` + `unplugin-auto-import`**

```bash
npm install -D unplugin-vue-components unplugin-auto-import
```

```js
// vite.config.js
import AutoImport from 'unplugin-auto-import/vite'
import Components from 'unplugin-vue-components/vite'
import { ElementPlusResolver } from 'unplugin-vue-components/resolvers'

export default defineConfig({
  plugins: [
    vue(),
    AutoImport({
      resolvers: [ElementPlusResolver()],
      imports: ['vue', 'vue-router', 'pinia'],
    }),
    Components({
      resolvers: [ElementPlusResolver()],
    }),
  ],
})
```

**效果：** 移除 `import ElementPlus from 'element-plus'` 全量引入，主包从 1047KB → ~500KB。

> **注意：** Element Plus 按需引入可能影响当前的 CSS 变量覆盖（`theme-dark.css`/`theme-light.css`）。需测试所有组件样式是否正常。

### 6.3 路由懒加载优化

**当前已使用懒加载，但可进一步优化：**

```ts
// router/index.ts — 添加预加载
const Dashboard = () => import(/* webpackPrefetch: true */ '@/views/Dashboard/index.vue')
const NginxManage = () => import(/* webpackPrefetch: true */ '@/views/NginxManage/index.vue')
// 其他路由按需加载，不预取
```

### 6.4 图片和字体优化

**字体：**
```html
<!-- index.html — 预连接 -->
<link rel="preconnect" href="https://fonts.googleapis.com" />
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin />
<link rel="preload" as="style" href="https://fonts.googleapis.com/css2?family=DM+Sans:wght@400;500;600;700&display=swap" />
```

**字体加载策略：**
```css
html {
  font-family: 'DM Sans', 'Noto Sans SC', system-ui, -apple-system, sans-serif;
  font-display: swap; /* 确保 */
}
```

### 6.5 构建产物预估

| 文件 | 当前（gzip） | 优化后（gzip） | 减少 |
|------|-------------|---------------|------|
| 主包 | 346 KB | ~170 KB | -51% |
| Dashboard | 213 KB | ~85 KB | -60% |
| CSS | 52 KB | ~50 KB | -4% |
| 其他 chunks | ~49 KB | ~45 KB | -8% |
| **总计** | **~660 KB** | **~350 KB** | **-47%** |

---

## 七、阶段五：测试与回归验证（第 11-12 周）

> 目标：确保零功能回归，性能达标。

### 7.1 功能回归测试清单

| 模块 | 测试项 |
|------|--------|
| **登录** | 正常登录、错误密码、token 过期自动跳转 |
| **Dashboard** | 统计数据加载、远程状态刷新、图表渲染、自动定时刷新 |
| **LVS** | 服务器切换、VIP 列表加载、上线/下线/切换、标签管理、批量操作、预览→执行 |
| **Nginx** | 服务器/配置切换、upstream 列表、上线/下线/切换、批量、备份、配置查看 |
| **K8S** | Rollout 列表、单项目/全量上线/同步/回滚、预览→执行、WebSocket 流式输出 |
| **预生产** | 状态加载、缩容/扩容、LVS 检查、批量操作、WebSocket 流式输出 |
| **服务器管理** | CRUD、测试连接、启停、批量操作、LDAP 导入 |
| **用户管理** | CRUD、重置密码、启停、LDAP 导入 |
| **日志审计** | 列表加载、筛选、分页 |
| **通用** | 主题切换（dark/light）、侧边栏折叠、移动端适配、WebSocket 状态栏 |

### 7.2 性能验证

```bash
# 构建体积
npm run build  # 检查所有 chunk < 500KB

# Lighthouse 审计（需先部署到可访问的地址）
npx lighthouse http://localhost:3000 --output html --view
# 目标：LCP < 2.5s, FCP < 1.8s, CLS < 0.1

# 包分析
npx vite-bundle-visualizer
```

### 7.3 兼容性测试

| 浏览器 | 版本 |
|--------|------|
| Chrome | 最新 2 个大版本 |
| Firefox | 最新 2 个大版本 |
| Safari | 最新 2 个大版本 |
| Edge | 最新 2 个大版本 |
| 移动端 Chrome | Android 12+ |
| 移动端 Safari | iOS 16+ |

---

## 八、风险评估与缓解措施

| 风险 | 概率 | 影响 | 缓解措施 |
|------|------|------|---------|
| Element Plus 按需引入导致样式丢失 | 中 | 高 | 阶段团单独分支，逐组件验证；保留全量引入作为回退方案 |
| TypeScript 迁移引入编译错误 | 中 | 中 | 使用 `strict: false` + `allowJs: true` 渐进迁移，不阻塞现有功能 |
| 组件拆分后 props/emit 接口变化 | 低 | 高 | 先写接口定义（TypeScript interface），再实现；每个拆分单独 PR 审查 |
| ECharts 按需引入遗漏图表类型 | 低 | 中 | 先统计所有使用的图表类型，再配置 imports |
| 移动端响应式改动影响桌面端 | 低 | 中 | 使用 `min-width` 断点（mobile-first），桌面端默认样式不变 |
| 首次 ESLint/Prettier 格式化产生大量 diff | 高 | 低 | 单独 commit，不混入功能变更 |
| 国际化（zh-CN）资源加载时机 | 低 | 低 | Element Plus locale 保持当前 `app.use(ElementPlus, { locale: zhCn })` 方式 |

---

## 九、时间线总览

```
第 1-2 周   ████████  阶段一：工程化基础设施
                      ├── ESLint + Prettier 配置
                      ├── TypeScript 环境搭建
                      ├── Vite 配置优化
                      └── 首次格式化 commit

第 3-5 周   ████████████  阶段二：代码架构重构
                          ├── 目录结构重组
                          ├── 新增 composables (usePagination/useBatchOperation)
                          ├── 通用组件抽取 (EmptyState/StatusBadge/PageCard)
                          ├── 超大组件拆分 (Nginx/Dashboard/LVS)
                          ├── 工具函数统一 (formatTime 等)
                          └── 路由守卫统一

第 6-8 周   ████████████  阶段三：UI/UX 视觉升级
                          ├── 色彩对比度优化
                          ├── 字号/间距标准化
                          ├── 响应式断点增强
                          ├── 交互动效 (页面切换/骨架屏/列表进入)
                          ├── StreamOutput 主题化
                          └── 图标统一

第 9-10 周  ████████  阶段四：性能优化
                      ├── ECharts 按需引入
                      ├── Element Plus 按需引入
                      ├── 路由预加载
                      └── 字体加载优化

第 11-12 周 ████████  阶段五：测试与验证
                      ├── 功能回归测试
                      ├── 性能验证 (Lighthouse)
                      ├── 兼容性测试
                      └── 文档更新
```

**总计：12 周（约 3 个月）**

**里程碑：**

| 时间点 | 里程碑 | 验收标准 |
|--------|--------|---------|
| 第 2 周末 | 工程化就绪 | ESLint/TS 可用，所有文件格式化完成 |
| 第 5 周末 | 架构重构完成 | 所有组件 < 400 行，重复代码清零 |
| 第 8 周末 | UI 升级完成 | 3 个断点适配，动效上线 |
| 第 10 周末 | 性能优化完成 | 主包 < 250KB(gzip)，Dashboard < 120KB(gzip) |
| 第 12 周末 | 全量回归通过 | 功能零回归，LCP < 2.5s |

---

## 十、附录：设计系统规范

### 10.1 当前 Token 体系（保持不变）

OpsCenter 已有完善的 CSS Custom Properties 体系，覆盖 60+ token。以下为梳理后的完整 token map：

```
背景层级:  --bg-base → --bg-card → --bg-elevated
文字层级:  --text-primary → --text-regular → --text-secondary → --text-placeholder
边框层级:  --border-default → --border-strong
强调色:    --color-primary → --color-primary-hover → --color-primary-active
状态色:    --color-success / --color-warning / --color-danger / --color-info
模块色:    --module-lvs / --module-nginx / --module-k8s / --module-preprod / --module-log
侧边栏:    --sidebar-bg / --sidebar-text / --sidebar-active-*
终端:      --terminal-bg / --terminal-text / --terminal-radius
间距:      --spacing-xs → --spacing-xl
字号:      --font-xs → --font-3xl
字重:      --weight-normal / --weight-medium / --weight-semibold / --weight-bold
对话框:    --dialog-sm → --dialog-xl
```

### 10.2 新增 Token 建议

```css
:root {
  /* 动效时长 */
  --duration-fast: 150ms;
  --duration-normal: 200ms;
  --duration-slow: 300ms;

  /* 动效缓动 */
  --ease-default: cubic-bezier(0.4, 0, 0.2, 1);
  --ease-in: cubic-bezier(0.4, 0, 1, 1);
  --ease-out: cubic-bezier(0, 0, 0.2, 1);

  /* 圆角 */
  --radius-sm: 4px;
  --radius-md: 8px;
  --radius-lg: 12px;
  --radius-xl: 16px;
  --radius-full: 9999px;

  /* 阴影 */
  --shadow-sm: 0 1px 2px rgba(0, 0, 0, 0.05);
  --shadow-md: 0 4px 6px rgba(0, 0, 0, 0.07);
  --shadow-lg: 0 10px 15px rgba(0, 0, 0, 0.1);
  --shadow-xl: 0 20px 25px rgba(0, 0, 0, 0.15);

  /* Z-Index 层级 */
  --z-dropdown: 1000;
  --z-sticky: 1020;
  --z-fixed: 1030;
  --z-modal-backdrop: 1040;
  --z-modal: 1050;
  --z-popover: 1060;
  --z-tooltip: 1070;
}
```

### 10.3 组件使用规范

| 组件 | 使用场景 | 注意事项 |
|------|---------|---------|
| `el-card.main-card` | 每个页面的主容器 | 唯一，包含 header 和 body |
| `.toolbar` | header 内的操作按钮栏 | 使用 flex 布局，`margin-left: auto` 分隔左右 |
| `.stat-chip` | header 内的统计徽章 | 使用 `stat-chip-success/danger/primary/warning` 变体 |
| `.pagination-wrapper` | 表格下方分页 | 左侧放统计信息，右侧放 `el-pagination` |
| `.terminal-pre` | 终端输出展示 | 使用 CSS token，不硬编码颜色 |
| `.preview-pre` | 预览对话框内代码 | 同上 |

---

> **文档维护：** 本计划为活文档，随重构推进持续更新。每个阶段完成后更新实际耗时和遇到的问题。
