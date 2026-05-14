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
          <template #title>操作日志</template>
        </el-menu-item>
        <el-menu-item v-if="userStore.isAdmin" index="/servers">
          <el-icon><Setting /></el-icon>
          <template #title>服务器管理</template>
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
          <span>{{ userStore.userInfo?.username }}</span>
          <el-button type="danger" link @click="handleLogout">退出</el-button>
        </div>
      </el-header>

      <el-main style="background-color: #f5f7fa; padding: 20px;">
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useUserStore } from '../stores/user'
import { getUserInfo } from '../api'
import { Monitor, Connection, Document, Box, ZoomOut, List, Setting, Fold, Expand } from '@element-plus/icons-vue'

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()
const isCollapse = ref(false)

onMounted(async () => {
  try {
    const info = await getUserInfo()
    userStore.setUserInfo(info)
  } catch (e) {
    console.error('Failed to get user info:', e)
  }
})

function handleLogout() {
  userStore.logout()
  router.push('/login')
}
</script>

<style scoped>
.el-aside {
  position: relative;
  overflow: hidden;
}
</style>
