<template>
  <el-container style="height: 100vh">
    <!-- 桌面端侧边栏 -->
    <el-aside class="desktop-sidebar" :width="appStore.isCollapse ? '64px' : '220px'" style="background-color: var(--sidebar-bg);">
      <div style="height: 60px; display: flex; align-items: center; justify-content: center; color: #fff; font-size: 18px; font-weight: bold; white-space: nowrap; overflow: hidden;">
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
          <el-icon><Monitor /></el-icon>
          <template #title>总览</template>
        </el-menu-item>
        <el-menu-item index="/lvs">
          <el-icon><Connection /></el-icon>
          <template #title>LVS管理</template>
        </el-menu-item>
        <el-menu-item index="/nginx">
          <el-icon><Document /></el-icon>
          <template #title>Nginx管理</template>
        </el-menu-item>
        <el-menu-item index="/k8s">
          <el-icon><Box /></el-icon>
          <template #title>K8S发布</template>
        </el-menu-item>
        <el-menu-item index="/preprod">
          <el-icon><ZoomOut /></el-icon>
          <template #title>预生产扩缩容</template>
        </el-menu-item>
        <el-menu-item index="/logs">
          <el-icon><List /></el-icon>
          <template #title>日志审计</template>
        </el-menu-item>
        <el-menu-item v-if="userStore.isAdmin" index="/servers">
          <el-icon><Setting /></el-icon>
          <template #title>服务器管理</template>
        </el-menu-item>
        <el-menu-item v-if="userStore.isAdmin" index="/users">
          <el-icon><UserFilled /></el-icon>
          <template #title>用户管理</template>
        </el-menu-item>
      </el-menu>

      <div style="position: absolute; bottom: 0; width: 100%; display: flex; justify-content: center; padding: 10px 0; cursor: pointer; color: var(--sidebar-text);" @click="appStore.toggleCollapse()">
        <el-icon :size="20"><Fold v-if="!appStore.isCollapse" /><Expand v-else /></el-icon>
      </div>
    </el-aside>

    <!-- 手机端抽屉侧边栏 -->
    <el-drawer v-model="drawerVisible" direction="ltr" :size="220" :show-close="false" class="mobile-drawer">
      <template #header>
        <div style="color: #fff; font-size: 18px; font-weight: bold;">OpsCenter</div>
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
          <el-icon><Monitor /></el-icon>
          <template #title>总览</template>
        </el-menu-item>
        <el-menu-item index="/lvs">
          <el-icon><Connection /></el-icon>
          <template #title>LVS管理</template>
        </el-menu-item>
        <el-menu-item index="/nginx">
          <el-icon><Document /></el-icon>
          <template #title>Nginx管理</template>
        </el-menu-item>
        <el-menu-item index="/k8s">
          <el-icon><Box /></el-icon>
          <template #title>K8S发布</template>
        </el-menu-item>
        <el-menu-item index="/preprod">
          <el-icon><ZoomOut /></el-icon>
          <template #title>预生产扩缩容</template>
        </el-menu-item>
        <el-menu-item index="/logs">
          <el-icon><List /></el-icon>
          <template #title>日志审计</template>
        </el-menu-item>
        <el-menu-item v-if="userStore.isAdmin" index="/servers">
          <el-icon><Setting /></el-icon>
          <template #title>服务器管理</template>
        </el-menu-item>
        <el-menu-item v-if="userStore.isAdmin" index="/users">
          <el-icon><UserFilled /></el-icon>
          <template #title>用户管理</template>
        </el-menu-item>
      </el-menu>
    </el-drawer>

    <el-container>
      <el-header style="display: flex; align-items: center; justify-content: space-between; border-bottom: 1px solid var(--border-default); background: var(--card-bg);">
        <div style="display: flex; align-items: center; gap: 10px;">
          <el-icon class="mobile-menu-btn" :size="22" @click="drawerVisible = true"><Fold /></el-icon>
          <div style="font-size: 16px; font-weight: 500;">{{ route.meta.title }}</div>
        </div>
        <div style="display: flex; align-items: center; gap: 16px;">
          <div class="theme-toggle" @click="appStore.toggleTheme()" :title="appStore.theme === 'dark' ? '切换到浅色模式' : '切换到深色模式'">
            <el-icon :size="18"><Sunny v-if="appStore.theme === 'dark'" /><Moon v-else /></el-icon>
          </div>
          <el-popover placement="bottom-end" trigger="click" :width="100" popper-class="user-popover" :show-after="0" :hide-after="0">
            <template #reference>
              <span style="cursor: pointer;">{{ userStore.userInfo?.username }} <el-icon><ArrowDown /></el-icon></span>
            </template>
            <div class="user-menu">
              <div class="user-menu-item" @click="handleCommand('profile')">个人信息</div>
              <div class="user-menu-item" @click="handleCommand('changePwd')">修改密码</div>
              <div class="user-menu-item user-menu-item--danger" @click="handleCommand('logout')">退出登录</div>
            </div>
          </el-popover>
        </div>
      </el-header>

      <el-main style="background-color: var(--content-bg); padding: 20px;">
        <router-view />
      </el-main>
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
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useUserStore } from '../stores/user'
import { useAppStore } from '../stores/app'
import { getUserInfo, changePassword, logout } from '../api'
import { ElMessage } from 'element-plus'
import { Monitor, Connection, Document, Box, ZoomOut, List, Setting, Fold, Expand, UserFilled, ArrowDown, Sunny, Moon } from '@element-plus/icons-vue'

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()
const appStore = useAppStore()

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
})

function handleCommand(cmd) {
  if (cmd === 'profile') {
    profileVisible.value = true
  } else if (cmd === 'logout') {
    logout().catch(() => {}).finally(() => {
      userStore.logout()
      router.push('/login').catch(() => {})
    })
  } else if (cmd === 'changePwd') {
    changePwdForm.value = { old_password: '', new_password: '', confirm_password: '' }
    changePwdVisible.value = true
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
}

/* Sidebar active indicator */
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
  height: 60%;
  background: var(--sidebar-active-indicator);
  border-radius: 0 2px 2px 0;
}

:deep(.el-menu-item:hover) {
  background-color: rgba(255, 255, 255, 0.04) !important;
}

/* Mobile: hide desktop sidebar, show hamburger */
.mobile-menu-btn { display: none; }

@media (max-width: 768px) {
  .desktop-sidebar { display: none !important; }
  .mobile-menu-btn { display: block; cursor: pointer; }
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

.user-menu-item {
  padding: 6px 0;
  cursor: pointer;
  font-size: 13px;
  color: var(--text-regular);
  transition: background 0.2s;
  text-align: center;
  border-radius: 4px;
}

.user-menu-item:hover {
  background: rgba(6, 182, 212, 0.1);
  color: var(--color-primary);
}

.user-menu-item--danger {
  color: #EF4444;
}

.user-menu-item--danger:hover {
  background: rgba(239, 68, 68, 0.1);
  color: #EF4444;
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
  color: #94A3B8;
  transition: background 0.2s, color 0.2s;
}

.theme-toggle:hover {
  background: rgba(6, 182, 212, 0.1);
  color: #06B6D4;
}
</style>

<style>
.user-popover.el-popover {
  padding: 4px 0 !important;
  min-width: 120px !important;
  width: 120px !important;
}

html.dark .user-popover.el-popover {
  background: #1A1D2E !important;
  border: 1px solid rgba(255, 255, 255, 0.06) !important;
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.4) !important;
}

html:not(.dark) .user-popover.el-popover {
  background: #ffffff !important;
  border: 1px solid #e4e7ed !important;
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.12) !important;
}
</style>
