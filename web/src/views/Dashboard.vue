<template>
  <div class="dashboard">
    <!-- 统计卡片网格 -->
    <el-row :gutter="20">
      <!-- 服务器管理（admin） -->
      <el-col v-if="userStore.isAdmin" :xs="24" :sm="12" :md="8">
        <div class="stat-card" @click="$router.push('/servers')">
          <div class="stat-header">
            <div class="stat-icon" style="background: rgba(100,116,139,0.1)">
              <el-icon :size="22" color="#64748B"><Monitor /></el-icon>
            </div>
            <span class="stat-title">服务器管理</span>
            <el-icon class="stat-arrow"><ArrowRight /></el-icon>
          </div>
          <div class="stat-body" v-if="!statsLoading && serverStats">
            <div class="stat-row">
              <span class="stat-label">总数</span>
              <span class="stat-value">{{ serverStats.total }}</span>
            </div>
            <div class="stat-row">
              <span class="stat-label">启用 / 禁用</span>
              <span class="stat-value">
                <span class="text-success">{{ serverStats.enabled }}</span>
                <span class="stat-divider">/</span>
                <span class="text-danger">{{ serverStats.disabled }}</span>
              </span>
            </div>
            <div class="stat-tags">
              <el-tag v-for="(count, type) in serverStats.by_type" :key="type" size="small" type="info" class="stat-tag">
                {{ typeLabel(type) }}: {{ count }}
              </el-tag>
            </div>
          </div>
          <div class="stat-body" v-else-if="statsLoading">
            <el-skeleton :rows="2" animated />
          </div>
          <div class="stat-body stat-error" v-else>
            <span>加载失败</span>
            <el-button text type="primary" size="small" @click.stop="loadStats">重试</el-button>
          </div>
        </div>
      </el-col>

      <!-- 用户管理（admin） -->
      <el-col v-if="userStore.isAdmin" :xs="24" :sm="12" :md="8">
        <div class="stat-card" @click="$router.push('/users')">
          <div class="stat-header">
            <div class="stat-icon" style="background: rgba(100,116,139,0.1)">
              <el-icon :size="22" color="#64748B"><User /></el-icon>
            </div>
            <span class="stat-title">用户管理</span>
            <el-icon class="stat-arrow"><ArrowRight /></el-icon>
          </div>
          <div class="stat-body" v-if="!statsLoading && userStats">
            <div class="stat-row">
              <span class="stat-label">总数</span>
              <span class="stat-value">{{ userStats.total }}</span>
            </div>
            <div class="stat-row">
              <span class="stat-label">启用 / 禁用</span>
              <span class="stat-value">
                <span class="text-success">{{ userStats.enabled }}</span>
                <span class="stat-divider">/</span>
                <span class="text-danger">{{ userStats.disabled }}</span>
              </span>
            </div>
            <div class="stat-tags">
              <el-tag v-for="(count, role) in userStats.by_role" :key="role" size="small" :type="role === 'admin' ? 'danger' : 'info'" class="stat-tag">
                {{ role === 'admin' ? '管理员' : '普通用户' }}: {{ count }}
              </el-tag>
            </div>
          </div>
          <div class="stat-body" v-else-if="statsLoading">
            <el-skeleton :rows="2" animated />
          </div>
          <div class="stat-body stat-error" v-else>
            <span>加载失败</span>
            <el-button text type="primary" size="small" @click.stop="loadStats">重试</el-button>
          </div>
        </div>
      </el-col>

      <!-- LVS 管理 -->
      <el-col :xs="24" :sm="12" :md="userStore.isAdmin ? 8 : 12">
        <div class="stat-card" @click="$router.push('/lvs')">
          <div class="stat-header">
            <div class="stat-icon" style="background: rgba(6,182,212,0.1)">
              <el-icon :size="22" color="#06B6D4"><Connection /></el-icon>
            </div>
            <span class="stat-title">LVS 管理</span>
            <el-icon class="stat-arrow"><ArrowRight /></el-icon>
          </div>
          <div class="stat-body" v-if="!remoteLoading && lvsStats">
            <div class="stat-row">
              <span class="stat-label">VirtualServer</span>
              <span class="stat-value">{{ lvsStats.vs_count }}</span>
            </div>
            <div class="stat-row">
              <span class="stat-label">RealServer 在线 / 离线</span>
              <span class="stat-value">
                <span class="text-success">{{ lvsStats.rs_online }}</span>
                <span class="stat-divider">/</span>
                <span class="text-danger">{{ lvsStats.rs_offline }}</span>
              </span>
            </div>
            <div class="stat-row">
              <span class="stat-label">ActiveConn / InActConn</span>
              <span class="stat-value">
                <span>{{ lvsStats.total_active_conn }}</span>
                <span class="stat-divider">/</span>
                <span>{{ lvsStats.total_inact_conn }}</span>
              </span>
            </div>
          </div>
          <div class="stat-body" v-else-if="remoteLoading">
            <el-skeleton :rows="2" animated />
          </div>
          <div class="stat-body stat-error" v-else>
            <span>{{ remoteError || '加载失败' }}</span>
            <el-button text type="primary" size="small" @click.stop="loadRemoteStats">重试</el-button>
          </div>
        </div>
      </el-col>

      <!-- Nginx 管理 -->
      <el-col :xs="24" :sm="12" :md="userStore.isAdmin ? 8 : 12">
        <div class="stat-card" @click="$router.push('/nginx')">
          <div class="stat-header">
            <div class="stat-icon" style="background: rgba(34,197,94,0.1)">
              <el-icon :size="22" color="#22C55E"><Document /></el-icon>
            </div>
            <span class="stat-title">Nginx 管理</span>
            <el-icon class="stat-arrow"><ArrowRight /></el-icon>
          </div>
          <div class="stat-body" v-if="!remoteLoading && nginxStats">
            <div class="stat-row">
              <span class="stat-label">Upstream 组</span>
              <span class="stat-value">{{ nginxStats.upstream_count }}</span>
            </div>
            <div class="stat-row">
              <span class="stat-label">Server 在线 / 离线</span>
              <span class="stat-value">
                <span class="text-success">{{ nginxStats.server_online }}</span>
                <span class="stat-divider">/</span>
                <span class="text-danger">{{ nginxStats.server_offline }}</span>
              </span>
            </div>
          </div>
          <div class="stat-body" v-else-if="remoteLoading">
            <el-skeleton :rows="2" animated />
          </div>
          <div class="stat-body stat-error" v-else>
            <span>{{ remoteError || '加载失败' }}</span>
            <el-button text type="primary" size="small" @click.stop="loadRemoteStats">重试</el-button>
          </div>
        </div>
      </el-col>

      <!-- K8S 发布 -->
      <el-col :xs="24" :sm="12" :md="userStore.isAdmin ? 8 : 12">
        <div class="stat-card" @click="$router.push('/k8s')">
          <div class="stat-header">
            <div class="stat-icon" style="background: rgba(245,158,11,0.1)">
              <el-icon :size="22" color="#F59E0B"><Box /></el-icon>
            </div>
            <span class="stat-title">K8S 发布</span>
            <el-icon class="stat-arrow"><ArrowRight /></el-icon>
          </div>
          <div class="stat-body" v-if="!remoteLoading && k8sStats">
            <div class="stat-row">
              <span class="stat-label">Rollout 总数</span>
              <span class="stat-value">{{ k8sStats.total_rollouts }}</span>
            </div>
            <div class="stat-row">
              <span class="stat-label">待发布 / 已发布</span>
              <span class="stat-value">
                <span class="text-warning">{{ k8sStats.pending }}</span>
                <span class="stat-divider">/</span>
                <span class="text-success">{{ k8sStats.online }}</span>
              </span>
            </div>
            <div class="stat-tags" v-if="k8sStats.by_namespace && Object.keys(k8sStats.by_namespace).length > 0">
              <el-tag v-for="(count, ns) in k8sStats.by_namespace" :key="ns" size="small" type="warning" class="stat-tag">
                {{ ns }}: {{ count }}
              </el-tag>
            </div>
          </div>
          <div class="stat-body" v-else-if="remoteLoading">
            <el-skeleton :rows="2" animated />
          </div>
          <div class="stat-body stat-error" v-else>
            <span>{{ remoteError || '加载失败' }}</span>
            <el-button text type="primary" size="small" @click.stop="loadRemoteStats">重试</el-button>
          </div>
        </div>
      </el-col>

      <!-- 预生产扩缩容 -->
      <el-col :xs="24" :sm="12" :md="userStore.isAdmin ? 8 : 12">
        <div class="stat-card" @click="$router.push('/preprod')">
          <div class="stat-header">
            <div class="stat-icon" style="background: rgba(239,68,68,0.1)">
              <el-icon :size="22" color="#EF4444"><ZoomOut /></el-icon>
            </div>
            <span class="stat-title">预生产扩缩容</span>
            <el-icon class="stat-arrow"><ArrowRight /></el-icon>
          </div>
          <div class="stat-body" v-if="!remoteLoading && preprodStats">
            <div class="stat-row">
              <span class="stat-label">资源总数</span>
              <span class="stat-value">{{ preprodStats.total_resources }}</span>
            </div>
            <div class="stat-row">
              <span class="stat-label">已缩容 / 已扩容 / 正常</span>
              <span class="stat-value">
                <span class="text-danger">{{ preprodStats.scaled_down }}</span>
                <span class="stat-divider">/</span>
                <span class="text-warning">{{ preprodStats.expanded }}</span>
                <span class="stat-divider">/</span>
                <span class="text-success">{{ preprodStats.normal }}</span>
              </span>
            </div>
          </div>
          <div class="stat-body" v-else-if="remoteLoading">
            <el-skeleton :rows="2" animated />
          </div>
          <div class="stat-body stat-error" v-else>
            <span>{{ remoteError || '加载失败' }}</span>
            <el-button text type="primary" size="small" @click.stop="loadRemoteStats">重试</el-button>
          </div>
        </div>
      </el-col>
    </el-row>

    <!-- 最近操作 -->
    <el-card class="main-card" style="margin-top: 20px;">
      <template #header>
        <div style="display: flex; align-items: center; justify-content: space-between;">
          <span style="font-weight: 600; font-size: 15px;">最近操作</span>
          <div style="display: flex; align-items: center; gap: 8px;">
            <el-button text type="primary" size="small" :loading="remoteLoading" @click="loadRemoteStats">
              <el-icon style="margin-right: 4px"><Refresh /></el-icon>
              刷新远程数据
            </el-button>
            <el-button text type="primary" @click="$router.push('/logs')">查看全部</el-button>
          </div>
        </div>
      </template>
      <el-table :data="logs" stripe v-if="logs.length > 0" v-force-reflow>
        <el-table-column label="时间" width="180">
          <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
        </el-table-column>
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
import { getLogs, getDashboardStats, getDashboardRemoteStats } from '../api'
import { useUserStore } from '../stores/user'
import { ElMessage } from 'element-plus'
import { Connection, Document, Box, ZoomOut, Monitor, User, ArrowRight, Refresh } from '@element-plus/icons-vue'

const userStore = useUserStore()

const logs = ref([])

// MySQL 统计（即时）
const statsLoading = ref(true)
const serverStats = ref(null)
const userStats = ref(null)

// SSH 远程统计（慢）
const remoteLoading = ref(true)
const remoteError = ref(null)
const lvsStats = ref(null)
const nginxStats = ref(null)
const k8sStats = ref(null)
const preprodStats = ref(null)

const typeLabels = { lvs: 'LVS', nginx: 'Nginx', kubernetes: 'K8S', preprod: 'Preprod' }
function typeLabel(type) { return typeLabels[type] || type }

const moduleLabels = markRaw({ lvs: 'LVS', nginx: 'Nginx', k8s: 'Kubernetes', preprod: 'K8s-PrePro', auth: '认证', server: '服务器' })
const moduleTagTypes = markRaw({ lvs: '', nginx: 'success', k8s: 'warning', preprod: 'warning', auth: 'danger', server: 'info' })

function formatTime(t) {
  if (!t) return ''
  return new Date(t).toLocaleString('zh-CN', { timeZone: 'Asia/Shanghai' })
}

function moduleLabel(m) { return moduleLabels[m] || m }
function moduleTagType(m) { return moduleTagTypes[m] || '' }

async function loadStats() {
  statsLoading.value = true
  try {
    const res = await getDashboardStats()
    serverStats.value = res.servers || null
    userStats.value = res.users || null
  } catch (e) {
    ElMessage.error('加载 MySQL 统计失败')
  } finally {
    statsLoading.value = false
  }
}

async function loadRemoteStats() {
  remoteLoading.value = true
  remoteError.value = null
  try {
    const res = await getDashboardRemoteStats()
    lvsStats.value = res.lvs || null
    nginxStats.value = res.nginx || null
    k8sStats.value = res.k8s || null
    preprodStats.value = res.preprod || null
  } catch (e) {
    ElMessage.error('加载远程统计失败')
    remoteError.value = '远程数据加载失败'
  } finally {
    remoteLoading.value = false
  }
}

onMounted(async () => {
  // 并行加载 MySQL 统计、SSH 远程统计、操作日志
  loadStats()
  loadRemoteStats()
  try {
    const res = await getLogs({ page: 1, size: 10 })
    logs.value = res.data || []
  } catch (e) {
    ElMessage.error('加载日志失败')
  }
})
</script>

<style scoped>
.dashboard {
  padding: 0;
}

.stat-card {
  background: var(--card-bg);
  border-radius: var(--card-radius);
  border: var(--card-border);
  cursor: pointer;
  transition: transform 0.2s, border-color 0.2s;
  margin-bottom: 20px;
  overflow: hidden;
}

.stat-card:hover {
  transform: translateY(-4px);
  border-color: rgba(6, 182, 212, 0.2);
}

.stat-card:active {
  transform: translateY(-1px);
}

.stat-header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 16px 20px 0;
}

.stat-icon {
  width: 40px;
  height: 40px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.stat-title {
  font-size: 15px;
  font-weight: 600;
  color: #E2E8F0;
  flex: 1;
}

.stat-arrow {
  color: #64748B;
  font-size: 14px;
}

.stat-body {
  padding: 12px 20px 16px;
  min-height: 60px;
}

.stat-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 4px 0;
}

.stat-label {
  font-size: 13px;
  color: #64748B;
}

.stat-value {
  font-size: 14px;
  font-weight: 600;
  color: #E2E8F0;
}

.stat-divider {
  margin: 0 4px;
  color: #64748B;
  font-weight: 400;
}

.stat-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 8px;
}

.stat-tag {
  font-size: 12px;
}

.stat-error {
  display: flex;
  align-items: center;
  gap: 8px;
  color: #64748B;
  font-size: 13px;
}

.text-success { color: #22C55E; }
.text-danger { color: #EF4444; }
.text-warning { color: #F59E0B; }
</style>
