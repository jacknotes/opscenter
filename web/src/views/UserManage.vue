<script setup lang="ts">
import { ref, reactive, computed, onMounted, nextTick } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { userApi, extractErrorMessage } from '@/api'
import type { User, LdapUser } from '@/api/types'
import { useAuthStore } from '@/stores/auth'
import { useTablePaging } from '@/composables/useTablePaging'
import { useSelection } from '@/composables/useSelection'
import { useBatchOperation } from '@/composables/useBatchOperation'
import { showLoadError, showBatchResult } from '@/utils/message'
import { PAGE_SIZES, DEFAULT_PAGE_SIZE } from '@/utils/constants'
import { i18n } from '@/i18n'

const t = i18n.global.t
const auth = useAuthStore()

/** 后端 users 列表额外返回 online 字段（见 internal/handler/auth.go ListUsers） */
type UserRow = User & { online?: boolean }

interface SortPayload {
  prop?: string | null
  order?: 'ascending' | 'descending' | null
}

const pageSizeOptions: number[] = [...PAGE_SIZES]

// ---------- 列表 ----------
const loading = ref(false)
const rows = ref<UserRow[]>([])
const keyword = ref('')

async function load(): Promise<void> {
  loading.value = true
  try {
    rows.value = await userApi.list()
    restoreSelection()
  } catch (err) {
    showLoadError(err, '加载用户列表失败')
  } finally {
    loading.value = false
  }
}

onMounted(load)

const filtered = computed(() => {
  const kw = keyword.value.trim().toLowerCase()
  if (!kw) return rows.value
  return rows.value.filter((u) => {
    const statusText = u.enabled ? '已启用' : '已禁用'
    const roleText = u.role === 'admin' ? '管理员' : '普通用户'
    const authText = u.auth_source === 'ldap' ? 'ldap' : '本地'
    return (
      u.username.toLowerCase().includes(kw) ||
      u.name.toLowerCase().includes(kw) ||
      u.email.toLowerCase().includes(kw) ||
      u.role.toLowerCase().includes(kw) ||
      roleText.includes(kw) ||
      statusText.includes(kw) ||
      authText.toLowerCase().includes(kw) ||
      (kw === '启用' && u.enabled) ||
      (kw === '禁用' && !u.enabled) ||
      (kw === '本地' && u.auth_source !== 'ldap') ||
      (kw === '锁定' && u.locked) ||
      (kw === '正常' && !u.locked) ||
      (kw === '在线' && u.online === true) ||
      (kw === '离线' && !u.online)
    )
  })
})

// ---------- 排序 + 分页（客户端） ----------
const {
  paged,
  currentPage,
  pageSize,
  total,
  onSortChange: onTableSortChange,
} = useTablePaging(filtered, 20)

// ---------- 跨页选择 ----------
const {
  selectedIds,
  allSelected,
  tableRef,
  handleSelectionChange,
  handleSizeChange,
  handleCurrentChange,
  toggleSelectAll,
  restoreSelection,
} = useSelection<UserRow>('id', paged, { search: keyword, currentPage })

const selectedRows = computed(() => rows.value.filter((u) => selectedIds.value.has(u.id)))
const hasSelection = computed(() => selectedRows.value.length > 0)
/** 批量启停方向：勾选全部为禁用 → 启用；否则 → 禁用（对齐 v1） */
const batchEnable = computed(
  () => selectedRows.value.length > 0 && selectedRows.value.every((u) => !u.enabled),
)

function handleSortChange({ prop, order }: SortPayload): void {
  onTableSortChange({ prop, order })
  nextTick(() => restoreSelection())
}

// ---------- 新增 / 编辑 ----------
const editVisible = ref(false)
const editingId = ref<number | null>(null)
const saving = ref(false)

const form = reactive({
  username: '',
  password: '',
  name: '',
  email: '',
  role: 'user' as 'admin' | 'user',
  enabled: true,
})

async function openCreate(): Promise<void> {
  editingId.value = null
  Object.assign(form, { username: '', password: '', name: '', email: '', role: 'user', enabled: true })
  editVisible.value = true
}

function openEdit(row: UserRow): void {
  editingId.value = row.id
  Object.assign(form, {
    username: row.username,
    password: '',
    name: row.name,
    email: row.email,
    role: row.role,
    enabled: row.enabled,
  })
  editVisible.value = true
}

async function save(): Promise<void> {
  if (!form.username.trim() || !form.name.trim() || !form.email.trim()) {
    ElMessage.warning(t('users.username'))
    return
  }
  saving.value = true
  try {
    if (editingId.value === null) {
      await userApi.create({
        username: form.username.trim(),
        password: form.password,
        name: form.name.trim(),
        email: form.email.trim(),
        role: form.role,
      })
    } else {
      await userApi.update(editingId.value, {
        username: form.username.trim(),
        name: form.name.trim(),
        email: form.email.trim(),
        role: form.role,
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

// ---------- 启停 / 删除 / 批量 ----------
function isProtected(row: Pick<UserRow, 'id' | 'username'>): boolean {
  return row.username === 'admin' || row.id === auth.user?.id
}

/** 生成跳过提示信息，精确描述实际跳过了哪些用户（无需提示时返回 null） */
function buildSkipMessage(original: UserRow[], operable: UserRow[], ...extraReasons: string[]): string | null {
  if (operable.length >= original.length) return null
  const skipped = original.filter((r) => !operable.includes(r))
  const reasons: string[] = []
  if (skipped.some((r) => r.username === 'admin')) reasons.push('admin')
  if (skipped.some((r) => r.id === auth.user?.id)) reasons.push('当前用户')
  reasons.push(...extraReasons)
  return `已自动跳过${reasons.join('、')}，将对剩余 ${operable.length} 个用户执行操作`
}

async function toggle(row: UserRow): Promise<void> {
  if (isProtected(row)) {
    ElMessage.warning(t('users.selfProtect'))
    return
  }
  await userApi.toggle(row.id)
  ElMessage.success(t('common.execSuccess'))
  await load()
}

async function remove(row: UserRow): Promise<void> {
  if (isProtected(row)) {
    ElMessage.warning(t('users.selfProtect'))
    return
  }
  await ElMessageBox.confirm(row.username, t('common.confirmDelete', { count: 1 }), { type: 'warning' })
  await userApi.delete(row.id)
  ElMessage.success(t('common.execSuccess'))
  selectedIds.value.delete(row.id)
  await load()
}

async function batchDelete(): Promise<void> {
  const targets = selectedRows.value.filter((u) => !isProtected(u))
  if (targets.length === 0) {
    ElMessage.warning('选中的用户中没有可删除的用户')
    return
  }
  const skipMsg = buildSkipMessage(selectedRows.value, targets)
  if (skipMsg) ElMessage.info(skipMsg)
  await ElMessageBox.confirm(t('common.confirmDelete', { count: targets.length }), t('common.confirm'), {
    type: 'warning',
  })
  const res = await userApi.batchDelete(targets.map((u) => u.id))
  showBatchResult(res)
  selectedIds.value.clear()
  await load()
}

async function batchToggle(enabled: boolean): Promise<void> {
  const targets = selectedRows.value.filter((u) => !isProtected(u))
  if (targets.length === 0) {
    ElMessage.warning('选中的用户中没有可操作的用户')
    return
  }
  const skipMsg = buildSkipMessage(selectedRows.value, targets)
  if (skipMsg) ElMessage.info(skipMsg)
  const res = await userApi.batchToggle(targets.map((u) => u.id), enabled)
  showBatchResult(res)
  selectedIds.value.clear()
  await load()
}

// ---------- 解锁 / 强制下线 ----------
const batchOp = useBatchOperation()

async function unlock(row: UserRow): Promise<void> {
  if (isProtected(row)) {
    ElMessage.warning(t('users.selfProtect'))
    return
  }
  try {
    await userApi.unlockUser(row.id)
    ElMessage.success('已解锁')
    await load()
  } catch (err) {
    ElMessage.error(extractErrorMessage(err))
  }
}

async function kick(row: UserRow): Promise<void> {
  if (isProtected(row)) {
    ElMessage.warning(t('users.selfProtect'))
    return
  }
  try {
    await userApi.kickUser(row.id)
    ElMessage.success('已强制下线')
    await load()
  } catch (err) {
    const status = (err as { response?: { status?: number } } | null)?.response?.status
    if (status === 400) {
      ElMessage.warning('该用户当前已不在线')
    } else {
      ElMessage.error(extractErrorMessage(err))
    }
  }
}

async function batchUnlock(): Promise<void> {
  const targets = selectedRows.value.filter((u) => u.locked && !isProtected(u))
  if (targets.length === 0) {
    ElMessage.warning('选中的用户中没有可解锁的用户')
    return
  }
  const skipMsg = buildSkipMessage(selectedRows.value, targets)
  if (skipMsg) ElMessage.info(skipMsg)
  const names = targets.map((r) => r.username).join('、')
  await ElMessageBox.confirm(`确定要解锁以下 ${targets.length} 个用户吗？\n${names}`, '批量解锁', { type: 'warning' })
  await batchOp.confirmBatch('批量解锁', async () => {
    try {
      const res = await userApi.batchUnlockUsers(targets.map((u) => u.id))
      showBatchResult(res)
      selectedIds.value.clear()
      await load()
    } catch (err) {
      ElMessage.error(extractErrorMessage(err))
    }
  })
}

async function batchKick(): Promise<void> {
  const targets = selectedRows.value.filter((u) => u.online && !isProtected(u))
  if (targets.length === 0) {
    ElMessage.warning('选中的用户中没有可下线的在线用户')
    return
  }
  const offlineSkipped = selectedRows.value.some((r) => !r.online)
  const skipMsg = buildSkipMessage(selectedRows.value, targets, ...(offlineSkipped ? ['离线用户'] : []))
  if (skipMsg) ElMessage.info(skipMsg)
  const names = targets.map((r) => r.username).join('、')
  await ElMessageBox.confirm(`确定要强制下线以下 ${targets.length} 个用户吗？\n${names}`, '批量下线', {
    type: 'warning',
  })
  await batchOp.confirmBatch('批量下线', async () => {
    try {
      const res = await userApi.batchKickUsers(targets.map((u) => u.id))
      showBatchResult(res)
      selectedIds.value.clear()
      await load()
    } catch (err) {
      ElMessage.error(extractErrorMessage(err))
    }
  })
}

// ---------- 重置密码 ----------
const resetVisible = ref(false)
const resetTarget = ref<UserRow | null>(null)
const newPassword = ref('')
const resetting = ref(false)

function openReset(row: UserRow): void {
  if (row.auth_source === 'ldap') {
    ElMessage.warning('LDAP 用户不支持重置密码')
    return
  }
  resetTarget.value = row
  newPassword.value = ''
  resetVisible.value = true
}

async function doReset(): Promise<void> {
  if (!resetTarget.value || !newPassword.value) return
  resetting.value = true
  try {
    await userApi.resetPassword(resetTarget.value.id, newPassword.value)
    ElMessage.success(t('common.execSuccess'))
    resetVisible.value = false
  } catch (err) {
    ElMessage.error(extractErrorMessage(err))
  } finally {
    resetting.value = false
  }
}

// ---------- LDAP 导入 ----------
interface LdapRow extends LdapUser {
  imported?: boolean
}

const ldapVisible = ref(false)
const ldapLoading = ref(false)
const importing = ref(false)
const ldapRows = ref<LdapRow[]>([])
const ldapSearch = ref('')

const ldapFiltered = computed(() => {
  const kw = ldapSearch.value.trim().toLowerCase()
  if (!kw) return ldapRows.value
  return ldapRows.value.filter(
    (u) =>
      u.username.toLowerCase().includes(kw) ||
      (u.name && u.name.toLowerCase().includes(kw)) ||
      (u.email && u.email.toLowerCase().includes(kw)),
  )
})

const {
  paged: ldapPaged,
  currentPage: ldapCurrentPage,
  pageSize: ldapPageSize,
  total: ldapTotal,
  onSortChange: onLdapSortChange,
} = useTablePaging<LdapRow>(ldapFiltered, DEFAULT_PAGE_SIZE)

const {
  selectedIds: ldapSelectedIds,
  tableRef: ldapTableRef,
  handleSelectionChange: handleLdapSelectionChange,
  handleSizeChange: handleLdapSizeChange,
  handleCurrentChange: handleLdapCurrentChange,
  restoreSelection: restoreLdapSelection,
} = useSelection<LdapRow>('username', ldapPaged, { search: ldapSearch, currentPage: ldapCurrentPage })

const ldapSelectedRows = computed(() => ldapRows.value.filter((u) => ldapSelectedIds.value.has(u.username)))

function handleLdapSortChange({ prop, order }: SortPayload): void {
  onLdapSortChange({ prop, order })
  nextTick(() => restoreLdapSelection())
}

async function openLdap(): Promise<void> {
  ldapVisible.value = true
  ldapLoading.value = true
  ldapSearch.value = ''
  ldapSelectedIds.value.clear()
  try {
    const ldapList = await userApi.listLdap()
    // 标记已导入的用户，防止重复勾选
    const importedUsernames = new Set(rows.value.filter((u) => u.auth_source === 'ldap').map((u) => u.username))
    ldapRows.value = (ldapList || []).map((u) => ({ ...u, imported: importedUsernames.has(u.username) }))
  } catch (err) {
    ElMessage.error(extractErrorMessage(err))
    ldapVisible.value = false
  } finally {
    ldapLoading.value = false
  }
}

function handleLdapDialogClose(): void {
  ldapRows.value = []
  ldapSearch.value = ''
  ldapSelectedIds.value.clear()
}

async function doImport(): Promise<void> {
  const users = ldapSelectedRows.value
  if (users.length === 0) return
  importing.value = true
  try {
    const res = await userApi.importLdap(
      users.map((u) => ({ username: u.username, name: u.name, email: u.email, dn: u.dn })),
    )
    showBatchResult(res)
    ldapVisible.value = false
    await load()
  } catch (err) {
    ElMessage.error(extractErrorMessage(err))
  } finally {
    importing.value = false
  }
}
</script>

<template>
  <div class="page">
    <div class="page-head">
      <div>
        <h1 class="page-title grad-text">{{ t('users.title') }}</h1>
        <p class="page-subtitle">{{ t('users.subtitle') }}</p>
      </div>
      <div class="page-actions">
        <el-button @click="openLdap">{{ t('users.ldapImport') }}</el-button>
        <el-button type="primary" @click="openCreate">{{ t('common.add') }}</el-button>
      </div>
    </div>

    <div class="page-actions" style="margin-bottom: var(--space-4)">
      <el-input
        v-model="keyword"
        :placeholder="t('logs.keyword')"
        clearable
        style="width: 220px"
      />
      <el-button :disabled="!hasSelection" @click="batchToggle(batchEnable)">
        {{ batchEnable ? '批量启用' : '批量禁用' }}
      </el-button>
      <el-button :disabled="!hasSelection" @click="batchUnlock">批量解锁</el-button>
      <el-button type="warning" :disabled="!hasSelection" @click="batchKick">批量强制下线</el-button>
      <el-button type="danger" :disabled="!hasSelection" @click="batchDelete">
        {{ t('common.batchDelete') }}
      </el-button>
    </div>

    <div v-loading="loading" class="card table-card reveal d-1">
      <el-table
        ref="tableRef"
        :data="paged"
        row-key="id"
        @selection-change="handleSelectionChange"
        @sort-change="handleSortChange"
      >
        <el-table-column type="selection" width="42" />
        <el-table-column prop="username" :label="t('users.username')" min-width="130" sortable="custom">
          <template #default="{ row }">
            <span class="mono">{{ row.username }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="name" :label="t('users.name')" min-width="120" sortable="custom" />
        <el-table-column prop="email" :label="t('users.email')" min-width="180" show-overflow-tooltip />
        <el-table-column prop="role" :label="t('users.role')" width="90" sortable="custom">
          <template #default="{ row }">
            <el-tag size="small" :type="row.role === 'admin' ? 'danger' : 'info'" effect="plain">
              {{ row.role }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="auth_source" :label="t('users.authSource')" width="90" />
        <el-table-column prop="enabled" :label="t('common.enabled')" width="90" sortable="custom">
          <template #default="{ row }">
            <el-tag size="small" :type="row.enabled ? 'success' : 'info'" effect="plain">
              {{ row.enabled ? t('common.enabled') : t('common.disabled') }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="locked" label="锁定状态" width="90" sortable="custom">
          <template #default="{ row }">
            <el-tag v-if="row.locked" size="small" type="danger">锁定</el-tag>
            <el-tag v-else size="small" type="info" effect="plain">正常</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="online" label="在线状态" width="90" align="center" sortable="custom">
          <template #default="{ row }">
            <span
              class="online-dot"
              :class="row.online ? 'online' : 'offline'"
              :title="row.online ? t('common.online') : t('common.offline')"
            />
          </template>
        </el-table-column>
        <el-table-column :label="t('common.operation')" width="380" fixed="right">
          <template #default="{ row }">
            <el-button link size="small" @click="openEdit(row as UserRow)">{{ t('common.edit') }}</el-button>
            <el-button link type="warning" size="small" :disabled="row.auth_source === 'ldap'" @click="openReset(row as UserRow)">
              {{ t('users.resetPassword') }}
            </el-button>
            <el-button
              link
              :type="row.enabled ? 'warning' : 'success'"
              size="small"
              :disabled="isProtected(row as Pick<UserRow, 'id' | 'username'>)"
              @click="toggle(row as UserRow)"
            >
              {{ row.enabled ? t('common.disabled') : t('common.enabled') }}
            </el-button>
            <el-button
              v-if="row.locked"
              link
              type="warning"
              size="small"
              :disabled="isProtected(row as Pick<UserRow, 'id' | 'username'>)"
              @click="unlock(row as UserRow)"
            >
              解锁
            </el-button>
            <el-button
              v-if="row.online"
              link
              type="danger"
              size="small"
              :disabled="isProtected(row as Pick<UserRow, 'id' | 'username'>)"
              @click="kick(row as UserRow)"
            >
              强制下线
            </el-button>
            <el-button link type="danger" size="small" :disabled="isProtected(row as Pick<UserRow, 'id' | 'username'>)" @click="remove(row as UserRow)">
              {{ t('common.delete') }}
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination-wrapper">
        <div class="pagination-left">
          <span v-if="hasSelection" class="selection-count">已选 {{ selectedRows.length }} 项</span>
          <el-button v-if="paged.length > 0" link type="primary" size="small" @click="toggleSelectAll()">
            {{ allSelected ? '清空本页' : '全选本页' }}
          </el-button>
        </div>
        <el-pagination
          v-model:current-page="currentPage"
          v-model:page-size="pageSize"
          layout="total, sizes, prev, pager, next"
          :total="total"
          :page-sizes="pageSizeOptions"
          @size-change="handleSizeChange"
          @current-change="handleCurrentChange"
        />
      </div>
    </div>

    <!-- 新增/编辑 -->
    <el-dialog
      v-model="editVisible"
      :title="editingId === null ? t('users.createTitle') : t('users.editTitle')"
      width="520px"
      append-to-body
    >
      <el-form label-position="top">
        <el-form-item :label="t('users.username')" required>
          <el-input v-model="form.username" :disabled="editingId !== null" class="mono" />
        </el-form-item>
        <el-form-item v-if="editingId === null" :label="t('users.password')" required>
          <el-input v-model="form.password" type="password" show-password :placeholder="t('users.passwordStrength')" />
        </el-form-item>
        <el-form-item :label="t('users.name')" required>
          <el-input v-model="form.name" />
        </el-form-item>
        <el-form-item :label="t('users.email')" required>
          <el-input v-model="form.email" type="email" />
        </el-form-item>
        <el-form-item :label="t('users.role')">
          <el-radio-group v-model="form.role" :disabled="editingId !== null && isProtected({ id: editingId, username: form.username })">
            <el-radio-button value="user">user</el-radio-button>
            <el-radio-button value="admin">admin</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="editingId !== null" :label="t('common.enabled')">
          <el-switch v-model="form.enabled" :disabled="isProtected({ id: editingId, username: form.username })" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="saving" @click="save">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- 重置密码 -->
    <el-dialog v-model="resetVisible" :title="t('users.resetPassword')" width="440px" append-to-body>
      <el-input
        v-model="newPassword"
        type="password"
        show-password
        :placeholder="t('users.passwordStrength')"
      />
      <template #footer>
        <el-button @click="resetVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="resetting" @click="doReset">{{ t('common.confirm') }}</el-button>
      </template>
    </el-dialog>

    <!-- LDAP 导入 -->
    <el-dialog v-model="ldapVisible" :title="t('users.ldapImport')" width="720px" append-to-body @close="handleLdapDialogClose">
      <p class="ldap-desc">{{ t('users.ldapImportDesc') }}</p>
      <div class="ldap-toolbar">
        <span class="ldap-hint">选择要导入的用户，已导入的用户不可重复勾选</span>
        <div class="ldap-toolbar-actions">
          <el-input
            v-model="ldapSearch"
            :placeholder="t('logs.keyword')"
            clearable
            style="width: 200px"
          />
          <el-button :loading="ldapLoading" @click="openLdap">{{ t('common.refresh') }}</el-button>
        </div>
      </div>
      <el-table
        ref="ldapTableRef"
        v-loading="ldapLoading"
        :data="ldapPaged"
        row-key="username"
        size="small"
        max-height="340"
        @selection-change="handleLdapSelectionChange"
        @sort-change="handleLdapSortChange"
      >
        <el-table-column type="selection" width="42" :selectable="(row: LdapRow) => !row.imported" />
        <el-table-column prop="username" :label="t('users.username')" min-width="130" sortable="custom" />
        <el-table-column prop="name" :label="t('users.name')" min-width="110" sortable="custom" />
        <el-table-column prop="email" :label="t('users.email')" min-width="180" sortable="custom" show-overflow-tooltip />
        <el-table-column label="导入状态" width="90" align="center">
          <template #default="{ row }">
            <el-tag v-if="row.imported" size="small" type="success" effect="plain">已导入</el-tag>
            <el-tag v-else size="small" type="info" effect="plain">未导入</el-tag>
          </template>
        </el-table-column>
      </el-table>
      <div class="ldap-pagination">
        <span v-if="ldapSelectedRows.length > 0" class="selection-count">已选 {{ ldapSelectedRows.length }} 项</span>
        <el-pagination
          v-model:current-page="ldapCurrentPage"
          v-model:page-size="ldapPageSize"
          layout="total, sizes, prev, pager, next"
          small
          :total="ldapTotal"
          :page-sizes="pageSizeOptions"
          @size-change="handleLdapSizeChange"
          @current-change="handleLdapCurrentChange"
        />
      </div>
      <template #footer>
        <el-button @click="ldapVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="importing" :disabled="ldapSelectedRows.length === 0" @click="doImport">
          {{ t('users.import') }} ({{ ldapSelectedRows.length }})
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.table-card {
  padding: var(--space-3);
}

.online-dot {
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  vertical-align: middle;
}

.online-dot.online {
  background: #67c23a;
  box-shadow: 0 0 4px #67c23a80;
}

.online-dot.offline {
  background: #c0c4cc;
}

.ldap-desc {
  color: var(--text-secondary);
  font-size: var(--text-sm);
  margin-top: 0;
}

.ldap-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: var(--space-2);
  margin-bottom: var(--space-3);
}

.ldap-hint {
  font-size: var(--text-sm);
  color: var(--text-secondary);
}

.ldap-toolbar-actions {
  display: flex;
  gap: var(--space-2);
}

.ldap-pagination {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-top: var(--space-2);
  padding: var(--space-2) 0;
}
</style>
