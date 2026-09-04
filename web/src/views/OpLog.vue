<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { logApi, extractErrorMessage } from '@/api'
import type { OperationLog, LogQuery } from '@/api/types'
import { i18n } from '@/i18n'

const t = i18n.global.t

const loading = ref(false)
const logs = ref<OperationLog[]>([])
const total = ref(0)
const query = reactive({
  page: 1,
  size: 20,
  module: '',
  status: '',
  username: '',
  keyword: '',
  start_time: '',
  end_time: '',
})

const MODULES = ['lvs', 'nginx', 'k8s', 'preprod', 'server', 'auth']

const dateRange = ref<[string, string] | null>(null)

async function load(): Promise<void> {
  loading.value = true
  try {
    const params: LogQuery = {
      page: query.page,
      size: query.size,
    }
    if (query.module) params.module = query.module
    if (query.status) params.status = query.status
    if (query.username) params.username = query.username
    if (query.keyword) params.keyword = query.keyword
    if (dateRange.value) {
      // 契约：start_time/end_time 均为 YYYY-MM-DD
      params.start_time = dateRange.value[0]
      params.end_time = dateRange.value[1]
    }
    const res = await logApi.list(params)
    logs.value = res.data ?? []
    total.value = res.total
  } catch (err) {
    ElMessage.error(extractErrorMessage(err))
  } finally {
    loading.value = false
  }
}

function resetFilters(): void {
  query.module = ''
  query.status = ''
  query.username = ''
  query.keyword = ''
  dateRange.value = null
  query.page = 1
  void load()
}

function search(): void {
  query.page = 1
  void load()
}

function changePage(p: number): void {
  query.page = p
  void load()
}

function changeSize(s: number): void {
  query.size = s
  query.page = 1
  void load()
}

onMounted(load)

// ---------- 详情 / 输出 ----------
const detailVisible = ref(false)
const current = ref<OperationLog | null>(null)

function showDetail(row: OperationLog): void {
  current.value = row
  detailVisible.value = true
}

const moduleType = (m: string): 'primary' | 'warning' | 'success' | 'danger' | 'info' =>
  m === 'k8s' ? 'primary' : m === 'nginx' ? 'warning' : m === 'lvs' ? 'success' : m === 'preprod' ? 'danger' : 'info'

function formatTime(ts: string): string {
  if (!ts) return '-'
  return ts.replace('T', ' ').slice(0, 19)
}
</script>

<template>
  <div class="page">
    <div class="page-head">
      <div>
        <h1 class="page-title grad-text">{{ t('logs.title') }}</h1>
        <p class="page-subtitle">{{ t('logs.subtitle') }}</p>
      </div>
      <div class="page-actions">
        <el-button :loading="loading" @click="load">{{ t('common.refresh') }}</el-button>
      </div>
    </div>

    <div class="filter-bar card">
      <el-select v-model="query.module" :placeholder="t('logs.module')" clearable style="width: 130px">
        <el-option v-for="m in MODULES" :key="m" :value="m" :label="m.toUpperCase()" />
      </el-select>
      <el-select v-model="query.status" :placeholder="t('logs.status')" clearable style="width: 120px">
        <el-option value="success" :label="t('common.success')" />
        <el-option value="failed" :label="t('common.failed')" />
      </el-select>
      <el-input v-model="query.username" :placeholder="t('logs.user')" clearable style="width: 140px" />
      <el-input
        v-model="query.keyword"
        :placeholder="t('logs.keywordPlaceholder')"
        clearable
        style="width: 220px"
        @keyup.enter="search"
      />
      <el-date-picker
        v-model="dateRange"
        type="daterange"
        value-format="YYYY-MM-DD"
        :start-placeholder="t('logs.startTime')"
        :end-placeholder="t('logs.endTime')"
        style="width: 260px"
      />
      <el-button type="primary" @click="search">{{ t('common.search') }}</el-button>
      <el-button @click="resetFilters">{{ t('common.reset') }}</el-button>
    </div>

    <div v-loading="loading" class="card table-card reveal d-1">
      <el-table :data="logs" size="default">
        <el-table-column :label="t('logs.time')" width="170">
          <template #default="{ row }">
            <span class="mono">{{ formatTime(row.created_at) }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="username" :label="t('logs.user')" width="110" />
        <el-table-column :label="t('logs.module')" width="100">
          <template #default="{ row }">
            <el-tag size="small" :type="moduleType(row.module)" effect="plain">{{ row.module }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="action" :label="t('logs.action')" min-width="120" />
        <el-table-column prop="target" :label="t('logs.target')" min-width="180" show-overflow-tooltip />
        <el-table-column prop="server_name" :label="t('logs.serverName')" width="130" show-overflow-tooltip />
        <el-table-column :label="t('logs.status')" width="90">
          <template #default="{ row }">
            <el-tag size="small" :type="row.status === 'success' ? 'success' : 'danger'" effect="plain">
              {{ row.status === 'success' ? t('common.success') : t('common.failed') }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="ip" :label="t('logs.ip')" width="140" show-overflow-tooltip />
        <el-table-column :label="t('common.operation')" width="90" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" size="small" @click="showDetail(row as OperationLog)">
              {{ t('logs.detail') }}
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <div class="pager">
        <el-pagination
          layout="total, sizes, prev, pager, next"
          :total="total"
          :current-page="query.page"
          :page-size="query.size"
          :page-sizes="[20, 50, 100]"
          @current-change="changePage"
          @size-change="changeSize"
        />
      </div>
    </div>

    <!-- 详情弹窗 -->
    <el-dialog v-model="detailVisible" :title="t('logs.detail')" width="760px" append-to-body>
      <template v-if="current">
        <el-descriptions :column="2" border size="small">
          <el-descriptions-item :label="t('logs.time')">
            <span class="mono">{{ formatTime(current.created_at) }}</span>
          </el-descriptions-item>
          <el-descriptions-item :label="t('logs.user')">{{ current.username }}</el-descriptions-item>
          <el-descriptions-item :label="t('logs.module')">{{ current.module }}</el-descriptions-item>
          <el-descriptions-item :label="t('logs.action')">{{ current.action }}</el-descriptions-item>
          <el-descriptions-item :label="t('logs.serverName')">{{ current.server_name || '-' }}</el-descriptions-item>
          <el-descriptions-item :label="t('logs.ip')">{{ current.ip || '-' }}</el-descriptions-item>
          <el-descriptions-item :label="t('logs.status')">
            <el-tag size="small" :type="current.status === 'success' ? 'success' : 'danger'">
              {{ current.status === 'success' ? t('common.success') : t('common.failed') }}
            </el-tag>
          </el-descriptions-item>
          <el-descriptions-item :label="t('logs.projects')">
            {{ current.project_names === '*' ? t('common.all') : current.project_names || '-' }}
          </el-descriptions-item>
          <el-descriptions-item :label="t('logs.target')" :span="2">
            <span class="mono">{{ current.target || '-' }}</span>
          </el-descriptions-item>
        </el-descriptions>
        <div v-if="current.detail" class="section-title">{{ t('logs.detail') }}</div>
        <pre v-if="current.detail" class="out-pre mono">{{ current.detail }}</pre>
        <div class="section-title">{{ t('logs.output') }}</div>
        <pre class="out-pre mono tall">{{ current.output || t('common.none') }}</pre>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.filter-bar {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-3);
  padding: var(--space-4);
  margin-bottom: var(--space-4);
  align-items: center;
}

.table-card {
  padding: var(--space-3);
}

.pager {
  display: flex;
  justify-content: flex-end;
  padding: var(--space-3) var(--space-2) 0;
}

.out-pre {
  background: var(--bg-deep);
  border: 1px solid var(--border-faint);
  border-radius: var(--radius-sm);
  padding: var(--space-3);
  margin: 0 0 var(--space-3);
  max-height: 260px;
  overflow: auto;
  font-size: var(--text-xs);
  line-height: 1.7;
  white-space: pre-wrap;
  word-break: break-all;
}

.out-pre.tall {
  max-height: 420px;
}

.section-title {
  color: var(--text-secondary);
  font-size: var(--text-sm);
  margin: var(--space-3) 0 var(--space-2);
}
</style>
