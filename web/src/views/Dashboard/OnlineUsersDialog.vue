<script setup lang="ts">
import { ref, watch } from 'vue'
import { dashboardApi } from '@/api'
import type { OnlineUser } from '@/api/types'
import { extractErrorMessage } from '@/api/client'
import { i18n } from '@/i18n'

const t = i18n.global.t

const visible = defineModel<boolean>('visible', { default: false })

const loading = ref(false)
const list = ref<OnlineUser[]>([])

async function load(): Promise<void> {
  loading.value = true
  try {
    const res = await dashboardApi.onlineUsers()
    list.value = res.users ?? []
  } catch (err) {
    list.value = []
    console.warn(extractErrorMessage(err, t('dashboard.remoteError')))
  } finally {
    loading.value = false
  }
}

watch(visible, (v) => {
  if (v) void load()
})
</script>

<template>
  <el-dialog
    v-model="visible"
    :title="t('dashboard.onlineUsers')"
    width="640px"
    :close-on-click-modal="true"
  >
    <el-table :data="list" v-loading="loading" stripe style="width: 100%">
      <el-table-column prop="username" label="用户名" min-width="100" />
      <el-table-column prop="role" label="角色" width="90">
        <template #default="{ row }">
          <el-tag :type="row.role === 'admin' ? 'danger' : 'info'" size="small">
            {{ row.role === 'admin' ? '管理员' : '普通用户' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="login_method" label="登录方式" width="100">
        <template #default="{ row }">
          <el-tag :type="row.login_method === 'ldap' ? 'warning' : 'success'" size="small" effect="plain">
            {{ row.login_method === 'ldap' ? 'LDAP' : '本地' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="login_time" label="登录时间" min-width="160">
        <template #default="{ row }">
          {{ row.login_time ? new Date(row.login_time).toLocaleString('zh-CN', { hour12: false }) : '-' }}
        </template>
      </el-table-column>
      <el-table-column prop="last_active" label="最近活跃" min-width="160">
        <template #default="{ row }">
          {{ row.last_active ? new Date(row.last_active).toLocaleString('zh-CN', { hour12: false }) : '-' }}
        </template>
      </el-table-column>
    </el-table>
    <template #footer>
      <el-button :loading="loading" @click="load">{{ t('common.refresh') }}</el-button>
      <el-button @click="visible = false">关闭</el-button>
    </template>
  </el-dialog>
</template>
