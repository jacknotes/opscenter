<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import {
  Odometer,
  Connection,
  Guide,
  Box,
  ScaleToOriginal,
  Document,
  Cpu,
  User,
  UserFilled,
  Fold,
  Expand,
  Sunny,
  Moon,
  Key,
  SwitchButton,
} from '@element-plus/icons-vue'
import { useAuthStore } from '@/stores/auth'
import { useTheme } from '@/composables/useTheme'
import { STORAGE_KEYS } from '@/utils/constants'
import { i18n } from '@/i18n'
import ChangePasswordDialog from './ChangePasswordDialog.vue'

const t = i18n.global.t

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const { theme, toggle: toggleTheme } = useTheme()

// 侧边栏折叠状态持久化（刷新后恢复）
const collapsed = ref(localStorage.getItem(STORAGE_KEYS.SIDEBAR_COLLAPSED) === '1')
watch(collapsed, (v) => {
  if (v) localStorage.setItem(STORAGE_KEYS.SIDEBAR_COLLAPSED, '1')
  else localStorage.removeItem(STORAGE_KEYS.SIDEBAR_COLLAPSED)
})

interface NavItem {
  path: string
  titleKey: string
  icon: typeof Odometer
  adminOnly?: boolean
}

const navItems: NavItem[] = [
  { path: '/dashboard', titleKey: 'nav.dashboard', icon: Odometer },
  { path: '/lvs', titleKey: 'nav.lvs', icon: Connection },
  { path: '/nginx', titleKey: 'nav.nginx', icon: Guide },
  { path: '/k8s', titleKey: 'nav.k8s', icon: Box },
  { path: '/preprod', titleKey: 'nav.preprod', icon: ScaleToOriginal },
  { path: '/logs', titleKey: 'nav.logs', icon: Document },
  { path: '/servers', titleKey: 'nav.servers', icon: Cpu, adminOnly: true },
  { path: '/users', titleKey: 'nav.users', icon: UserFilled, adminOnly: true },
]

const visibleNav = computed(() => navItems.filter((item) => !item.adminOnly || auth.isAdmin))

const activeMenu = computed(() => route.path)
const pageTitle = computed(() => {
  const key = (route.meta.titleKey as string) || 'app.name'
  return t(key)
})

const avatarChar = computed(() => (auth.displayName || '?').charAt(0).toUpperCase())

// 个人信息弹窗
const profileVisible = ref(false)

async function handleLogout(): Promise<void> {
  await auth.logout()
  ElMessage.success(t('auth.logoutSuccess'))
  await router.replace('/login')
}

function handleChangePassword(): void {
  // 打开修改密码弹窗（全局事件，由页面级对话框组件监听）
  window.dispatchEvent(new CustomEvent('opscenter:change-password'))
}

function openSwagger(): void {
  window.open('/swagger/index.html', '_blank', 'noopener')
}

// 快捷键 r：刷新当前页面数据（输入框内不触发），页面监听 page-refresh 事件
function handleKeydown(e: KeyboardEvent): void {
  const target = e.target as HTMLElement | null
  const tag = target?.tagName
  if (tag === 'INPUT' || tag === 'TEXTAREA' || target?.isContentEditable) return
  if (e.key === 'r' && !e.metaKey && !e.ctrlKey && !e.altKey) {
    e.preventDefault()
    window.dispatchEvent(new CustomEvent('page-refresh'))
  }
}

onMounted(() => {
  document.addEventListener('keydown', handleKeydown)
  // 拉取最新用户信息（角色/启用状态可能被管理员变更），失败静默（保留登录时缓存）
  auth.refreshUser().catch(() => {})
})
onUnmounted(() => document.removeEventListener('keydown', handleKeydown))
</script>

<template>
  <div class="app-layout" :class="{ collapsed }">
    <!-- 侧边栏（桌面） -->
    <aside class="sidebar">
      <div class="brand">
        <span class="brand-mark">✦</span>
        <span v-show="!collapsed" class="grad-text brand-name">OpsCenter</span>
      </div>

      <div v-show="!collapsed" class="status-pill mono">
        <span class="dot-live" />
        <span>SIGNAL ONLINE</span>
      </div>

      <el-menu :default-active="activeMenu" :collapse="collapsed" :collapse-transition="false" router>
        <el-menu-item v-for="item in visibleNav" :key="item.path" :index="item.path">
          <el-icon><component :is="item.icon" /></el-icon>
          <template #title>{{ t(item.titleKey) }}</template>
        </el-menu-item>
      </el-menu>

      <div v-show="!collapsed" class="sidebar-version mono">{{ t('app.version') }}</div>
    </aside>

    <!-- 主区域 -->
    <div class="main-area">
      <!-- 顶栏 -->
      <header class="topbar">
        <div class="topbar-left">
          <el-button text circle @click="collapsed = !collapsed" :title="t('nav.collapse')" class="collapse-btn">
            <el-icon v-if="collapsed"><Expand /></el-icon>
            <el-icon v-else><Fold /></el-icon>
          </el-button>
          <span class="page-title">{{ pageTitle }}</span>
          <span class="mono route-path">{{ route.path }}</span>
        </div>

        <div class="topbar-right">
          <el-button text circle :title="t('common.theme')" @click="toggleTheme">
            <el-icon v-if="theme === 'dark'"><Sunny /></el-icon>
            <el-icon v-else><Moon /></el-icon>
          </el-button>

          <el-dropdown trigger="click">
            <div class="user-chip">
              <span class="avatar">{{ avatarChar }}</span>
              <span class="user-name">{{ auth.displayName }}</span>
              <span class="role-badge mono">{{ auth.user?.role }}</span>
            </div>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item @click="profileVisible = true">
                  <el-icon><User /></el-icon>{{ t('common.profile') }}
                </el-dropdown-item>
                <el-dropdown-item @click="handleChangePassword">
                  <el-icon><Key /></el-icon>{{ t('common.changePassword') }}
                </el-dropdown-item>
                <el-dropdown-item v-if="auth.isAdmin" divided @click="openSwagger">
                  <el-icon><Document /></el-icon>API 文档
                </el-dropdown-item>
                <el-dropdown-item divided @click="handleLogout">
                  <el-icon><SwitchButton /></el-icon>{{ t('common.logout') }}
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </header>

      <!-- 内容区 -->
      <main class="content">
        <router-view v-slot="{ Component }">
          <transition name="page" mode="out-in">
            <component :is="Component" />
          </transition>
        </router-view>
      </main>
    </div>

    <!-- 移动端底部导航（全部条目，横向可滚动） -->
    <nav class="mobile-nav">
      <router-link
        v-for="item in visibleNav"
        :key="item.path"
        :to="item.path"
        class="mobile-nav-item"
        :class="{ active: route.path === item.path }"
      >
        <el-icon><component :is="item.icon" /></el-icon>
        <span>{{ t(item.titleKey) }}</span>
      </router-link>
    </nav>
    <!-- 个人信息 -->
    <el-dialog v-model="profileVisible" :title="t('common.profile')" width="min(440px, 92vw)" append-to-body>
      <el-descriptions :column="1" border>
        <el-descriptions-item label="用户名">
          <span class="mono">{{ auth.user?.username }}</span>
        </el-descriptions-item>
        <el-descriptions-item label="姓名">{{ auth.user?.name || '-' }}</el-descriptions-item>
        <el-descriptions-item label="邮箱">{{ auth.user?.email || '-' }}</el-descriptions-item>
        <el-descriptions-item label="角色">
          <el-tag size="small" :type="auth.isAdmin ? 'danger' : 'info'" effect="plain">{{ auth.user?.role }}</el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="认证来源">{{ auth.user?.auth_source || 'local' }}</el-descriptions-item>
      </el-descriptions>
    </el-dialog>
    <!-- 修改密码（顶栏用户菜单触发） -->
    <ChangePasswordDialog />
  </div>
</template>

<style scoped>
.app-layout {
  min-height: 100vh;
  display: flex;
}

/* ---- 侧边栏 ---- */
.sidebar {
  width: var(--sidebar-w);
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  padding: var(--space-5) 0 var(--space-4);
  background: var(--bg-card);
  backdrop-filter: blur(14px) saturate(1.2);
  -webkit-backdrop-filter: blur(14px) saturate(1.2);
  border-right: 1px solid var(--border);
  transition: width var(--dur-base) var(--ease-out);
  position: sticky;
  top: 0;
  height: 100vh;
}

.app-layout.collapsed .sidebar {
  width: var(--sidebar-w-collapsed);
}

.brand {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: 0 var(--space-5);
  margin-bottom: var(--space-5);
  white-space: nowrap;
  overflow: hidden;
}

.brand-mark {
  font-size: var(--text-xl);
  color: var(--indigo-400);
  text-shadow: 0 0 18px rgba(99, 102, 241, 0.6);
}

.brand-name {
  font-family: var(--font-display);
  font-size: var(--text-lg);
  font-weight: 700;
  letter-spacing: 0.03em;
}

.status-pill {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  margin: 0 var(--space-4) var(--space-4);
  padding: var(--space-2) var(--space-3);
  border: 1px solid var(--border);
  border-radius: var(--radius-pill);
  font-size: var(--text-xs);
  color: var(--text-secondary);
  letter-spacing: 0.12em;
  background: var(--bg-input);
  white-space: nowrap;
}

.sidebar .el-menu {
  flex: 1;
  overflow-y: auto;
}

.sidebar-version {
  text-align: center;
  color: var(--text-faint);
  font-size: var(--text-xs);
  margin-top: var(--space-3);
}

/* ---- 主区域 ---- */
.main-area {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
}

.topbar {
  height: var(--topbar-h);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 var(--space-6);
  border-bottom: 1px solid var(--border-faint);
  background: var(--bg-card);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  position: sticky;
  top: 0;
  z-index: 20;
}

.topbar-left {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  min-width: 0;
}

.page-title {
  font-family: var(--font-display);
  font-size: var(--text-lg);
  font-weight: 600;
}

.route-path {
  color: var(--text-faint);
  font-size: var(--text-xs);
}

@media (max-width: 900px) {
  .route-path {
    display: none;
  }
}

.topbar-right {
  display: flex;
  align-items: center;
  gap: var(--space-3);
}

.user-chip {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  cursor: pointer;
  padding: var(--space-1) var(--space-2);
  border-radius: var(--radius-pill);
  transition: background var(--dur-fast) var(--ease-out);
}

.user-chip:hover {
  background: var(--el-fill-color);
}

.avatar {
  width: 32px;
  height: 32px;
  border-radius: var(--radius-pill);
  background-image: var(--grad-primary);
  color: #fff;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-weight: 700;
  font-size: var(--text-sm);
}

.user-name {
  color: var(--text-primary);
  font-size: var(--text-sm);
}

.role-badge {
  color: var(--text-muted);
  font-size: var(--text-xs);
  text-transform: uppercase;
}

@media (max-width: 600px) {
  .user-name,
  .role-badge {
    display: none;
  }
}

/* ---- 内容区 ---- */
.content {
  flex: 1;
  min-width: 0;
}

.page-enter-active,
.page-leave-active {
  transition: opacity var(--dur-base) var(--ease-out), transform var(--dur-base) var(--ease-out);
}
.page-enter-from {
  opacity: 0;
  transform: translateY(8px);
}
.page-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}

/* ---- 移动端 ---- */
.mobile-nav {
  display: none;
}

@media (max-width: 768px) {
  .sidebar {
    display: none;
  }
  .app-layout.collapsed .sidebar {
    width: 0;
  }
  .mobile-nav {
    display: flex;
    position: fixed;
    bottom: 0;
    left: 0;
    right: 0;
    height: calc(var(--mobile-nav-h) + env(safe-area-inset-bottom, 0px));
    padding-bottom: env(safe-area-inset-bottom, 0px);
    background: var(--bg-glass);
    backdrop-filter: blur(16px);
    -webkit-backdrop-filter: blur(16px);
    border-top: 1px solid var(--border);
    z-index: 30;
    overflow-x: auto;
  }
  .mobile-nav-item {
    flex: 1 0 64px;
    min-width: 64px;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 2px;
    text-decoration: none;
    color: var(--text-muted);
    font-size: var(--text-xs);
    transition: color var(--dur-fast) var(--ease-out);
  }
  .mobile-nav-item.active {
    color: var(--indigo-400);
  }
  .content {
    padding-bottom: calc(var(--mobile-nav-h) + env(safe-area-inset-bottom, 0px));
  }
}
</style>
