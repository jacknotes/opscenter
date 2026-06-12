<template>
  <el-container style="height: 100vh">
    <!-- 桌面端侧边栏 -->
    <el-aside
      class="desktop-sidebar"
      :width="appStore.isCollapse ? '64px' : '220px'"
      style="background-color: var(--sidebar-bg)"
    >
      <div
        style="
          height: 60px;
          display: flex;
          align-items: center;
          justify-content: center;
          color: #fff;
          font-size: 18px;
          font-weight: bold;
          white-space: nowrap;
          overflow: hidden;
        "
      >
        {{ appStore.isCollapse ? 'OC' : 'OpsCenter' }}
      </div>
      <el-menu
        :default-active="route.path"
        router
        :collapse="appStore.isCollapse"
        :collapse-transition="false"
        background-color="var(--sidebar-bg)"
        text-color="var(--sidebar-text)"
        active-text-color="var(--sidebar-active-text)"
      >
        <el-menu-item index="/dashboard">
          <el-icon class="menu-icon-dashboard"><Monitor /></el-icon>
          <template #title>总览</template>
        </el-menu-item>
        <el-menu-item index="/lvs">
          <el-icon class="menu-icon-lvs"><Connection /></el-icon>
          <template #title>LVS管理</template>
        </el-menu-item>
        <el-menu-item index="/nginx">
          <el-icon class="menu-icon-nginx"><Document /></el-icon>
          <template #title>Nginx管理</template>
        </el-menu-item>
        <el-menu-item index="/k8s">
          <el-icon class="menu-icon-k8s"><Box /></el-icon>
          <template #title>K8S发布</template>
        </el-menu-item>
        <el-menu-item index="/preprod">
          <el-icon class="menu-icon-preprod"><ZoomOut /></el-icon>
          <template #title>预生产扩缩容</template>
        </el-menu-item>
        <el-menu-item index="/logs">
          <el-icon class="menu-icon-log"><List /></el-icon>
          <template #title>日志审计</template>
        </el-menu-item>
        <el-menu-item v-if="userStore.isAdmin" index="/servers">
          <el-icon class="menu-icon-server"><Setting /></el-icon>
          <template #title>服务器管理</template>
        </el-menu-item>
        <el-menu-item v-if="userStore.isAdmin" index="/users">
          <el-icon class="menu-icon-user"><UserFilled /></el-icon>
          <template #title>用户管理</template>
        </el-menu-item>
      </el-menu>

      <div
        style="
          position: absolute;
          bottom: 0;
          width: 100%;
          display: flex;
          justify-content: center;
          padding: 10px 0;
          cursor: pointer;
          color: var(--sidebar-text);
        "
        @click="appStore.toggleCollapse()"
      >
        <el-icon :size="20"><Fold v-if="!appStore.isCollapse" /><Expand v-else /></el-icon>
      </div>
    </el-aside>

    <!-- 手机端抽屉侧边栏 -->
    <el-drawer v-model="drawerVisible" direction="ltr" :size="220" :show-close="false" class="mobile-drawer">
      <template #header>
        <div style="color: #fff; font-size: 18px; font-weight: bold">OpsCenter</div>
      </template>
      <el-menu
        :default-active="route.path"
        router
        :collapse="false"
        :collapse-transition="false"
        background-color="var(--sidebar-bg)"
        text-color="var(--sidebar-text)"
        active-text-color="var(--sidebar-active-text)"
        @select="drawerVisible = false"
      >
        <el-menu-item index="/dashboard">
          <el-icon class="menu-icon-dashboard"><Monitor /></el-icon>
          <template #title>总览</template>
        </el-menu-item>
        <el-menu-item index="/lvs">
          <el-icon class="menu-icon-lvs"><Connection /></el-icon>
          <template #title>LVS管理</template>
        </el-menu-item>
        <el-menu-item index="/nginx">
          <el-icon class="menu-icon-nginx"><Document /></el-icon>
          <template #title>Nginx管理</template>
        </el-menu-item>
        <el-menu-item index="/k8s">
          <el-icon class="menu-icon-k8s"><Box /></el-icon>
          <template #title>K8S发布</template>
        </el-menu-item>
        <el-menu-item index="/preprod">
          <el-icon class="menu-icon-preprod"><ZoomOut /></el-icon>
          <template #title>预生产扩缩容</template>
        </el-menu-item>
        <el-menu-item index="/logs">
          <el-icon class="menu-icon-log"><List /></el-icon>
          <template #title>日志审计</template>
        </el-menu-item>
        <el-menu-item v-if="userStore.isAdmin" index="/servers">
          <el-icon class="menu-icon-server"><Setting /></el-icon>
          <template #title>服务器管理</template>
        </el-menu-item>
        <el-menu-item v-if="userStore.isAdmin" index="/users">
          <el-icon class="menu-icon-user"><UserFilled /></el-icon>
          <template #title>用户管理</template>
        </el-menu-item>
      </el-menu>
    </el-drawer>

    <el-container>
      <el-header class="app-header">
        <div style="display: flex; align-items: center; gap: 10px">
          <el-icon class="mobile-menu-btn" :size="22" @click="drawerVisible = true"><Fold /></el-icon>
          <div class="header-title">{{ route.meta.title }}</div>
        </div>
        <div style="display: flex; align-items: center; gap: 12px">
          <div
            class="theme-toggle"
            :title="appStore.theme === 'dark' ? '切换到浅色模式' : '切换到深色模式'"
            @click="appStore.toggleTheme()"
          >
            <el-icon :size="18"><Sunny v-if="appStore.theme === 'dark'" /><Moon v-else /></el-icon>
          </div>
          <el-popover
            placement="bottom-end"
            trigger="click"
            :width="160"
            popper-class="user-popover"
            :show-after="0"
            :hide-after="0"
          >
            <template #reference>
              <div class="user-avatar-trigger">
                <div class="user-avatar">{{ userInitial }}</div>
                <span class="user-name">{{ userStore.userInfo?.username }}</span>
                <el-icon :size="12"><ArrowDown /></el-icon>
              </div>
            </template>
            <div class="user-menu">
              <div class="user-menu-header">
                <div class="user-menu-avatar">{{ userInitial }}</div>
                <div class="user-menu-info">
                  <div class="user-menu-name">{{ userStore.userInfo?.name || userStore.userInfo?.username }}</div>
                  <div class="user-menu-role">{{ userStore.isAdmin ? '管理员' : '普通用户' }}</div>
                </div>
              </div>
              <div class="user-menu-divider"></div>
              <div class="user-menu-item" @click="handleCommand('profile')">个人信息</div>
              <div class="user-menu-item" @click="handleCommand('changePwd')">修改密码</div>
              <div v-if="userStore.isAdmin" class="user-menu-item" @click="handleCommand('swagger')">API 文档</div>
              <div class="user-menu-divider"></div>
              <div class="user-menu-item user-menu-item--danger" @click="handleCommand('logout')">退出登录</div>
            </div>
          </el-popover>
        </div>
      </el-header>

      <!-- 签名元素：顶部渐变线 -->
      <div class="accent-line"></div>

      <el-main class="content-area">
        <router-view v-slot="{ Component }">
          <transition name="page-fade" mode="out-in">
            <keep-alive>
              <component :is="Component" />
            </keep-alive>
          </transition>
        </router-view>
      </el-main>

      <!-- 底部状态栏 -->
      <div v-if="wsStore.status !== 'idle'" class="status-bar">
        <div class="status-bar-left">
          <span :class="['status-dot-indicator', `status-${wsStore.status}`]"></span>
          <span class="status-bar-text">
            {{ wsStatusLabel }}
          </span>
        </div>
      </div>
    </el-container>
  </el-container>

  <!-- User Profile Dialog -->
  <el-dialog v-model="profileVisible" title="个人信息" width="400px">
    <el-descriptions :column="1" border>
      <el-descriptions-item label="用户名">{{ userStore.userInfo?.username }}</el-descriptions-item>
      <el-descriptions-item label="姓名">{{ userStore.userInfo?.name }}</el-descriptions-item>
      <el-descriptions-item label="邮箱">{{ userStore.userInfo?.email }}</el-descriptions-item>
      <el-descriptions-item label="角色">
        <el-tag :type="userStore.userInfo?.role === 'admin' ? 'danger' : 'info'" size="small">
          {{ userStore.userInfo?.role === 'admin' ? '管理员' : '普通用户' }}
        </el-tag>
      </el-descriptions-item>
    </el-descriptions>
  </el-dialog>

  <!-- Change Password Dialog -->
  <el-dialog v-model="changePwdVisible" title="修改密码" width="400px">
    <el-form :model="changePwdForm" label-width="80px">
      <el-form-item label="原密码" required>
        <el-input v-model="changePwdForm.old_password" type="password" show-password />
      </el-form-item>
      <el-form-item label="新密码" required>
        <el-input v-model="changePwdForm.new_password" type="password" show-password />
      </el-form-item>
      <el-form-item label="确认密码" required>
        <el-input v-model="changePwdForm.confirm_password" type="password" show-password />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="changePwdVisible = false">取消</el-button>
      <el-button type="primary" :loading="changePwdLoading" @click="submitChangePwd">确定</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, computed, onMounted, onActivated, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useUserStore } from '../stores/user'
import { useAppStore } from '../stores/app'
import { getUserInfo, changePassword, logout } from '../api'
import { clearServerCache } from '../composables/useServerSelector'
import { useWebSocketStore } from '../stores/websocket'
import { ElMessage } from 'element-plus'
import {
  Monitor,
  Connection,
  Document,
  Box,
  ZoomOut,
  List,
  Setting,
  Fold,
  Expand,
  UserFilled,
  ArrowDown,
  Sunny,
  Moon,
} from '@element-plus/icons-vue'

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()
const appStore = useAppStore()
const wsStore = useWebSocketStore()

const userInitial = computed(() => {
  const name = userStore.userInfo?.name || userStore.userInfo?.username || ''
  return name.charAt(0).toUpperCase()
})

const wsStatusLabel = computed(() => {
  const map = { connecting: '正在连接...', streaming: '命令执行中...', done: '执行完成', error: '执行异常' }
  return map[wsStore.status] || ''
})

const drawerVisible = ref(false)
const profileVisible = ref(false)
const changePwdVisible = ref(false)
const changePwdLoading = ref(false)
const changePwdForm = ref({ old_password: '', new_password: '', confirm_password: '' })

onMounted(async () => {
  try {
    const info = await getUserInfo()
    userStore.setUserInfo(info)
  } catch (e) {
    console.error('Failed to get user info:', e)
  }
  document.addEventListener('keydown', handleKeydown)
})

// keep-alive 缓存场景：重新激活时刷新用户信息（如 session 过期重新登录后）
onActivated(async () => {
  try {
    const info = await getUserInfo()
    userStore.setUserInfo(info)
  } catch (e) {
    // 静默失败，不影响页面使用
  }
})

onUnmounted(() => {
  document.removeEventListener('keydown', handleKeydown)
})

function handleKeydown(e) {
  // 跳过输入框内的按键
  const tag = e.target.tagName
  if (tag === 'INPUT' || tag === 'TEXTAREA' || e.target.isContentEditable) return

  // r 刷新当前页面
  if (e.key === 'r' && !e.metaKey && !e.ctrlKey && !e.altKey) {
    e.preventDefault()
    window.dispatchEvent(new CustomEvent('page-refresh'))
  }
}

async function handleCommand(cmd) {
  if (cmd === 'profile') {
    profileVisible.value = true
  } else if (cmd === 'logout') {
    // 先调用后端登出 API（需要 token），确保完成后再清理本地状态并跳转
    clearServerCache()
    try {
      await logout()
    } catch {
      // 即使 API 失败也继续清理
    }
    userStore.logout()
    router.push('/login').catch(() => {})
  } else if (cmd === 'changePwd') {
    changePwdForm.value = { old_password: '', new_password: '', confirm_password: '' }
    changePwdVisible.value = true
  } else if (cmd === 'swagger') {
    const base = import.meta.env.DEV ? 'http://localhost:18080' : ''
    window.open(base + '/swagger/index.html', '_blank')
  }
}

async function submitChangePwd() {
  if (!changePwdForm.value.old_password || !changePwdForm.value.new_password) {
    ElMessage.warning('请填写完整')
    return
  }
  if (changePwdForm.value.new_password !== changePwdForm.value.confirm_password) {
    ElMessage.warning('两次输入的新密码不一致')
    return
  }
  changePwdLoading.value = true
  try {
    const { old_password, new_password } = changePwdForm.value
    await changePassword(userStore.userInfo.id, { old_password, new_password })
    ElMessage.success('密码修改成功')
    changePwdVisible.value = false
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '修改失败')
  } finally {
    changePwdLoading.value = false
  }
}
</script>

<style scoped>
.el-aside {
  position: relative;
  overflow: hidden;
  background: var(--sidebar-bg);
  border-right: 1px solid var(--border-default);
}

/* ===== Header ===== */
.app-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid var(--border-default);
  background: var(--card-bg);
  position: sticky;
  top: 0;
  z-index: 100;
}

.header-title {
  font-size: var(--font-lg);
  font-weight: var(--weight-medium);
  color: var(--text-primary);
}

/* ===== 用户头像触发器 ===== */
.user-avatar-trigger {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  padding: 4px 8px;
  border-radius: 8px;
  transition: background 0.2s ease;
}
.user-avatar-trigger:hover {
  background: var(--bg-elevated);
}
.user-avatar {
  width: 28px;
  height: 28px;
  border-radius: 6px;
  background: linear-gradient(135deg, #06b6d4, #0891b2);
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  font-size: var(--font-sm);
  font-weight: var(--weight-bold);
  flex-shrink: 0;
}
.user-name {
  font-size: var(--font-sm);
  color: var(--text-primary);
  font-weight: var(--weight-medium);
}

/* Sidebar active indicator — wider with glow */
:deep(.el-menu-item.is-active) {
  background-color: var(--sidebar-active-bg) !important;
  position: relative;
}

:deep(.el-menu-item.is-active::before) {
  content: '';
  position: absolute;
  left: 0;
  top: 50%;
  transform: translateY(-50%);
  width: 3px;
  height: 65%;
  background: var(--sidebar-active-indicator);
  border-radius: 0 3px 3px 0;
  box-shadow:
    0 0 8px rgba(6, 182, 212, 0.5),
    0 0 16px rgba(6, 182, 212, 0.2);
}

:deep(.el-menu-item:hover) {
  background-color: rgba(255, 255, 255, 0.04) !important;
}

/* 模块色彩图标 */
.menu-icon-dashboard {
  color: var(--color-primary) !important;
}
.menu-icon-lvs {
  color: var(--module-lvs) !important;
}
.menu-icon-nginx {
  color: var(--module-nginx) !important;
}
.menu-icon-k8s {
  color: var(--module-k8s) !important;
}
.menu-icon-preprod {
  color: var(--module-preprod) !important;
}
.menu-icon-log {
  color: var(--module-log) !important;
}
.menu-icon-server {
  color: var(--module-server) !important;
}
.menu-icon-user {
  color: var(--module-user) !important;
}

/* Mobile: hide desktop sidebar, show hamburger */
.mobile-menu-btn {
  display: none;
}

@media (max-width: 768px) {
  .desktop-sidebar {
    display: none !important;
  }
  .mobile-menu-btn {
    display: block;
    cursor: pointer;
  }
}

:deep(.mobile-drawer .el-drawer__header) {
  background: var(--sidebar-bg);
  color: #fff;
  margin-bottom: 0;
  padding: 16px 20px;
}

:deep(.mobile-drawer .el-drawer__body) {
  padding: 0;
  background: var(--sidebar-bg);
}

.user-menu {
  display: flex;
  flex-direction: column;
  margin: 0;
}

.user-menu-header {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
}

.user-menu-avatar {
  width: 32px;
  height: 32px;
  border-radius: 8px;
  background: linear-gradient(135deg, #06b6d4, #0891b2);
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  font-size: 14px;
  font-weight: 700;
  flex-shrink: 0;
}

.user-menu-info {
  min-width: 0;
}

.user-menu-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.user-menu-role {
  font-size: 11px;
  color: var(--text-secondary);
}

.user-menu-divider {
  height: 1px;
  background: var(--border-default);
  margin: 4px 0;
}

.user-menu-item {
  padding: 6px 12px;
  cursor: pointer;
  font-size: 13px;
  color: var(--text-regular);
  transition: background 0.2s;
  border-radius: 4px;
  margin: 0 4px;
}

.user-menu-item:hover {
  background: rgba(6, 182, 212, 0.1);
  color: var(--color-primary);
}

.user-menu-item--danger {
  color: #ef4444;
}

.user-menu-item--danger:hover {
  background: rgba(239, 68, 68, 0.1);
  color: #ef4444;
}

/* 主题切换按钮 */
.theme-toggle {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border-radius: 8px;
  cursor: pointer;
  color: #94a3b8;
  transition:
    background 0.2s,
    color 0.2s;
}

.theme-toggle:hover {
  background: rgba(6, 182, 212, 0.1);
  color: #06b6d4;
}

/* 底部状态栏 */
.status-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 4px 20px;
  background: var(--bg-elevated);
  border-top: 1px solid var(--border-default);
  flex-shrink: 0;
  height: 28px;
}

.status-bar-left {
  display: flex;
  align-items: center;
  gap: 8px;
}

.status-bar-text {
  font-size: 12px;
  color: var(--text-secondary);
}

.status-dot-indicator {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
  transition: all 0.3s ease;
}

.status-dot-indicator.status-connecting {
  background: #f59e0b;
  box-shadow: 0 0 6px rgba(245, 158, 11, 0.6);
  animation: pulse-glow-amber 1.5s infinite;
}

.status-dot-indicator.status-streaming {
  background: #22c55e;
  box-shadow: 0 0 6px rgba(34, 197, 94, 0.6);
  animation: pulse-glow-green 1s infinite;
}

.status-dot-indicator.status-done {
  background: #22c55e;
  box-shadow: 0 0 4px rgba(34, 197, 94, 0.4);
}

.status-dot-indicator.status-error {
  background: #ef4444;
}

@keyframes pulse {
  0%,
  100% {
    opacity: 1;
  }
  50% {
    opacity: 0.4;
  }
}

@keyframes pulse-glow-green {
  0%,
  100% {
    opacity: 1;
    box-shadow: 0 0 6px rgba(34, 197, 94, 0.6);
  }
  50% {
    opacity: 0.5;
    box-shadow: 0 0 12px rgba(34, 197, 94, 0.8);
  }
}

@keyframes pulse-glow-amber {
  0%,
  100% {
    opacity: 1;
    box-shadow: 0 0 6px rgba(245, 158, 11, 0.6);
  }
  50% {
    opacity: 0.5;
    box-shadow: 0 0 12px rgba(245, 158, 11, 0.8);
  }
}

/* ===== 签名元素：顶部渐变线 ===== */
.accent-line {
  height: 2px;
  background: var(--accent-line);
  background-size: 200% 100%;
  animation: accent-shift 8s ease-in-out infinite;
  flex-shrink: 0;
  opacity: 0.7;
  will-change: background-position;
  position: sticky;
  top: 60px;
  z-index: 99;
}

@keyframes accent-shift {
  0%,
  100% {
    background-position: 0% 50%;
  }
  50% {
    background-position: 100% 50%;
  }
}

/* ===== 内容区：点阵纹理背景 ===== */
.content-area {
  background-color: var(--content-bg);
  background-image: var(--dot-grid);
  background-size: 24px 24px;
  padding: 20px;
  position: relative;
}
</style>

<style>
.user-popover.el-popover {
  padding: 4px 0 !important;
  min-width: 160px !important;
  width: 160px !important;
}

html.dark .user-popover.el-popover {
  background: #1a1d2e !important;
  border: 1px solid rgba(255, 255, 255, 0.06) !important;
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.4) !important;
}

html:not(.dark) .user-popover.el-popover {
  background: #ffffff !important;
  border: 1px solid #e4e7ed !important;
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.12) !important;
}
</style>
