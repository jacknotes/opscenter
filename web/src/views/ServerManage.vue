<template>
  <div>
    <el-card>
      <template #header>
        <div style="display: flex; align-items: center; justify-content: space-between;">
          <span>服务器管理</span>
          <el-button type="primary" @click="handleAdd">添加服务器</el-button>
        </div>
      </template>

      <el-table :data="servers" stripe border>
        <el-table-column prop="id" label="ID" width="60" />
        <el-table-column prop="name" label="名称" width="150" />
        <el-table-column prop="host" label="IP地址" width="130" />
        <el-table-column prop="port" label="SSH端口" width="80" />
        <el-table-column prop="username" label="用户名" width="100" />
        <el-table-column prop="server_type" label="类型" width="120">
          <template #default="{ row }">
            <el-tag :type="row.server_type === 'lvs' ? '' : row.server_type === 'nginx' ? 'success' : 'warning'">{{ row.server_type === 'kubernetes' ? 'Kubernetes' : row.server_type }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="env" label="环境" width="80" />
        <el-table-column prop="description" label="描述" min-width="150" show-overflow-tooltip />
        <el-table-column label="操作" width="260" fixed="right">
          <template #default="{ row }">
            <el-button-group size="small">
              <el-button type="success" @click="handleTest(row)">测试连接</el-button>
              <el-button type="primary" @click="handleEdit(row)">编辑</el-button>
              <el-button type="warning" @click="handleCopy(row)">复制</el-button>
              <el-button type="danger" @click="handleDelete(row)">删除</el-button>
            </el-button-group>
          </template>
        </el-table-column>
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
        <el-form-item v-if="form.auth_type === 'password'" label="SSH密码">
          <el-input v-model="form.password" type="password" show-password :placeholder="(isEdit || isCopy) && form.has_password ? '已设置密码，留空表示不修改' : '请输入SSH密码'" />
        </el-form-item>
        <el-form-item v-if="form.auth_type === 'key'" label="私钥">
          <el-input v-model="form.private_key" type="textarea" :rows="4" :placeholder="(isEdit || isCopy) && form.has_private_key ? '已设置私钥，留空表示不修改' : '请输入私钥'" />
        </el-form-item>
        <el-form-item label="服务器类型" required>
          <el-select v-model="form.server_type">
            <el-option label="LVS" value="lvs" />
            <el-option label="Nginx" value="nginx" />
            <el-option label="Kubernetes" value="kubernetes" />
          </el-select>
        </el-form-item>
        <el-form-item label="环境">
          <el-input v-model="form.env" placeholder="env1 / env2 / both" />
        </el-form-item>
        <el-form-item label="脚本路径" v-if="form.server_type !== 'nginx'">
          <el-input v-model="form.script_path" placeholder="/shell/lvs.sh" />
        </el-form-item>
        <el-form-item label="脚本密码" v-if="form.server_type !== 'nginx'">
          <el-input v-model="form.script_password" type="password" show-password :placeholder="(isEdit || isCopy) && form.has_script_password ? '已设置密码，留空表示不修改' : '请输入脚本密码'" />
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
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleSubmit">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { getServers, getServerForEdit, createServer, updateServer, deleteServer, testConnection } from '../api'
import { ElMessage, ElMessageBox } from 'element-plus'

const servers = ref([])
const dialogVisible = ref(false)
const isEdit = ref(false)
const isCopy = ref(false)
const editId = ref(null)
const submitting = ref(false)
const form = ref(getDefaultForm())

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
    description: ''
  }
}

onMounted(() => {
  loadData()
})

async function loadData() {
  try {
    servers.value = await getServers()
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

async function handleDelete(row) {
  try {
    await ElMessageBox.confirm(`确定要删除服务器 "${row.name}" 吗？`, '确认删除')
    await deleteServer(row.id)
    ElMessage.success('删除成功')
    await loadData()
  } catch (e) {
    if (e !== 'cancel') {
      ElMessage.error('删除失败')
    }
  }
}

async function handleSubmit() {
  submitting.value = true
  try {
    if (isEdit.value) {
      await updateServer(editId.value, form.value)
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

async function handleTest(row) {
  const loading = ElMessage({ message: '正在测试连接...', type: 'info', duration: 0 })
  try {
    const res = await testConnection(row.id)
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
