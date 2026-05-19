<template>
  <el-container style="height: 100vh">
    <el-aside :width="isCollapse ? '64px' : '220px'" style="background-color: #304156;">
      <div style="height: 60px; display: flex; align-items: center; justify-content: center; color: #fff; font-size: 18px; font-weight: bold; white-space: nowrap; overflow: hidden;">
        {{ isCollapse ? 'OC' : 'OpsCenter' }}
      </div>
      <el-menu
        :default-active="route.path"
        router
        :collapse="isCollapse"
        :collapse-transition="false"
        background-color="#304156"
        text-color="#bfcbd9"
        active-text-color="#409EFF"
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

      <div style="position: absolute; bottom: 0; width: 100%; display: flex; justify-content: center; padding: 10px 0; cursor: pointer; color: #bfcbd9;" @click="isCollapse = !isCollapse">
        <el-icon :size="20"><Fold v-if="!isCollapse" /><Expand v-else /></el-icon>
      </div>
    </el-aside>

    <el-container>
      <el-header style="display: flex; align-items: center; justify-content: space-between; border-bottom: 1px solid #eee; background: #fff;">
        <div style="font-size: 16px; font-weight: 500;">{{ route.meta.title }}</div>
        <div style="display: flex; align-items: center; gap: 16px;">
          <el-dropdown @command="handleCommand">
            <span style="cursor: pointer;">{{ userStore.userInfo?.username }} <el-icon><ArrowDown /></el-icon></span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="changePwd">修改密码</el-dropdown-item>
                <el-dropdown-item command="logout" divided>退出登录</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </el-header>

      <el-main style="background-color: #f5f7fa; padding: 20px;">
        <router-view />
      </el-main>
    </el-container>
  </el-container>

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
import { getUserInfo, changePassword, logout } from '../api'
import { ElMessage } from 'element-plus'
import { Monitor, Connection, Document, Box, ZoomOut, List, Setting, Fold, Expand, UserFilled, ArrowDown } from '@element-plus/icons-vue'

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()
const isCollapse = ref(false)

const changePwdVisible = ref(false)
const changePwdLoading = ref(false)
const changePwdForm = ref({ old_password: '', new_password: '' })

onMounted(async () => {
  try {
    const info = await getUserInfo()
    userStore.setUserInfo(info)
  } catch (e) {
    console.error('Failed to get user info:', e)
  }
})

function handleCommand(cmd) {
  if (cmd === 'logout') {
    logout().catch(() => {})
    userStore.logout()
    router.push('/login')
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
    await changePassword(userStore.userInfo.id, changePwdForm.value)
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
</style>
