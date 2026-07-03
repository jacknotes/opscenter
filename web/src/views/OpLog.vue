<template>
  <div>
    <el-card class="main-card">
      <template #header>
        <div style="display: flex; align-items: center; gap: 10px">
          <span class="filter-label">模块:</span>
          <el-select v-model="module" style="width: 150px" @change="onModuleChange">
            <el-option label="全部" value="all" />
            <el-option label="LVS" value="lvs" />
            <el-option label="Nginx" value="nginx" />
            <el-option label="k8s" value="k8s" />
            <el-option label="k8s-prepro" value="preprod" />
            <el-option v-if="userStore.isAdmin" label="认证" value="auth" />
            <el-option v-if="userStore.isAdmin" label="服务器" value="server" />
          </el-select>
          <span class="filter-label">状态:</span>
          <el-select v-model="status" style="width: 150px" @change="loadData">
            <el-option label="全部" value="all" />
            <el-option label="成功" value="success" />
            <el-option label="失败" value="failed" />
          </el-select>
          <div style="flex-shrink: 0; width: 240px">
            <el-date-picker
              v-model="dateRange"
              type="daterange"
              range-separator="-"
              start-placeholder="开始日期"
              end-placeholder="结束日期"
              value-format="YYYY-MM-DD"
              style="width: 100%"
              :shortcuts="dateShortcuts"
              clearable
              @change="onDateChange"
              @clear="onDateChange"
            />
          </div>
          <el-input
            v-model="keyword"
            style="width: 250px; margin-left: 20px"
            placeholder="搜索操作人/动作/目标/服务器/IP"
            clearable
            @change="onSearch"
            @clear="onSearch"
          />
          <el-button type="info" class="el-button--cyan" @click="handleRefresh">{{
            hasFilters ? '查询' : '刷新'
          }}</el-button>
        </div>
      </template>

      <el-table v-force-reflow v-loading="loading" :data="logs" stripe border max-height="calc(100vh - 200px)" @sort-change="handleSortChange">
        <el-table-column type="expand" width="50">
          <template #default="{ row }">
            <div style="padding: 10px">
              <p><strong>命令：</strong></p>
              <pre class="command-block">{{ row.detail }}</pre>
              <p style="margin-top: 10px"><strong>输出：</strong></p>
              <pre class="output-block">{{ row.output }}</pre>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="100" sortable="custom">
          <template #default="{ row }">
            <el-tag :type="row.status === 'success' ? 'success' : 'danger'">{{ statusLabel(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="时间" width="180" sortable="custom">
          <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column prop="username" label="操作人" width="100" sortable="custom" />
        <el-table-column prop="module" label="模块" width="100" sortable="custom">
          <template #default="{ row }">
            <el-tag :type="moduleTagType(row.module)">{{ moduleLabel(row.module) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="ip" label="IP" width="130" show-overflow-tooltip />
        <el-table-column prop="server_name" label="服务器" width="150" show-overflow-tooltip />
        <el-table-column prop="action" label="动作" width="120" />
        <el-table-column prop="target" label="目标" min-width="200" show-overflow-tooltip />
      </el-table>

      <div class="pagination-wrapper">
        <div></div>
        <el-pagination
          v-model:current-page="page"
          v-model:page-size="pageSize"
          :total="total"
          :page-sizes="[10, 20, 50, 100]"
          layout="total, sizes, prev, pager, next"
          @size-change="loadData"
          @current-change="loadData"
        />
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, shallowRef, computed, onMounted, onActivated } from 'vue'
import { getLogs } from '../api'
import { useUserStore } from '../stores/user'
import { ElMessage } from 'element-plus'
import { DEFAULT_PAGE_SIZE } from '../utils/constants'
import { formatTime } from '../utils/format'

const userStore = useUserStore()

const logs = shallowRef([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(DEFAULT_PAGE_SIZE)
const loading = ref(false)
const module = ref('all')
const keyword = ref('')
const status = ref('all')
const dateRange = ref(null)
const sortBy = ref('created_at')
const sortOrder = ref('desc')
const dateShortcuts = [
  {
    text: '近一周',
    value: () => {
      const e = new Date()
      const s = new Date()
      s.setDate(s.getDate() - 7)
      return [s, e]
    },
  },
  {
    text: '近一个月',
    value: () => {
      const e = new Date()
      const s = new Date()
      s.setMonth(s.getMonth() - 1)
      return [s, e]
    },
  },
  {
    text: '近三个月',
    value: () => {
      const e = new Date()
      const s = new Date()
      s.setMonth(s.getMonth() - 3)
      return [s, e]
    },
  },
]

const hasFilters = computed(
  () => module.value !== 'all' || status.value !== 'all' || !!keyword.value || !!dateRange.value
)

onMounted(() => {
  loadData()
})

onActivated(() => {
  loadData()
})

function onModuleChange() {
  page.value = 1
  loadData()
}

function onSearch() {
  page.value = 1
  loadData()
}

const moduleLabels = { lvs: 'LVS', nginx: 'Nginx', k8s: 'K8S', preprod: '预生产', auth: '认证', server: '服务器' }
const moduleTagTypes = {
  lvs: 'primary',
  nginx: 'success',
  k8s: 'warning',
  preprod: 'warning',
  auth: 'danger',
  server: 'info',
}

function moduleLabel(m) {
  return moduleLabels[m] || m
}

function moduleTagType(m) {
  return moduleTagTypes[m] || ''
}

const statusLabels = { success: '成功', failed: '失败' }
function statusLabel(s) {
  return statusLabels[s] || s
}

function onDateChange() {
  page.value = 1
  loadData()
}

function handleSortChange({ prop, order }) {
  sortBy.value = prop || 'created_at'
  sortOrder.value = order === 'ascending' ? 'asc' : 'desc'
  page.value = 1
  loadData()
}

async function handleRefresh() {
  try {
    await loadData()
    ElMessage.success('刷新成功')
  } catch (e) {
    // loadData 已经处理了错误提示
  }
}

async function loadData() {
  loading.value = true
  try {
    const params = { page: page.value, size: pageSize.value }
    if (module.value && module.value !== 'all') params.module = module.value
    if (keyword.value) params.keyword = keyword.value
    if (status.value && status.value !== 'all') params.status = status.value
    if (dateRange.value && dateRange.value.length === 2) {
      params.start_time = dateRange.value[0]
      params.end_time = dateRange.value[1]
    }
    params.sort_by = sortBy.value
    params.sort_order = sortOrder.value
    const res = await getLogs(params)
    logs.value = res.data || []
    total.value = res.total || 0
  } catch (e) {
    ElMessage.error('加载日志失败')
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.command-block {
  background: var(--bg-elevated);
  color: var(--text-primary);
  padding: 10px;
  border-radius: 6px;
  white-space: pre-wrap;
  word-break: break-all;
  font-family: 'JetBrains Mono', monospace;
  font-size: var(--font-base);
  line-height: 1.6;
}
.command-block::selection,
.command-block *::selection {
  background: rgba(6, 182, 212, 0.5);
  color: #fff;
}
.output-block {
  background: var(--terminal-bg);
  color: var(--terminal-text);
  padding: 10px;
  border-radius: 6px;
  max-height: 300px;
  overflow-y: auto;
  white-space: pre-wrap;
  word-break: break-all;
  font-family: 'JetBrains Mono', monospace;
  font-size: var(--font-base);
  line-height: 1.6;
}
.output-block::selection,
.output-block *::selection {
  background: rgba(34, 211, 238, 0.5);
  color: #fff;
}
</style>
