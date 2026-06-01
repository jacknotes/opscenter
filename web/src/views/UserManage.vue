<template>
  <div>
    <el-card class="main-card">
      <template #header>
        <div class="toolbar">
          <el-button type="primary" @click="handleAdd">添加用户</el-button>
          <el-button :type="selectedRow?.enabled ? 'warning' : 'success'" @click="handleToggleSelected" :disabled="!selectedRow || selectedRow?.username === 'admin'">{{ selectedRow?.enabled ? '禁用' : '启用' }}</el-button>
          <el-button type="primary" @click="handleEditSelected" :disabled="!selectedRow">编辑</el-button>
          <el-button type="warning" @click="handleResetPwdSelected" :disabled="!selectedRow || selectedRow?.username === 'admin'">重置密码</el-button>
          <el-button type="danger" @click="handleDeleteSelected" :disabled="!selectedRow || selectedRow?.id === currentUserId || selectedRow?.username === 'admin'">删除</el-button>
          <el-input v-model="searchQuery" placeholder="搜索用户名 / 姓名 / 邮箱" clearable style="width: 250px; margin-left: auto;" />
        </div>
      </template>

      <el-table :data="filteredUsers" stripe border :row-class-name="({ row }) => row.enabled === false ? 'disabled-row' : ''" @selection-change="handleSelectionChange" ref="tableRef" v-force-reflow max-height="calc(100vh - 200px)">
        <el-table-column type="selection" width="45" />
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
        <el-table-column prop="created_at" label="创建时间" min-width="160">
          <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
        </el-table-column>
      </el-table>
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
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { getUsers, createUser, updateUser, deleteUser, resetPassword, toggleUserEnabled } from '../api'
import { useUserStore } from '../stores/user'
import { ElMessage, ElMessageBox } from 'element-plus'

const userStore = useUserStore()
const currentUserId = computed(() => userStore.userInfo?.id)

const users = ref([])
const searchQuery = ref('')

const filteredUsers = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return users.value
  return users.value.filter(u =>
    u.username.toLowerCase().includes(q) ||
    u.name.toLowerCase().includes(q) ||
    u.email.toLowerCase().includes(q) ||
    u.role.toLowerCase().includes(q)
  )
})

const dialogVisible = ref(false)
const isEdit = ref(false)
const editId = ref(null)
const submitting = ref(false)
const form = ref({ username: '', password: '', name: '', email: '', role: 'user', enabled: true })
const selectedRow = ref(null)
const tableRef = ref(null)

const resetPwdVisible = ref(false)
const resetPwdForm = ref({ id: null, username: '', password: '' })

function formatTime(t) {
  if (!t) return ''
  return new Date(t).toLocaleString('zh-CN')
}

function handleSelectionChange(rows) {
  if (rows.length > 1) {
    tableRef.value.clearSelection()
    tableRef.value.toggleRowSelection(rows[rows.length - 1], true)
    selectedRow.value = rows[rows.length - 1]
  } else if (rows.length === 1) {
    selectedRow.value = rows[0]
  } else {
    selectedRow.value = null
  }
}

onMounted(() => {
  loadData()
})

async function loadData() {
  try {
    users.value = await getUsers()
  } catch (e) {
    ElMessage.error('加载用户列表失败')
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

function handleDeleteSelected() {
  if (!selectedRow.value) return
  handleDelete(selectedRow.value)
}

async function handleDelete(row) {
  try {
    await ElMessageBox.confirm(`确定要删除用户 "${row.username}" 吗？`, '确认删除')
    await deleteUser(row.id)
    ElMessage.success('删除成功')
    selectedRow.value = null
    await loadData()
  } catch (e) {
    if (e !== 'cancel') {
      ElMessage.error(e.response?.data?.error || '删除失败')
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
  background: var(--card-bg, #141722);
  border-radius: 8px;
  border: 1px solid rgba(255, 255, 255, 0.06);
  flex-wrap: wrap;
}
:deep(.disabled-row) {
  background-color: rgba(255, 255, 255, 0.03) !important;
  opacity: 0.6;
}
:deep(.disabled-row:hover > td) {
  background-color: rgba(255, 255, 255, 0.05) !important;
}
</style>
