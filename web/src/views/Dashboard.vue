<template>
  <div>
    <el-row :gutter="20">
      <el-col :span="6">
        <el-card shadow="hover" @click="$router.push('/lvs')">
          <template #header>LVS管理</template>
          <div style="text-align: center; font-size: 32px; color: #409EFF;">
            <el-icon><Connection /></el-icon>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover" @click="$router.push('/nginx')">
          <template #header>Nginx管理</template>
          <div style="text-align: center; font-size: 32px; color: #67C23A;">
            <el-icon><Document /></el-icon>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover" @click="$router.push('/k8s')">
          <template #header>K8S发布</template>
          <div style="text-align: center; font-size: 32px; color: #E6A23C;">
            <el-icon><Box /></el-icon>
          </div>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover" @click="$router.push('/preprod')">
          <template #header>预生产缩扩容</template>
          <div style="text-align: center; font-size: 32px; color: #F56C6C;">
            <el-icon><ZoomOut /></el-icon>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <el-card style="margin-top: 20px;">
      <template #header>最近操作</template>
      <el-table :data="logs" stripe>
        <el-table-column prop="created_at" label="时间" width="180" />
        <el-table-column prop="username" label="操作人" width="100" />
        <el-table-column prop="module" label="模块" width="100" />
        <el-table-column prop="action" label="动作" width="100" />
        <el-table-column prop="target" label="目标" />
        <el-table-column prop="status" label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="row.status === 'success' ? 'success' : 'danger'">{{ row.status }}</el-tag>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { getLogs } from '../api'
import { Connection, Document, Box, ZoomOut } from '@element-plus/icons-vue'

const logs = ref([])

onMounted(async () => {
  try {
    const res = await getLogs({ page: 1, size: 10 })
    logs.value = res.data || []
  } catch (e) {
    console.error('Failed to load logs:', e)
  }
})
</script>
