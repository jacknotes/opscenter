<template>
  <div>
    <el-card class="main-card">
      <template #header>
        <div class="toolbar">
          <el-button type="primary" @click="handleAdd">添加服务器</el-button>
          <el-button type="success" @click="handleTest" :disabled="!selectedRow">测试连接</el-button>
          <el-button type="primary" @click="handleEditSelected" :disabled="!selectedRow">编辑</el-button>
          <el-button type="info" class="el-button--cyan" @click="handleCopySelected" :disabled="!selectedRow">复制</el-button>
          <el-button type="danger" @click="handleDeleteSelected" :disabled="!selectedRow">删除</el-button>
          <el-input v-model="searchQuery" placeholder="搜索名称 / IP / 类型" clearable style="width: 250px; margin-left: auto;" />
        </div>
      </template>

      <el-table :data="filteredServers" stripe border :row-class-name="tableRowClassName" @selection-change="handleSelectionChange" ref="tableRef" v-force-reflow max-height="calc(100vh - 200px)">
        <el-table-column type="selection" width="45" />
        <el-table-column label="状态" width="80" align="center">
          <template #default="{ row }">
            <el-tag :type="row.enabled ? 'success' : 'info'" size="small">{{ row.enabled ? '已启用' : '已禁用' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="name" label="名称" min-width="120" />
        <el-table-column prop="host" label="IP地址" min-width="120" />
        <el-table-column prop="port" label="SSH端口" width="80" />
        <el-table-column prop="username" label="用户名" min-width="100" />
        <el-table-column prop="server_type" label="类型" min-width="140">
          <template #default="{ row }">
            <el-tag :type="row.server_type === 'lvs' ? 'primary' : row.server_type === 'nginx' ? 'success' : row.server_type === 'preprod' ? 'warning' : 'info'">{{ row.server_type === 'kubernetes' ? 'k8s' : row.server_type === 'preprod' ? 'k8s-prepro' : row.server_type }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="env" label="环境" width="80" />
        <el-table-column prop="description" label="描述" min-width="200" show-overflow-tooltip />
      </el-table>
    </el-card>

    <!-- Add/Edit Dialog -->
    <el-dialog v-model="dialogVisible" :title="isEdit ? '编辑服务器' : (isCopy ? '复制服务器' : '添加服务器')" width="600px">
      <el-form :model="form" label-width="120px">
        <el-form-item label="名称" required>
          <el-input v-model="form.name" />
        </el-form-item>
        <el-form-item label="IP地址" required>
          <el-input v-model="form.host" />
        </el-form-item>
        <el-form-item label="SSH端口">
          <el-input-number v-model="form.port" :min="1" :max="65535" />
        </el-form-item>
        <el-form-item label="用户名" required>
          <el-input v-model="form.username" />
        </el-form-item>
        <el-form-item label="认证类型" required>
          <el-radio-group v-model="form.auth_type">
            <el-radio value="password">密码</el-radio>
            <el-radio value="key">密钥</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="form.auth_type === 'password'" label="SSH密码" :required="!isEdit">
          <el-input v-model="form.password" type="password" show-password :placeholder="isEdit && form.has_password ? '已设置密码，留空表示不修改' : '请输入SSH密码'" />
        </el-form-item>
        <el-form-item v-if="form.auth_type === 'key'" label="私钥" :required="!isEdit">
          <el-input v-model="form.private_key" type="textarea" :rows="4" :placeholder="isEdit && form.has_private_key ? '已设置私钥，留空表示不修改' : '请输入私钥'" />
        </el-form-item>
        <el-form-item label="服务器类型" required>
          <el-select v-model="form.server_type">
            <el-option label="LVS" value="lvs" />
            <el-option label="Nginx" value="nginx" />
            <el-option label="k8s" value="kubernetes" />
            <el-option label="k8s-prepro" value="preprod" />
          </el-select>
        </el-form-item>
        <el-form-item label="环境">
          <el-input v-model="form.env" placeholder="env1 / env2 / both" />
        </el-form-item>
        <el-form-item label="脚本路径" v-if="form.server_type !== 'nginx'">
          <el-input v-model="form.script_path" placeholder="/shell/lvs.sh" />
        </el-form-item>
        <el-form-item label="脚本密码" v-if="form.server_type !== 'nginx'">
          <el-input v-model="form.script_password" type="password" show-password :placeholder="isEdit && form.has_script_password ? '已设置密码，留空表示不修改' : '请输入脚本密码'" />
        </el-form-item>
        <el-form-item label="配置路径" v-if="form.server_type === 'nginx'">
          <el-input v-model="form.config_path" placeholder="Nginx配置目录" />
        </el-form-item>
        <el-form-item label="配置文件模式" v-if="form.server_type === 'nginx'">
          <el-input v-model="form.config_pattern" placeholder="upstreamserver_*.conf" />
        </el-form-item>
        <el-form-item label="备份路径" v-if="form.server_type === 'nginx'">
          <el-input v-model="form.backup_path" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="form.description" type="textarea" />
        </el-form-item>
        <el-form-item label="状态">
          <el-switch v-model="form.enabled" active-text="启用" inactive-text="禁用" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleSubmit">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { getServers, getServerForEdit, createServer, updateServer, deleteServer, testConnection } from '../api'
import { ElMessage, ElMessageBox } from 'element-plus'

const servers = ref([])
const searchQuery = ref('')

const filteredServers = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return servers.value
  return servers.value.filter(s =>
    s.name.toLowerCase().includes(q) ||
    s.host.toLowerCase().includes(q) ||
    s.server_type.toLowerCase().includes(q) ||
    s.env.toLowerCase().includes(q) ||
    String(s.port).includes(q)
  )
})
const dialogVisible = ref(false)
const isEdit = ref(false)
const isCopy = ref(false)
const editId = ref(null)
const submitting = ref(false)
const form = ref(getDefaultForm())
const selectedRow = ref(null)
const tableRef = ref(null)

function getDefaultForm() {
  return {
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
    enabled: true
  }
}

function tableRowClassName({ row }) {
  if (row.enabled === false) return 'disabled-row'
  return ''
}

function handleSelectionChange(rows) {
  if (rows.length > 1) {
    // 只允许选中一个，取消之前的选择
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
    servers.value = await getServers(undefined, true)
  } catch (e) {
    ElMessage.error('加载服务器列表失败')
  }
}

function handleAdd() {
  isEdit.value = false
  isCopy.value = false
  editId.value = null
  form.value = getDefaultForm()
  dialogVisible.value = true
}

function handleCopySelected() {
  if (!selectedRow.value) return
  handleCopy(selectedRow.value)
}

async function handleCopy(row) {
  isEdit.value = false
  isCopy.value = true
  editId.value = null
  try {
    const data = await getServerForEdit(row.id)
    form.value = { ...data, name: data.name + ' (副本)' }
    dialogVisible.value = true
  } catch (e) {
    ElMessage.error('获取服务器信息失败')
  }
}

function handleEditSelected() {
  if (!selectedRow.value) return
  handleEdit(selectedRow.value)
}

async function handleEdit(row) {
  isEdit.value = true
  isCopy.value = false
  editId.value = row.id
  try {
    const data = await getServerForEdit(row.id)
    form.value = data
    dialogVisible.value = true
  } catch (e) {
    ElMessage.error('获取服务器信息失败')
  }
}

function handleDeleteSelected() {
  if (!selectedRow.value) return
  handleDelete(selectedRow.value)
}

async function handleDelete(row) {
  try {
    await ElMessageBox.confirm(`确定要删除服务器 "${row.name}" 吗？`, '确认删除')
    await deleteServer(row.id)
    ElMessage.success('删除成功')
    selectedRow.value = null
    await loadData()
  } catch (e) {
    if (e !== 'cancel') {
      ElMessage.error('删除失败')
    }
  }
}

async function handleSubmit() {
  if (!isEdit.value) {
    if (form.value.auth_type === 'password' && !form.value.password) {
      ElMessage.warning('请输入SSH密码')
      return
    }
    if (form.value.auth_type === 'key' && !form.value.private_key) {
      ElMessage.warning('请输入私钥')
      return
    }
  }
  submitting.value = true
  try {
    if (isEdit.value) {
      const data = { ...form.value }
      if (data.auth_type === 'password') {
        if (!data.password && form.value.has_password) data.password = '__keep__'
      } else {
        if (!data.private_key && form.value.has_private_key) data.private_key = '__keep__'
      }
      if (!data.script_password && form.value.has_script_password) data.script_password = '__keep__'
      await updateServer(editId.value, data)
      ElMessage.success('更新成功')
    } else {
      await createServer(form.value)
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

async function handleTest() {
  if (!selectedRow.value) return
  const loading = ElMessage({ message: '正在测试连接...', type: 'info', duration: 0 })
  try {
    const res = await testConnection(selectedRow.value.id)
    loading.close()
    if (res.success) {
      ElMessage.success(res.message)
    } else {
      ElMessage.error(res.error)
    }
  } catch (e) {
    loading.close()
    ElMessage.error(e.response?.data?.error || '测试失败')
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
  background: #fff;
  border-radius: 8px;
  border: 1px solid #e4e7ed;
  flex-wrap: wrap;
}
:deep(.disabled-row) {
  background-color: #fafafa !important;
  opacity: 0.6;
}
:deep(.disabled-row:hover > td) {
  background-color: #f5f5f5 !important;
}
</style>
