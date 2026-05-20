<template>
  <div class="dashboard">
    <!-- 功能入口卡片 -->
    <el-row :gutter="20">
      <el-col :span="6" v-for="card in featureCards" :key="card.title">
        <div class="feature-card" @click="$router.push(card.route)">
          <div class="feature-icon" :style="{ background: card.iconBg }">
            <el-icon :size="28" :color="card.iconColor"><component :is="card.icon" /></el-icon>
          </div>
          <div class="feature-info">
            <div class="feature-title">{{ card.title }}</div>
            <div class="feature-desc">{{ card.desc }}</div>
          </div>
        </div>
      </el-col>
    </el-row>

    <!-- 最近操作 -->
    <el-card class="main-card" style="margin-top: 20px;">
      <template #header>
        <div style="display: flex; align-items: center; justify-content: space-between;">
          <span style="font-weight: 600; font-size: 15px;">最近操作</span>
          <el-button text type="primary" @click="$router.push('/logs')">查看全部</el-button>
        </div>
      </template>
      <el-table :data="logs" stripe v-if="logs.length > 0">
        <el-table-column prop="created_at" label="时间" width="180" />
        <el-table-column prop="username" label="操作人" width="100" />
        <el-table-column prop="module" label="模块" width="100">
          <template #default="{ row }">
            <el-tag :type="moduleTagType(row.module)" size="small">{{ moduleLabel(row.module) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="ip" label="IP" width="130" show-overflow-tooltip />
        <el-table-column prop="server_name" label="服务器" width="150" show-overflow-tooltip />
        <el-table-column prop="action" label="动作" width="100" />
        <el-table-column prop="target" label="目标" show-overflow-tooltip />
        <el-table-column prop="status" label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="row.status === 'success' ? 'success' : 'danger'" size="small">{{ row.status === 'success' ? '成功' : '失败' }}</el-tag>
          </template>
        </el-table-column>
      </el-table>
      <div v-else class="empty-state">
        <el-icon class="empty-state-icon"><Document /></el-icon>
        <span class="empty-state-text">暂无操作记录</span>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted, markRaw } from 'vue'
import { getLogs } from '../api'
import { Connection, Document, Box, ZoomOut } from '@element-plus/icons-vue'

const logs = ref([])

const featureCards = [
  { title: 'LVS 管理', desc: 'LVS 负载均衡上下线与切换', route: '/lvs', icon: markRaw(Connection), iconColor: '#409EFF', iconBg: 'rgba(64, 158, 255, 0.1)' },
  { title: 'Nginx 管理', desc: 'Nginx upstream 配置管理', route: '/nginx', icon: markRaw(Document), iconColor: '#67C23A', iconBg: 'rgba(103, 194, 58, 0.1)' },
  { title: 'K8S 发布', desc: 'Kubernetes Argo Rollout 发布', route: '/k8s', icon: markRaw(Box), iconColor: '#E6A23C', iconBg: 'rgba(230, 162, 60, 0.1)' },
  { title: '预生产扩缩容', desc: '预生产环境资源扩缩容', route: '/preprod', icon: markRaw(ZoomOut), iconColor: '#F56C6C', iconBg: 'rgba(245, 108, 108, 0.1)' },
]

const moduleLabels = markRaw({ lvs: 'LVS', nginx: 'Nginx', k8s: 'Kubernetes', preprod: 'K8s-PrePro', auth: '认证', server: '服务器' })
const moduleTagTypes = markRaw({ lvs: '', nginx: 'success', k8s: 'warning', preprod: 'warning', auth: 'danger', server: 'info' })

function moduleLabel(m) { return moduleLabels[m] || m }
function moduleTagType(m) { return moduleTagTypes[m] || '' }

onMounted(async () => {
  try {
    const res = await getLogs({ page: 1, size: 10 })
    logs.value = res.data || []
  } catch (e) {
    console.error('Failed to load logs:', e)
  }
})
</script>

<style scoped>
.dashboard {
  padding: 0;
}

.feature-card {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 24px 20px;
  background: var(--card-bg);
  border-radius: var(--card-radius);
  box-shadow: var(--card-shadow);
  cursor: pointer;
  transition: transform 0.2s, box-shadow 0.2s;
  margin-bottom: 20px;
}

.feature-card:hover {
  transform: translateY(-4px);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.1);
}

.feature-card:active {
  transform: translateY(-1px);
}

.feature-icon {
  width: 52px;
  height: 52px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.feature-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.feature-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--text-primary);
}

.feature-desc {
  font-size: 12px;
  color: var(--text-secondary);
  line-height: 1.4;
}
</style>
