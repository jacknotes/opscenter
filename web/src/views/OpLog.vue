<template>
  <div>
    <el-card>
      <template #header>
        <div style="display: flex; align-items: center; gap: 10px; flex-wrap: wrap;">
          <span style="white-space: nowrap;">模块:</span>
          <el-select v-model="module" style="width: 150px" @change="loadData">
            <el-option label="ALL" value="all" />
            <el-option label="LVS" value="lvs" />
            <el-option label="Nginx" value="nginx" />
            <el-option label="Kubernetes" value="k8s" />
            <el-option label="Kubernetes-PrePro" value="preprod" />
            <el-option label="认证" value="auth" />
            <el-option label="服务器" value="server" />
          </el-select>
          <span style="white-space: nowrap;">服务器:</span>
          <el-select v-model="serverId" style="width: 200px" @change="loadData">
            <el-option label="ALL" :value="0" />
            <el-option v-for="s in servers" :key="s.id" :label="s.name" :value="s.id" />
          </el-select>
          <span style="white-space: nowrap;">操作人:</span>
          <el-input v-model="username" style="width: 120px" placeholder="用户名" clearable @change="loadData" @clear="loadData" />
          <span style="white-space: nowrap;">状态:</span>
          <el-select v-model="status" style="width: 100px" @change="loadData">
            <el-option label="ALL" value="all" />
            <el-option label="成功" value="success" />
            <el-option label="失败" value="failed" />
          </el-select>
          <span style="white-space: nowrap;">时间:</span>
          <el-date-picker v-model="dateRange" type="daterange" range-separator="-" start-placeholder="开始" end-placeholder="结束" value-format="YYYY-MM-DD" style="width: 260px" @change="loadData" />
        </div>
      </template>

      <el-table :data="logs" stripe border>
        <el-table-column prop="created_at" label="时间" width="180" />
        <el-table-column prop="username" label="操作人" width="100" />
        <el-table-column prop="module" label="模块" width="100">
          <template #default="{ row }">
            <el-tag :type="moduleTagType(row.module)">{{ moduleLabel(row.module) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="ip" label="IP" width="130" show-overflow-tooltip />
        <el-table-column prop="server_name" label="服务器" width="150" show-overflow-tooltip />
        <el-table-column prop="action" label="动作" width="120" />
        <el-table-column prop="target" label="目标" min-width="200" show-overflow-tooltip />
        <el-table-column prop="status" label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="row.status === 'success' ? 'success' : 'danger'">{{ row.status }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column type="expand" label="详情" width="80">
          <template #default="{ row }">
            <div style="padding: 10px;">
              <p><strong>命令：</strong></p>
              <pre style="background: #f5f5f5; padding: 10px; border-radius: 4px;">{{ row.detail }}</pre>
              <p style="margin-top: 10px;"><strong>输出：</strong></p>
              <pre style="background: #1e1e1e; color: #d4d4d4; padding: 10px; border-radius: 4px; max-height: 300px; overflow-y: auto;">{{ row.output }}</pre>
            </div>
          </template>
        </el-table-column>
      </el-table>

      <el-pagination
        style="margin-top: 15px; justify-content: flex-end;"
        v-model:current-page="page"
        v-model:page-size="pageSize"
        :total="total"
        :page-sizes="[20, 50, 100]"
        layout="total, sizes, prev, pager, next"
        @size-change="loadData"
        @current-change="loadData"
      />
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { getLogs, getServers } from '../api'
import { ElMessage } from 'element-plus'

const logs = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const module = ref('all')
const serverId = ref(0)
const servers = ref([])
const username = ref('')
const status = ref('all')
const dateRange = ref(null)

onMounted(async () => {
  try {
    servers.value = await getServers()
  } catch (e) {
    console.error('Failed to load servers:', e)
  }
  loadData()
})

const moduleLabels = { lvs: 'LVS', nginx: 'Nginx', k8s: 'Kubernetes', preprod: 'K8s-PrePro', auth: '认证', server: '服务器' }
const moduleTagTypes = { lvs: '', nginx: 'success', k8s: 'warning', preprod: 'warning', auth: 'danger', server: 'info' }

function moduleLabel(m) {
  return moduleLabels[m] || m
}

function moduleTagType(m) {
  return moduleTagTypes[m] || ''
}

async function loadData() {
  try {
    const params = { page: page.value, size: pageSize.value }
    if (module.value && module.value !== 'all') params.module = module.value
    if (serverId.value && serverId.value !== 0) params.server_id = serverId.value
    if (username.value) params.username = username.value
    if (status.value && status.value !== 'all') params.status = status.value
    if (dateRange.value && dateRange.value.length === 2) {
      params.start_time = dateRange.value[0]
      params.end_time = dateRange.value[1]
    }
    const res = await getLogs(params)
    logs.value = res.data || []
    total.value = res.total || 0
  } catch (e) {
    ElMessage.error('加载日志失败')
  }
}
</script>
