<template>
  <div>
    <el-card class="main-card">
      <template #header>
        <div class="toolbar">
          <el-button type="primary" @click="handleAdd">添加用户</el-button>
          <el-button type="success" @click="showLdapImport" v-if="ldapEnabled">导入 LDAP 用户</el-button>
          <el-button :type="batchToggleType" @click="handleBatchToggle" :disabled="selectedRows.length === 0">{{ batchToggleLabel }}</el-button>
          <el-button type="primary" @click="handleEditSelected" :disabled="selectedRows.length !== 1">编辑</el-button>
          <el-button type="warning" @click="handleResetPwdSelected" :disabled="selectedRows.length !== 1 || selectedRow?.username === 'admin' || selectedRow?.auth_source === 'ldap'">重置密码</el-button>
          <el-button type="danger" @click="handleBatchDelete" :disabled="selectedRows.length === 0">删除</el-button>
          <el-button type="info" class="el-button--cyan" @click="handleRefresh" :loading="loading">刷新</el-button>
          <el-input v-model="searchQuery" placeholder="搜索用户信息" clearable style="width: 300px; margin-left: auto;" />
        </div>
      </template>

      <el-table :data="paginatedUsers" stripe border :row-class-name="({ row }) => row.enabled === false ? 'disabled-row' : ''" @selection-change="handleSelectionChange" ref="tableRef" v-force-reflow max-height="calc(100vh - 250px)">
        <el-table-column type="selection" width="55" />
        <el-table-column label="状态" width="80" align="center">
          <template #default="{ row }">
            <el-tag :type="row.enabled ? 'success' : 'info'" size="small">{{ row.enabled ? '已启用' : '已禁用' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="username" label="用户名" min-width="100" />
        <el-table-column prop="name" label="姓名" min-width="100" />
        <el-table-column prop="email" label="邮箱" min-width="180" />
        <el-table-column label="角色" width="100">
          <template #default="{ row }">
            <el-tag :type="row.role === 'admin' ? 'danger' : 'info'">{{ row.role === 'admin' ? '管理员' : '普通用户' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="认证来源" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="row.auth_source === 'ldap' ? 'warning' : 'info'" size="small">{{ row.auth_source === 'ldap' ? 'LDAP' : '本地' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" min-width="160">
          <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
        </el-table-column>
      </el-table>

      <div class="pagination-wrapper">
        <div class="pagination-left">
          <span class="selection-count">已选 {{ selectedRows.length }}</span>
        </div>
        <el-pagination
          v-model:current-page="currentPage"
          v-model:page-size="pageSize"
          :page-sizes="[20, 50, 100, 200]"
          :total="filteredUsers.length"
          layout="total, sizes, prev, pager, next, jumper"
          @size-change="handleSizeChange"
          @current-change="handleCurrentChange"
        />
      </div>
    </el-card>

    <!-- Add/Edit Dialog -->
    <el-dialog v-model="dialogVisible" :title="isEdit ? '编辑用户' : '添加用户'" width="500px">
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
    <el-dialog v-model="resetPwdVisible" title="重置密码" width="400px">
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
    <el-dialog v-model="ldapImportVisible" title="导入 LDAP 用户" width="800px" @close="ldapUsers = []">
      <div v-if="ldapLoading" style="text-align: center; padding: 40px;">
        <el-icon class="is-loading" :size="32"><Loading /></el-icon>
        <p style="margin-top: 12px; color: #94A3B8;">正在从 LDAP 获取用户列表...</p>
      </div>
      <div v-else>
        <div style="margin-bottom: 12px; display: flex; justify-content: space-between; align-items: center;">
          <span style="font-size: 14px; color: #94A3B8;">选择要导入的用户，已导入的用户将自动跳过</span>
          <div style="display: flex; gap: 8px;">
            <el-input v-model="ldapSearch" placeholder="搜索用户" clearable style="width: 200px;" />
            <el-button type="info" class="el-button--cyan" @click="showLdapImport" :loading="ldapLoading">刷新</el-button>
          </div>
        </div>
        <el-table :data="paginatedLdapUsers" stripe border max-height="400" @selection-change="handleLdapSelectionChange" ref="ldapTableRef">
          <el-table-column type="selection" width="55" :selectable="(row) => !row.imported" />
          <el-table-column prop="username" label="用户名" width="120" />
          <el-table-column prop="name" label="姓名" min-width="120" />
          <el-table-column prop="email" label="邮箱" min-width="180" />
          <el-table-column label="状态" width="100" align="center">
            <template #default="{ row }">
              <el-tag v-if="row.imported" type="success" size="small">已导入</el-tag>
              <el-tag v-else type="info" size="small">未导入</el-tag>
            </template>
          </el-table-column>
        </el-table>
        <div class="ldap-pagination">
          <span class="selection-count">已选 {{ selectedLdapUsers.length }}</span>
          <el-pagination
            v-model:current-page="ldapCurrentPage"
            v-model:page-size="ldapPageSize"
            :page-sizes="[20, 50, 100]"
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
        <el-button type="primary" :loading="importing" :disabled="selectedLdapUsers.length === 0" @click="handleImportLdapUsers">
          导入 ({{ selectedLdapUsers.length }})
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { getUsers, createUser, updateUser, batchDeleteUsers, batchToggleUsers, resetPassword, toggleUserEnabled, getLdapUsers, importLdapUsers } from '../api'
import { useUserStore } from '../stores/user'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Loading } from '@element-plus/icons-vue'

const userStore = useUserStore()
const currentUserId = computed(() => userStore.userInfo?.id)

const users = ref([])
const searchQuery = ref('')
const currentPage = ref(1)
const pageSize = ref(20)

const filteredUsers = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return users.value
  return users.value.filter(u => {
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
      (q === '本地' && u.auth_source !== 'ldap')
    )
  })
})

const paginatedUsers = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  return filteredUsers.value.slice(start, start + pageSize.value)
})

const batchToggleType = computed(() => {
  if (selectedRows.value.length === 0) return 'success'
  // 如果选中的都是禁用状态，则显示启用按钮；否则显示禁用按钮
  const allDisabled = selectedRows.value.every(r => !r.enabled)
  return allDisabled ? 'success' : 'warning'
})

const batchToggleLabel = computed(() => {
  if (selectedRows.value.length === 0) return '启用'
  const allDisabled = selectedRows.value.every(r => !r.enabled)
  return allDisabled ? '启用' : '禁用'
})

const dialogVisible = ref(false)
const isEdit = ref(false)
const editId = ref(null)
const submitting = ref(false)
const loading = ref(false)
const form = ref({ username: '', password: '', name: '', email: '', role: 'user', enabled: true })
const selectedRow = ref(null)
const selectedRows = ref([])
const tableRef = ref(null)

const resetPwdVisible = ref(false)
const resetPwdForm = ref({ id: null, username: '', password: '' })

// LDAP 导入相关
const ldapEnabled = ref(false)
const ldapImportVisible = ref(false)
const ldapLoading = ref(false)
const importing = ref(false)
const ldapUsers = ref([])
const ldapSearch = ref('')
const selectedLdapUsers = ref([])
const ldapTableRef = ref(null)
const ldapCurrentPage = ref(1)
const ldapPageSize = ref(20)

const filteredLdapUsers = computed(() => {
  const q = ldapSearch.value.trim().toLowerCase()
  if (!q) return ldapUsers.value
  return ldapUsers.value.filter(u =>
    u.username.toLowerCase().includes(q) ||
    (u.name && u.name.toLowerCase().includes(q)) ||
    (u.email && u.email.toLowerCase().includes(q))
  )
})

const paginatedLdapUsers = computed(() => {
  const start = (ldapCurrentPage.value - 1) * ldapPageSize.value
  return filteredLdapUsers.value.slice(start, start + ldapPageSize.value)
})

function handleLdapSizeChange() {
  ldapCurrentPage.value = 1
  selectedLdapUsers.value = []
}

function handleLdapCurrentChange() {
  selectedLdapUsers.value = []
}

function formatTime(t) {
  if (!t) return ''
  return new Date(t).toLocaleString('zh-CN')
}

// HTML 转义，防止 XSS
function escapeHtml(str) {
  return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;').replace(/'/g, '&#39;')
}

// 显示批量操作结果消息
function showBatchResult(res) {
  const message = res.message || '操作完成'
  const success = res.deleted || res.updated || 0
  const failed = res.failed || 0

  // 转义 HTML 后将换行符转换为 <br>
  const htmlMessage = escapeHtml(message).replace(/\n/g, '<br>')

  if (failed === 0) {
    ElMessage({ message: htmlMessage, type: 'success', dangerouslyUseHTMLString: true })
  } else if (success === 0) {
    ElMessage({ message: htmlMessage, type: 'error', dangerouslyUseHTMLString: true })
  } else {
    ElMessage({ message: htmlMessage, type: 'warning', dangerouslyUseHTMLString: true, duration: 5000 })
  }
}

function handleSelectionChange(rows) {
  selectedRows.value = rows
  // 单选逻辑：用于编辑/禁用/重置密码等操作
  if (rows.length === 1) {
    selectedRow.value = rows[0]
  } else if (rows.length > 1) {
    selectedRow.value = rows[rows.length - 1]
  } else {
    selectedRow.value = null
  }
}

function handleSizeChange() {
  currentPage.value = 1
  selectedRows.value = []
  selectedRow.value = null
}

function handleCurrentChange() {
  // 页码变化时清除选择
  selectedRows.value = []
  selectedRow.value = null
}

// 搜索时重置到第一页
watch(searchQuery, () => {
  currentPage.value = 1
})

onMounted(() => {
  loadData()
  checkLdapEnabled()
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
  form.value = { username: row.username, name: row.name, email: row.email, password: '', role: row.role, enabled: row.enabled }
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

async function handleBatchToggle() {
  if (selectedRows.value.length === 0) return

  // 过滤掉 admin 和当前用户
  const operable = selectedRows.value.filter(r => r.username !== 'admin' && r.id !== currentUserId.value)
  if (operable.length === 0) {
    ElMessage.warning('选中的用户中没有可操作的用户')
    return
  }

  // 根据按钮标签判断操作：如果当前显示"禁用"，则执行禁用（enabled=false）；否则执行启用（enabled=true）
  const enabled = batchToggleLabel.value === '启用'
  const action = enabled ? '启用' : '禁用'

  const names = operable.map(r => r.username).join('、')
  try {
    await ElMessageBox.confirm(`确定要${action}以下 ${operable.length} 个用户吗？\n${names}`, `批量${action}`, { type: 'warning' })
    const res = await batchToggleUsers(operable.map(r => r.id), enabled)
    showBatchResult(res)
    selectedRow.value = null
    selectedRows.value = []
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
  const deletable = selectedRows.value.filter(r => r.username !== 'admin' && r.id !== currentUserId.value)
  if (deletable.length === 0) {
    ElMessage.warning('选中的用户中没有可删除的用户')
    return
  }

  const names = deletable.map(r => r.username).join('、')
  try {
    await ElMessageBox.confirm(`确定要删除以下 ${deletable.length} 个用户吗？\n${names}`, '批量删除', { type: 'warning' })
    const res = await batchDeleteUsers(deletable.map(r => r.id))
    showBatchResult(res)
    selectedRow.value = null
    selectedRows.value = []
    await loadData()
  } catch (e) {
    if (e !== 'cancel') {
      ElMessage.error(e.response?.data?.error || '批量删除失败')
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
      await updateUser(editId.value, { username: form.value.username, name: form.value.name, email: form.value.email, role: form.value.role, enabled: form.value.enabled })
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
async function showLdapImport() {
  ldapImportVisible.value = true
  ldapLoading.value = true
  ldapSearch.value = ''
  ldapCurrentPage.value = 1
  selectedLdapUsers.value = []
  try {
    const ldapList = await getLdapUsers()
    // 标记已导入的用户
    const importedUsernames = new Set(users.value.filter(u => u.auth_source === 'ldap').map(u => u.username))
    ldapUsers.value = (ldapList || []).map(u => ({
      ...u,
      imported: importedUsernames.has(u.username)
    }))
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '获取 LDAP 用户失败')
    ldapImportVisible.value = false
  } finally {
    ldapLoading.value = false
  }
}

function handleLdapSelectionChange(rows) {
  selectedLdapUsers.value = rows
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
:deep(.el-card__header) {
  border-bottom: none;
  padding-bottom: 0;
}
.toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 12px;
  padding: 10px 14px;
  background: var(--bg-elevated);
  border-radius: 8px;
  border: 1px solid var(--border-default);
  flex-wrap: wrap;
}
:deep(.disabled-row) {
  background-color: var(--bg-elevated) !important;
  opacity: 0.6;
}
:deep(.disabled-row:hover > td) {
  background-color: var(--bg-elevated) !important;
}
.pagination-wrapper {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-top: 16px;
  padding: 8px 0;
}
.pagination-left {
  display: flex;
  align-items: center;
}
.selection-count {
  font-size: var(--el-pagination-font-size, 13px);
  color: var(--el-text-color-regular);
  white-space: nowrap;
  line-height: 32px;
}
.ldap-pagination {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-top: 12px;
  padding: 8px 0;
}
</style>
