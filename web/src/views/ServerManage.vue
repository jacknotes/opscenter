<script setup lang="ts">
import { ref, reactive, onMounted, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { serverApi, extractErrorMessage } from '@/api'
import type { ServerEdit, ServerResponse, TestResult, BatchResult } from '@/api/types'
import { useTablePaging } from '@/composables/useTablePaging'
import { i18n } from '@/i18n'

const t = i18n.global.t

// ---------- 列表 ----------
const loading = ref(false)
const rows = ref<ServerResponse[]>([])
const keyword = ref('')
const typeFilter = ref('')

const SERVER_TYPES = ['lvs', 'nginx', 'kubernetes', 'preprod']

async function load(): Promise<void> {
  loading.value = true
  try {
    rows.value = await serverApi.list({ all: true })
  } catch (err) {
    ElMessage.error(extractErrorMessage(err))
  } finally {
    loading.value = false
  }
}

onMounted(load)

const filtered = computed(() => {
  const kw = keyword.value.trim().toLowerCase()
  return rows.value.filter((s) => {
    if (typeFilter.value && s.server_type !== typeFilter.value) return false
    if (!kw) return true
    return (
      s.name.toLowerCase().includes(kw) ||
      s.host.toLowerCase().includes(kw) ||
      s.username.toLowerCase().includes(kw) ||
      s.env.toLowerCase().includes(kw)
    )
  })
})

const { paged, currentPage, pageSize, total, onSortChange } = useTablePaging(filtered, 20)

const typeTag = (ty: string): 'success' | 'warning' | 'primary' | 'danger' | 'info' =>
  ty === 'lvs' ? 'success' : ty === 'nginx' ? 'warning' : ty === 'kubernetes' ? 'primary' : ty === 'preprod' ? 'danger' : 'info'

const selected = ref<ServerResponse[]>([])

// ---------- 新增 / 编辑 ----------
const editVisible = ref(false)
const editingId = ref<number | null>(null)
const saving = ref(false)

const form = reactive({
  name: '',
  host: '',
  port: 22,
  username: '',
  auth_type: 'password' as 'password' | 'key',
  password: '',
  private_key: '',
  server_type: 'lvs',
  env: '',
  script_path: '',
  script_password: '',
  config_path: '',
  config_pattern: '',
  backup_path: '',
  description: '',
  enabled: true,
  has_password: false,
  has_private_key: false,
  has_script_password: false,
})

function resetForm(): void {
  Object.assign(form, {
    name: '',
    host: '',
    port: 22,
    username: '',
    auth_type: 'password',
    password: '',
    private_key: '',
    server_type: 'lvs',
    env: '',
    script_path: '',
    script_password: '',
    config_path: '',
    config_pattern: '',
    backup_path: '',
    description: '',
    enabled: true,
    has_password: false,
    has_private_key: false,
    has_script_password: false,
  })
}

async function openCreate(): Promise<void> {
  editingId.value = null
  resetForm()
  editVisible.value = true
}

async function openEdit(row: ServerResponse): Promise<void> {
  editingId.value = row.id
  try {
    const detail: ServerEdit = await serverApi.getForEdit(row.id)
    Object.assign(form, {
      name: detail.name,
      host: detail.host,
      port: detail.port,
      username: detail.username,
      auth_type: detail.auth_type,
      password: '',
      private_key: '',
      server_type: detail.server_type,
      env: detail.env,
      script_path: detail.script_path,
      script_password: '',
      config_path: detail.config_path,
      config_pattern: detail.config_pattern,
      backup_path: detail.backup_path,
      description: detail.description,
      enabled: detail.enabled,
      has_password: detail.has_password,
      has_private_key: detail.has_private_key,
      has_script_password: detail.has_script_password,
    })
    editVisible.value = true
  } catch (err) {
    ElMessage.error(extractErrorMessage(err))
  }
}

async function save(): Promise<void> {
  if (!form.name.trim() || !form.host.trim() || !form.username.trim()) {
    ElMessage.warning(t('common.save'))
    return
  }
  saving.value = true
  try {
    if (editingId.value === null) {
      await serverApi.create({
        name: form.name.trim(),
        host: form.host.trim(),
        port: form.port,
        username: form.username.trim(),
        auth_type: form.auth_type,
        password: form.password,
        private_key: form.private_key,
        script_password: form.script_password,
        server_type: form.server_type,
        env: form.env,
        script_path: form.script_path,
        config_path: form.config_path,
        config_pattern: form.config_pattern,
        backup_path: form.backup_path,
        description: form.description,
        enabled: form.enabled,
      })
    } else {
      // 契约：Update 时敏感字段传 "__keep__" 表示保留原值
      await serverApi.update(editingId.value, {
        name: form.name.trim(),
        host: form.host.trim(),
        port: form.port,
        username: form.username.trim(),
        auth_type: form.auth_type,
        password: form.password || '__keep__',
        private_key: form.private_key || '__keep__',
        script_password: form.script_password || '__keep__',
        server_type: form.server_type,
        env: form.env,
        script_path: form.script_path,
        config_path: form.config_path,
        config_pattern: form.config_pattern,
        backup_path: form.backup_path,
        description: form.description,
        enabled: form.enabled,
      })
    }
    ElMessage.success(t('common.execSuccess'))
    editVisible.value = false
    await load()
  } catch (err) {
    ElMessage.error(extractErrorMessage(err))
  } finally {
    saving.value = false
  }
}

// ---------- 测试 / 启停 / 删除 / 批量 ----------
async function testConn(row: ServerResponse): Promise<void> {
  const res: TestResult = await serverApi.test(row.id).catch((err) => {
    ElMessage.error(extractErrorMessage(err))
    return { success: false } as TestResult
  })
  if (res.success) {
    ElMessage.success(`${t('servers.testSuccess')} · ${res.message ?? ''}`)
  } else {
    ElMessage.error(`${t('servers.testFailed')} · ${res.error ?? ''}`)
  }
}

async function toggle(row: ServerResponse): Promise<void> {
  await serverApi.toggle(row.id)
  ElMessage.success(t('common.execSuccess'))
  await load()
}

async function remove(row: ServerResponse): Promise<void> {
  await ElMessageBox.confirm(t('servers.deleteConfirm', { name: row.name }), t('common.confirm'), {
    type: 'warning',
  })
  await serverApi.delete(row.id)
  ElMessage.success(t('common.execSuccess'))
  await load()
}

function showBatchResult(res: BatchResult): void {
  ElMessage({ type: 'success', message: res.message, duration: 5000 })
}

async function batchDelete(): Promise<void> {
  const ids = selected.value.map((s) => s.id)
  if (ids.length === 0) return
  await ElMessageBox.confirm(t('common.confirmDelete', { count: ids.length }), t('common.confirm'), {
    type: 'warning',
  })
  const res = await serverApi.batchDelete(ids)
  showBatchResult(res)
  await load()
}

async function batchToggle(enabled: boolean): Promise<void> {
  const ids = selected.value.map((s) => s.id)
  if (ids.length === 0) return
  const res = await serverApi.batchToggle(ids, enabled)
  showBatchResult(res)
  await load()
}

async function batchTest(): Promise<void> {
  const ids = selected.value.map((s) => s.id)
  if (ids.length === 0) return
  const res = await serverApi.batchTest(ids)
  showBatchResult(res)
}
</script>

<template>
  <div class="page">
    <div class="page-head">
      <div>
        <h1 class="page-title grad-text">{{ t('servers.title') }}</h1>
        <p class="page-subtitle">{{ t('servers.subtitle') }}</p>
      </div>
      <div class="page-actions">
        <el-button type="primary" @click="openCreate">{{ t('common.add') }}</el-button>
        <el-button @click="load">{{ t('common.refresh') }}</el-button>
      </div>
    </div>

    <div class="page-actions" style="margin-bottom: var(--space-4)">
      <el-select v-model="typeFilter" :placeholder="t('servers.serverType')" clearable style="width: 150px">
        <el-option v-for="ty in SERVER_TYPES" :key="ty" :value="ty" :label="ty" />
      </el-select>
      <el-input
        v-model="keyword"
        :placeholder="t('logs.keyword')"
        clearable
        style="width: 220px"
        @keyup.enter="load"
      />
      <el-button :disabled="selected.length === 0" @click="batchTest">{{ t('common.batchTest') }}</el-button>
      <el-button :disabled="selected.length === 0" @click="batchToggle(true)">
        {{ t('common.batchToggle') }}
      </el-button>
      <el-button type="danger" :disabled="selected.length === 0" @click="batchDelete">
        {{ t('common.batchDelete') }}
      </el-button>
    </div>

    <div v-loading="loading" class="card table-card reveal d-1">
      <el-table
        :data="paged"
        row-key="id"
        @sort-change="onSortChange"
        @selection-change="(r: ServerResponse[]) => (selected = r)"
      >
        <el-table-column type="selection" width="42" />
        <el-table-column prop="name" :label="t('servers.name')" min-width="150" sortable>
          <template #default="{ row }">
            <span class="srv-name">{{ row.name }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="t('servers.host')" min-width="160">
          <template #default="{ row }">
            <span class="mono">{{ row.host }}:{{ row.port }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="username" :label="t('servers.username')" width="110" />
        <el-table-column :label="t('servers.authType')" width="100">
          <template #default="{ row }">
            {{ row.auth_type === 'password' ? t('servers.authPassword') : t('servers.authKey') }}
          </template>
        </el-table-column>
        <el-table-column :label="t('servers.serverType')" width="110">
          <template #default="{ row }">
            <el-tag size="small" :type="typeTag(row.server_type)" effect="plain">{{ row.server_type }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="env" :label="t('servers.env')" width="80" />
        <el-table-column :label="t('common.enabled')" width="90">
          <template #default="{ row }">
            <el-tag size="small" :type="row.enabled ? 'success' : 'info'" effect="plain">
              {{ row.enabled ? t('common.enabled') : t('common.disabled') }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="description" :label="t('servers.description')" min-width="140" show-overflow-tooltip />
        <el-table-column :label="t('common.operation')" width="300" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" size="small" @click="testConn(row as ServerResponse)">
              {{ t('servers.test') }}
            </el-button>
            <el-button link size="small" @click="openEdit(row as ServerResponse)">{{ t('common.edit') }}</el-button>
            <el-button link :type="row.enabled ? 'warning' : 'success'" size="small" @click="toggle(row as ServerResponse)">
              {{ row.enabled ? t('common.disabled') : t('common.enabled') }}
            </el-button>
            <el-button link type="danger" size="small" @click="remove(row as ServerResponse)">{{ t('common.delete') }}</el-button>
          </template>
        </el-table-column>
      </el-table>

      <div class="pager">
        <el-pagination
          layout="total, sizes, prev, pager, next"
          :total="total"
          :current-page="currentPage"
          :page-size="pageSize"
          :page-sizes="[20, 50, 100]"
          @current-change="(p: number) => (currentPage = p)"
          @size-change="(s: number) => (pageSize = s)"
        />
      </div>
    </div>

    <!-- 新增/编辑弹窗 -->
    <el-dialog
      v-model="editVisible"
      :title="editingId === null ? t('servers.createTitle') : t('servers.editTitle')"
      width="680px"
      append-to-body
    >
      <el-form label-position="top">
        <div class="form-grid">
          <el-form-item :label="t('servers.name')" required>
            <el-input v-model="form.name" />
          </el-form-item>
          <el-form-item :label="t('servers.env')">
            <el-input v-model="form.env" placeholder="prod / pre" />
          </el-form-item>
          <el-form-item :label="t('servers.host')" required>
            <el-input v-model="form.host" class="mono" />
          </el-form-item>
          <el-form-item :label="t('servers.port')">
            <el-input-number v-model="form.port" :min="1" :max="65535" />
          </el-form-item>
          <el-form-item :label="t('servers.username')" required>
            <el-input v-model="form.username" />
          </el-form-item>
          <el-form-item :label="t('servers.serverType')" required>
            <el-select v-model="form.server_type">
              <el-option v-for="ty in SERVER_TYPES" :key="ty" :value="ty" :label="ty" />
            </el-select>
          </el-form-item>
          <el-form-item :label="t('servers.authType')">
            <el-radio-group v-model="form.auth_type">
              <el-radio-button value="password">{{ t('servers.authPassword') }}</el-radio-button>
              <el-radio-button value="key">{{ t('servers.authKey') }}</el-radio-button>
            </el-radio-group>
          </el-form-item>
          <el-form-item :label="t('common.enabled')">
            <el-switch v-model="form.enabled" />
          </el-form-item>
        </div>

        <el-form-item v-if="form.auth_type === 'password'" :label="t('servers.authPassword')">
          <el-input v-model="form.password" type="password" show-password :placeholder="form.has_password ? t('servers.keep') : ''" />
        </el-form-item>
        <el-form-item v-else :label="t('servers.authKey')">
          <el-input v-model="form.private_key" type="textarea" :rows="4" :placeholder="form.has_private_key ? t('servers.keep') : ''" />
        </el-form-item>
        <el-form-item :label="t('servers.scriptPassword')">
          <el-input v-model="form.script_password" type="password" show-password :placeholder="form.has_script_password ? t('servers.keep') : ''" />
        </el-form-item>
        <el-form-item :label="t('servers.scriptPath')">
          <el-input v-model="form.script_path" class="mono" placeholder="/opt/scripts/lvs.sh" />
        </el-form-item>
        <div v-if="form.server_type === 'nginx'" class="form-grid">
          <el-form-item :label="t('servers.configPath')">
            <el-input v-model="form.config_path" class="mono" placeholder="/etc/nginx/conf.d" />
          </el-form-item>
          <el-form-item :label="t('servers.configPattern')">
            <el-input v-model="form.config_pattern" class="mono" placeholder="*.conf,!*.bak.conf" />
          </el-form-item>
          <el-form-item :label="t('servers.backupPath')">
            <el-input v-model="form.backup_path" class="mono" placeholder="/etc/nginx/backup" />
          </el-form-item>
        </div>
        <el-form-item :label="t('servers.description')">
          <el-input v-model="form.description" type="textarea" :rows="2" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="saving" @click="save">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.table-card {
  padding: var(--space-3);
}

.pager {
  display: flex;
  justify-content: flex-end;
  padding: var(--space-3) var(--space-2) 0;
}

.srv-name {
  font-weight: 600;
}

.form-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  column-gap: var(--space-4);
}

@media (max-width: 640px) {
  .form-grid {
    grid-template-columns: 1fr;
  }
}
</style>
