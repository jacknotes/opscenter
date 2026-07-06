<template>
  <div>
    <el-card class="main-card">
      <template #header>
        <div class="toolbar">
          <el-button type="primary" @click="handleAdd">添加用户</el-button>
          <el-button v-if="ldapEnabled" type="success" @click="showLdapImport">导入 LDAP 用户</el-button>
          <el-dropdown @command="handleMoreCommand">
            <el-button type="info" class="el-button--cyan">
              更多操作<el-icon class="el-icon--right"><ArrowDown /></el-icon>
            </el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="edit" :disabled="selectedRows.length !== 1">编辑</el-dropdown-item>
                <el-dropdown-item command="toggle" :disabled="selectedRows.length === 0">{{ batchToggleLabel }}</el-dropdown-item>
                <el-dropdown-item
                  command="resetPwd"
                  divided
                  :disabled="selectedRows.length !== 1 || selectedRow?.username === 'admin' || selectedRow?.auth_source === 'ldap'"
                >重置密码</el-dropdown-item>
                <el-dropdown-item command="batchUnlock" :disabled="selectedRows.length === 0">批量解锁</el-dropdown-item>
                <el-dropdown-item command="batchKick" :disabled="selectedRows.length === 0">批量下线</el-dropdown-item>
                <el-dropdown-item command="delete" divided :disabled="selectedRows.length === 0">删除</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
          <el-button type="info" class="el-button--cyan" :loading="loading" @click="handleRefresh">刷新</el-button>
          <el-input
            v-model="searchQuery"
            placeholder="搜索用户信息"
            clearable
            class="toolbar-search-input"
          />
        </div>
      </template>

      <el-table
        ref="tableRef"
        v-force-reflow
        :data="paginatedUsers"
        :row-key="(row) => row.id"
        stripe
        border
        :row-class-name="({ row }) => (row.enabled === false ? 'disabled-row' : '')"
        max-height="calc(100vh - 280px)"
        @selection-change="handleSelectionChange"
        @sort-change="handleSortChange"
      >
        <el-table-column type="selection" width="55" />
        <el-table-column label="状态" width="80" align="center" sortable="custom" column-key="enabled">
          <template #default="{ row }">
            <el-tag :type="row.enabled ? 'success' : 'info'" size="small">{{
              row.enabled ? '已启用' : '已禁用'
            }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="username" label="用户名" min-width="130" sortable="custom">
          <template #default="{ row }">
            <span>{{ row.username }}</span>
            <span class="online-dot" :class="row.online ? 'online' : 'offline'" :title="row.online ? '在线' : '离线'" />
            <el-tag v-if="row.locked" type="danger" size="small" class="lock-tag">锁</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="name" label="姓名" min-width="100" sortable="custom" />
        <el-table-column prop="email" label="邮箱" min-width="180" sortable="custom" />
        <el-table-column label="角色" width="100" sortable="custom" column-key="role">
          <template #default="{ row }">
            <el-tag :type="row.role === 'admin' ? 'danger' : 'info'">{{
              row.role === 'admin' ? '管理员' : '普通用户'
            }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="认证来源" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="row.auth_source === 'ldap' ? 'warning' : 'info'" size="small">{{
              row.auth_source === 'ldap' ? 'LDAP' : '本地'
            }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" min-width="160" sortable="custom">
          <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
        </el-table-column>
      </el-table>

      <div v-if="!loading && paginatedUsers.length === 0" class="empty-state">
        <el-icon class="empty-state-icon"><UserFilled /></el-icon>
        <span class="empty-state-text">{{ searchQuery ? '没有匹配的用户' : '暂无用户数据' }}</span>
      </div>

      <div class="pagination-wrapper">
        <div class="pagination-left">
          <span v-if="selectedRows.length > 0" class="selection-count">已选 {{ selectedRows.length }} 项</span>
        </div>
        <el-pagination
          v-model:current-page="currentPage"
          v-model:page-size="pageSize"
          :page-sizes="[10, 20, 50, 100]"
          :total="filteredUsers.length"
          layout="total, sizes, prev, pager, next, jumper"
          @size-change="handleSizeChange"
          @current-change="handleCurrentChange"
        />
      </div>
    </el-card>

    <!-- Add/Edit Dialog -->
    <el-dialog v-model="dialogVisible" :title="isEdit ? '编辑用户' : '添加用户'" width="min(500px, 90vw)" align-center>
      <el-form :model="form" label-width="80px">
        <el-form-item label="用户名" required>
          <el-input v-model="form.username" :disabled="isEdit" />
        </el-form-item>
        <el-form-item label="姓名" required>
          <el-input v-model="form.name" />
        </el-form-item>
        <el-form-item label="邮箱" required>
          <el-input v-model="form.email" />
        </el-form-item>
        <el-form-item v-if="!isEdit" label="密码" required>
          <el-input v-model="form.password" type="password" show-password />
        </el-form-item>
        <el-form-item label="角色" required>
          <el-radio-group v-model="form.role">
            <el-radio value="admin">管理员</el-radio>
            <el-radio value="user">普通用户</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="isEdit" label="状态">
          <el-switch v-model="form.enabled" active-text="启用" inactive-text="禁用" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleSubmit">确定</el-button>
      </template>
    </el-dialog>

    <!-- Reset Password Dialog -->
    <el-dialog v-model="resetPwdVisible" title="重置密码" width="min(400px, 90vw)" align-center>
      <el-form :model="resetPwdForm" label-width="80px">
        <el-form-item label="用户名">
          <el-input :model-value="resetPwdForm.username" disabled />
        </el-form-item>
        <el-form-item label="新密码" required>
          <el-input v-model="resetPwdForm.password" type="password" show-password placeholder="请输入新密码" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="resetPwdVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleSubmitResetPwd">确定</el-button>
      </template>
    </el-dialog>

    <!-- LDAP Import Dialog -->
    <el-dialog v-model="ldapImportVisible" title="导入 LDAP 用户" width="min(800px, 95vw)" align-center @close="handleLdapDialogClose">
      <div v-if="ldapLoading" style="text-align: center; padding: 40px">
        <el-icon class="is-loading" :size="32"><Loading /></el-icon>
        <p style="margin-top: 12px; color: var(--text-regular)">正在从 LDAP 获取用户列表...</p>
      </div>
      <div v-else>
        <div style="margin-bottom: 12px; display: flex; justify-content: space-between; align-items: center">
          <span style="font-size: 14px; color: var(--text-regular)">选择要导入的用户，已导入的用户将自动跳过</span>
          <div style="display: flex; gap: 8px">
            <el-input v-model="ldapSearch" placeholder="搜索用户" clearable style="width: 200px" />
            <el-button type="info" class="el-button--cyan" :loading="ldapLoading" @click="showLdapImport"
              >刷新</el-button
            >
          </div>
        </div>
        <el-table
          ref="ldapTableRef"
          :data="paginatedLdapUsers"
          stripe
          border
          max-height="400"
          @selection-change="handleLdapSelectionChange"
          @sort-change="handleLdapSortChange"
        >
          <el-table-column type="selection" width="55" :selectable="(row) => !row.imported" />
          <el-table-column prop="username" label="用户名" width="120" sortable="custom" />
          <el-table-column prop="name" label="姓名" min-width="120" sortable="custom" />
          <el-table-column prop="email" label="邮箱" min-width="180" sortable="custom" />
          <el-table-column label="状态" width="100" align="center">
            <template #default="{ row }">
              <el-tag v-if="row.imported" type="success" size="small">已导入</el-tag>
              <el-tag v-else type="info" size="small">未导入</el-tag>
            </template>
          </el-table-column>
        </el-table>
        <div class="ldap-pagination">
          <span v-if="selectedLdapUsers.length > 0" class="selection-count">已选 {{ selectedLdapUsers.length }} 项</span>
          <el-pagination
            v-model:current-page="ldapCurrentPage"
            v-model:page-size="ldapPageSize"
            :page-sizes="[10, 20, 50, 100]"
            :total="filteredLdapUsers.length"
            layout="total, sizes, prev, pager, next"
            small
            @size-change="handleLdapSizeChange"
            @current-change="handleLdapCurrentChange"
          />
        </div>
      </div>
      <template #footer>
        <el-button @click="ldapImportVisible = false">取消</el-button>
        <el-button
          type="primary"
          :loading="importing"
          :disabled="selectedLdapUsers.length === 0"
          @click="handleImportLdapUsers"
        >
          导入 ({{ selectedLdapUsers.length }})
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted, onActivated } from 'vue'
import {
  getUsers,
  createUser,
  updateUser,
  batchDeleteUsers,
  batchToggleUsers,
  resetPassword,
  toggleUserEnabled,
  getLdapUsers,
  importLdapUsers,
  batchUnlockUsers,
  batchKickUsers,
} from '../api'
import { showBatchResult } from '../utils/message'
import { formatTime } from '../utils/format'
import { useUserStore } from '../stores/user'
import { useSelection } from '../composables/useSelection'
import { DEFAULT_PAGE_SIZE } from '../utils/constants'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Loading, UserFilled, ArrowDown } from '@element-plus/icons-vue'

const userStore = useUserStore()
const currentUserId = computed(() => userStore.userInfo?.id)

const users = ref([])
const searchQuery = ref('')
const currentPage = ref(1)
const pageSize = ref(DEFAULT_PAGE_SIZE)
const sortProp = ref('')
const sortOrder = ref('')

const filteredUsers = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  let list = users.value
  if (q) {
    list = list.filter((u) => {
      // 状态映射
      const statusText = u.enabled ? '已启用' : '已禁用'
      // 角色映射
      const roleText = u.role === 'admin' ? '管理员' : '普通用户'
      // 认证来源映射
      const authText = u.auth_source === 'ldap' ? 'ldap' : '本地'

      return (
        u.username.toLowerCase().includes(q) ||
        u.name.toLowerCase().includes(q) ||
        u.email.toLowerCase().includes(q) ||
        u.role.toLowerCase().includes(q) ||
        roleText.includes(q) ||
        statusText.includes(q) ||
        authText.toLowerCase().includes(q) ||
        (q === '启用' && u.enabled) ||
        (q === '禁用' && !u.enabled) ||
        (q === '本地' && u.auth_source !== 'ldap') ||
        (q === '锁定' && u.locked) ||
        (q === '正常' && !u.locked) ||
        (q === '在线' && u.online)
      )
    })
  }
  if (sortProp.value && sortOrder.value) {
    const prop = sortProp.value
    const order = sortOrder.value === 'ascending' ? 1 : -1
    list = [...list].sort((a, b) => {
      let va, vb
      if (prop === 'enabled') {
        va = a.enabled ? 1 : 0
        vb = b.enabled ? 1 : 0
      } else if (prop === 'locked') {
        va = a.locked ? 1 : 0
        vb = b.locked ? 1 : 0
      } else if (prop === 'created_at') {
        va = a.created_at || ''
        vb = b.created_at || ''
      } else if (prop === 'role') {
        va = a.role || ''
        vb = b.role || ''
      } else {
        va = (a[prop] || '').toLowerCase()
        vb = (b[prop] || '').toLowerCase()
      }
      if (va < vb) return -1 * order
      if (va > vb) return 1 * order
      return 0
    })
  }
  return list
})

const paginatedUsers = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  return filteredUsers.value.slice(start, start + pageSize.value)
})

const batchToggleLabel = computed(() => {
  if (selectedRows.value.length === 0) return '启用'
  const allDisabled = selectedRows.value.every((r) => !r.enabled)
  return allDisabled ? '启用' : '禁用'
})

const dialogVisible = ref(false)
const isEdit = ref(false)
const editId = ref(null)
const submitting = ref(false)
const loading = ref(false)
const form = ref({ username: '', password: '', name: '', email: '', role: 'user', enabled: true })
// 跨页选择管理
const {
  selectedIds,
  tableRef,
  handleSelectionChange,
  handleSizeChange,
  handleCurrentChange,
} = useSelection('id', paginatedUsers, { search: searchQuery, currentPage })

const selectedRows = computed(() => users.value.filter((u) => selectedIds.value.has(u.id)))
const selectedRow = computed(() => (selectedRows.value.length > 0 ? selectedRows.value[selectedRows.value.length - 1] : null))

const resetPwdVisible = ref(false)
const resetPwdForm = ref({ id: null, username: '', password: '' })

// LDAP 导入相关
const ldapEnabled = ref(false)
const ldapImportVisible = ref(false)
const ldapLoading = ref(false)
const importing = ref(false)
const ldapUsers = ref([])
const ldapSearch = ref('')
const ldapTableRef = ref(null)
const ldapCurrentPage = ref(1)
const ldapPageSize = ref(DEFAULT_PAGE_SIZE)
const selectedLdapUsernames = ref(new Set())
const ldapSortProp = ref('')
const ldapSortOrder = ref('')

const filteredLdapUsers = computed(() => {
  const q = ldapSearch.value.trim().toLowerCase()
  let list = ldapUsers.value
  if (q) {
    list = list.filter(
      (u) =>
        u.username.toLowerCase().includes(q) ||
        (u.name && u.name.toLowerCase().includes(q)) ||
        (u.email && u.email.toLowerCase().includes(q))
    )
  }
  if (ldapSortProp.value && ldapSortOrder.value) {
    const prop = ldapSortProp.value
    const order = ldapSortOrder.value === 'ascending' ? 1 : -1
    list = [...list].sort((a, b) => {
      const va = (a[prop] || '').toLowerCase()
      const vb = (b[prop] || '').toLowerCase()
      if (va < vb) return -1 * order
      if (va > vb) return 1 * order
      return 0
    })
  }
  return list
})

const selectedLdapUsers = computed(() => {
  return ldapUsers.value.filter((u) => selectedLdapUsernames.value.has(u.username))
})

const paginatedLdapUsers = computed(() => {
  const start = (ldapCurrentPage.value - 1) * ldapPageSize.value
  return filteredLdapUsers.value.slice(start, start + ldapPageSize.value)
})

function handleLdapSizeChange() {
  ldapCurrentPage.value = 1
}

function handleLdapCurrentChange() {
  // 翻页不清除选择，由 watch(paginatedLdapUsers) 自动恢复选中态
}

function handleLdapSortChange({ prop, order }) {
  ldapSortProp.value = prop || ''
  ldapSortOrder.value = order || ''
}

// 翻页或数据变化后，恢复当前页中已选用户的勾选状态
watch(
  paginatedLdapUsers,
  () => {
    if (!ldapTableRef.value) return
    const names = selectedLdapUsernames.value
    paginatedLdapUsers.value.forEach((row) => {
      ldapTableRef.value.toggleRowSelection(row, names.has(row.username))
    })
  },
  { flush: 'post' }
)

function handleSortChange({ prop, order, column }) {
  sortProp.value = prop || column?.columnKey || ''
  sortOrder.value = order || ''
}

onMounted(() => {
  loadData()
  checkLdapEnabled()
})

onActivated(() => {
  loadData()
})

async function loadData(showMessage = false) {
  loading.value = true
  try {
    users.value = await getUsers()
    if (showMessage) {
      ElMessage.success('刷新成功')
    }
  } catch (e) {
    ElMessage.error('加载用户列表失败')
  } finally {
    loading.value = false
  }
}

function handleRefresh() {
  loadData(true)
}

async function checkLdapEnabled() {
  try {
    await getLdapUsers()
    ldapEnabled.value = true
  } catch (e) {
    ldapEnabled.value = false
  }
}

function handleAdd() {
  isEdit.value = false
  editId.value = null
  form.value = { username: '', password: '', name: '', email: '', role: 'user', enabled: true }
  dialogVisible.value = true
}

function handleEditSelected() {
  if (!selectedRow.value) return
  handleEdit(selectedRow.value)
}

function handleEdit(row) {
  isEdit.value = true
  editId.value = row.id
  form.value = {
    username: row.username,
    name: row.name,
    email: row.email,
    password: '',
    role: row.role,
    enabled: row.enabled,
  }
  dialogVisible.value = true
}

function handleToggleSelected() {
  if (!selectedRow.value) return
  handleToggle(selectedRow.value)
}

async function handleToggle(row) {
  try {
    const action = row.enabled ? '禁用' : '启用'
    await ElMessageBox.confirm(`确定要${action}用户 "${row.username}" 吗？`, '确认操作')
    const res = await toggleUserEnabled(row.id)
    row.enabled = res.enabled
    selectedRow.value = { ...row }
    ElMessage.success(`已${action}`)
  } catch (e) {
    if (e !== 'cancel') {
      ElMessage.error(e.response?.data?.error || '操作失败')
    }
  }
}

/**
 * 生成跳过提示信息，精确描述实际跳过了哪些用户
 * @param {Array} original - 原始选中的用户列表
 * @param {Array} filtered - 过滤后的用户列表
 * @param {string} extraReasons - 额外的跳过原因（如"离线用户"）
 * @returns {string|null} 提示信息，无需提示时返回 null
 */
function buildSkipMessage(original, filtered, ...extraReasons) {
  if (filtered.length >= original.length) return null
  const skipped = original.filter((r) => !filtered.includes(r))
  const reasons = []
  if (skipped.some((r) => r.username === 'admin')) reasons.push('admin')
  if (skipped.some((r) => r.id === currentUserId.value)) reasons.push('当前用户')
  reasons.push(...extraReasons)
  return `已自动跳过${reasons.join('、')}，将对剩余 ${filtered.length} 个用户执行操作`
}

async function handleBatchToggle() {
  if (selectedRows.value.length === 0) return

  // 过滤掉 admin 和当前用户
  const operable = selectedRows.value.filter((r) => r.username !== 'admin' && r.id !== currentUserId.value)
  if (operable.length === 0) {
    ElMessage.warning('选中的用户中没有可操作的用户')
    return
  }
  const skipMsg = buildSkipMessage(selectedRows.value, operable)
  if (skipMsg) ElMessage.info(skipMsg)

  // 根据按钮标签判断操作：如果当前显示"禁用"，则执行禁用（enabled=false）；否则执行启用（enabled=true）
  const enabled = batchToggleLabel.value === '启用'
  const action = enabled ? '启用' : '禁用'

  const names = operable.map((r) => r.username).join('、')
  try {
    await ElMessageBox.confirm(`确定要${action}以下 ${operable.length} 个用户吗？\n${names}`, `批量${action}`, {
      type: 'warning',
    })
    const res = await batchToggleUsers(
      operable.map((r) => r.id),
      enabled
    )
    showBatchResult(res)
    selectedIds.value.clear()
    await loadData()
  } catch (e) {
    if (e !== 'cancel') {
      ElMessage.error(e.response?.data?.error || `批量${action}失败`)
    }
  }
}

async function handleBatchDelete() {
  if (selectedRows.value.length === 0) return

  // 过滤掉 admin 和当前用户
  const deletable = selectedRows.value.filter((r) => r.username !== 'admin' && r.id !== currentUserId.value)
  if (deletable.length === 0) {
    ElMessage.warning('选中的用户中没有可删除的用户')
    return
  }
  const skipMsg = buildSkipMessage(selectedRows.value, deletable)
  if (skipMsg) ElMessage.info(skipMsg)

  const names = deletable.map((r) => r.username).join('、')
  try {
    await ElMessageBox.confirm(`确定要删除以下 ${deletable.length} 个用户吗？\n${names}`, '批量删除', {
      type: 'warning',
    })
    const res = await batchDeleteUsers(deletable.map((r) => r.id))
    showBatchResult(res)
    selectedIds.value.clear()
    await loadData()
  } catch (e) {
    if (e !== 'cancel') {
      ElMessage.error(e.response?.data?.error || '批量删除失败')
    }
  }
}

async function handleBatchUnlock() {
  if (selectedRows.value.length === 0) return

  const lockable = selectedRows.value.filter((r) => r.locked)
  if (lockable.length === 0) {
    ElMessage.warning('选中的用户中没有被锁定的用户')
    return
  }

  const names = lockable.map((r) => r.username).join('、')
  try {
    await ElMessageBox.confirm(`确定要解锁以下 ${lockable.length} 个用户吗？\n${names}`, '批量解锁')
    const res = await batchUnlockUsers(lockable.map((r) => r.id))
    showBatchResult(res)
    selectedIds.value.clear()
    await loadData()
  } catch (e) {
    if (e !== 'cancel') {
      ElMessage.error(e.response?.data?.error || '批量解锁失败')
    }
  }
}

function handleMoreCommand(command) {
  switch (command) {
    case 'edit':
      if (selectedRows.value.length !== 1) return
      handleEditSelected()
      break
    case 'toggle':
      if (selectedRows.value.length === 0) return
      handleBatchToggle()
      break
    case 'resetPwd':
      if (selectedRows.value.length !== 1 || selectedRow.value?.username === 'admin' || selectedRow.value?.auth_source === 'ldap') return
      handleResetPwdSelected()
      break
    case 'batchUnlock':
      handleBatchUnlock()
      break
    case 'batchKick':
      handleBatchKick()
      break
    case 'delete':
      handleBatchDelete()
      break
  }
}

async function handleBatchKick() {
  if (selectedRows.value.length === 0) return

  const kickable = selectedRows.value.filter(
    (r) => r.online && r.username !== 'admin' && r.id !== currentUserId.value
  )
  if (kickable.length === 0) {
    ElMessage.warning('选中的用户中没有可下线的在线用户')
    return
  }
  const offlineSkipped = selectedRows.value.filter((r) => !r.online).length > 0
  const skipMsg = buildSkipMessage(selectedRows.value, kickable, ...(offlineSkipped ? ['离线用户'] : []))
  if (skipMsg) ElMessage.info(skipMsg)

  const names = kickable.map((r) => r.username).join('、')
  try {
    await ElMessageBox.confirm(`确定要强制下线以下 ${kickable.length} 个用户吗？\n${names}`, '批量下线', {
      type: 'warning',
    })
    const res = await batchKickUsers(kickable.map((r) => r.id))
    showBatchResult(res)
    selectedIds.value.clear()
    await loadData()
  } catch (e) {
    if (e !== 'cancel') {
      ElMessage.error(e.response?.data?.error || '批量下线失败')
    }
  }
}

function handleResetPwdSelected() {
  if (!selectedRow.value) return
  handleResetPwd(selectedRow.value)
}

function handleResetPwd(row) {
  resetPwdForm.value = { id: row.id, username: row.username, password: '' }
  resetPwdVisible.value = true
}

async function handleSubmit() {
  if (!form.value.username) {
    ElMessage.warning('请输入用户名')
    return
  }
  if (!form.value.name) {
    ElMessage.warning('请输入姓名')
    return
  }
  if (!form.value.email) {
    ElMessage.warning('请输入邮箱')
    return
  }
  if (!isEdit.value && !form.value.password) {
    ElMessage.warning('请输入密码')
    return
  }

  submitting.value = true
  try {
    if (isEdit.value) {
      await updateUser(editId.value, {
        username: form.value.username,
        name: form.value.name,
        email: form.value.email,
        role: form.value.role,
        enabled: form.value.enabled,
      })
      ElMessage.success('更新成功')
    } else {
      await createUser(form.value)
      ElMessage.success('添加成功')
    }
    dialogVisible.value = false
    await loadData()
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '操作失败')
  } finally {
    submitting.value = false
  }
}

async function handleSubmitResetPwd() {
  if (!resetPwdForm.value.password) {
    ElMessage.warning('请输入新密码')
    return
  }

  submitting.value = true
  try {
    await resetPassword(resetPwdForm.value.id, { password: resetPwdForm.value.password })
    ElMessage.success('密码重置成功')
    resetPwdVisible.value = false
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '重置失败')
  } finally {
    submitting.value = false
  }
}

// LDAP 导入
function handleLdapDialogClose() {
  ldapUsers.value = []
  selectedLdapUsernames.value = new Set()
  ldapSortProp.value = ''
  ldapSortOrder.value = ''
}

async function showLdapImport() {
  ldapImportVisible.value = true
  ldapLoading.value = true
  ldapSearch.value = ''
  ldapCurrentPage.value = 1
  selectedLdapUsernames.value = new Set()
  ldapSortProp.value = ''
  ldapSortOrder.value = ''
  try {
    const ldapList = await getLdapUsers()
    // 标记已导入的用户
    const importedUsernames = new Set(users.value.filter((u) => u.auth_source === 'ldap').map((u) => u.username))
    ldapUsers.value = (ldapList || []).map((u) => ({
      ...u,
      imported: importedUsernames.has(u.username),
    }))
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '获取 LDAP 用户失败')
    ldapImportVisible.value = false
  } finally {
    ldapLoading.value = false
  }
}

function handleLdapSelectionChange(rows) {
  const currentPageUsernames = new Set(paginatedLdapUsers.value.map((r) => r.username))
  const selectedOnPage = new Set(rows.map((r) => r.username))
  const newSet = new Set(selectedLdapUsernames.value)
  // 移除当前页中未选中的
  for (const name of currentPageUsernames) {
    if (!selectedOnPage.has(name)) {
      newSet.delete(name)
    }
  }
  // 添加当前页中选中的
  for (const name of selectedOnPage) {
    newSet.add(name)
  }
  selectedLdapUsernames.value = newSet
}

async function handleImportLdapUsers() {
  if (selectedLdapUsers.value.length === 0) return

  importing.value = true
  try {
    const res = await importLdapUsers(selectedLdapUsers.value)
    ElMessage.success(res.message)
    ldapImportVisible.value = false
    await loadData()
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '导入失败')
  } finally {
    importing.value = false
  }
}
</script>

<style scoped>
.ldap-pagination {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-top: 12px;
  padding: 8px 0;
}
.online-dot {
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  margin-left: 6px;
  vertical-align: middle;
}
.online-dot.online {
  background: #67c23a;
  box-shadow: 0 0 4px #67c23a80;
}
.online-dot.offline {
  background: #c0c4cc;
}
.lock-tag {
  margin-left: 4px;
}
</style>
