<template>
  <div>
    <el-card>
      <template #header>
        <div style="display: flex; align-items: center; justify-content: space-between;">
          <span>K8s部署</span>
          <div style="display: flex; gap: 10px;">
            <el-select v-model="serverId" placeholder="选择K8s服务器" style="width: 200px" @change="loadData">
              <el-option v-for="s in servers" :key="s.id" :label="s.name" :value="s.id" />
            </el-select>
            <el-button type="primary" @click="handleFull('online')">全量上线</el-button>
            <el-button type="warning" @click="handleFull('sync')">全量同步</el-button>
            <el-button type="danger" @click="handleFull('rollback')">全量回滚</el-button>
          </div>
        </div>
      </template>

      <el-table :data="rollouts" stripe border @selection-change="handleSelectionChange">
        <el-table-column type="selection" width="55" />
        <el-table-column prop="namespace" label="命名空间" width="150" />
        <el-table-column prop="name" label="名称" min-width="250" />
        <el-table-column prop="strategy" label="策略" width="100" />
        <el-table-column prop="status" label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="row.status === 'Paused' ? 'warning' : 'success'">{{ row.status }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="step" label="步骤" width="80" />
        <el-table-column prop="ready" label="就绪" width="80" />
        <el-table-column label="操作" width="250">
          <template #default="{ row }">
            <el-button-group size="small">
              <el-button type="success" @click="handleSingle(row, 'online')">上线</el-button>
              <el-button type="warning" @click="handleSingle(row, 'sync')">同步</el-button>
              <el-button type="danger" @click="handleSingle(row, 'rollback')">回滚</el-button>
            </el-button-group>
          </template>
        </el-table-column>
      </el-table>

      <div style="margin-top: 15px;">
        <el-button type="primary" :disabled="selected.length === 0" @click="handleBatch('online')">批量上线</el-button>
        <el-button type="warning" :disabled="selected.length === 0" @click="handleBatch('sync')">批量同步</el-button>
        <el-button type="danger" :disabled="selected.length === 0" @click="handleBatch('rollback')">批量回滚</el-button>
      </div>
    </el-card>

    <!-- Preview Dialog -->
    <el-dialog v-model="previewVisible" title="变更预览" width="600px">
      <div v-if="previewData">
        <p><strong>操作：</strong>{{ previewData.description }}</p>
        <p><strong>命令：</strong></p>
        <ul>
          <li v-for="cmd in previewData.commands" :key="cmd"><code>{{ cmd }}</code></li>
        </ul>
        <el-divider />
        <p><strong>当前状态：</strong></p>
        <pre style="background: #f5f5f5; padding: 10px; border-radius: 4px; max-height: 300px; overflow-y: auto;">{{ previewData.current_status }}</pre>
      </div>
      <template #footer>
        <el-button @click="previewVisible = false">取消</el-button>
        <el-button type="primary" :loading="executing" @click="executePreview">确认执行</el-button>
      </template>
    </el-dialog>

    <!-- Output Area -->
    <el-card v-if="output" style="margin-top: 20px;">
      <template #header>执行结果</template>
      <pre style="background: #1e1e1e; color: #d4d4d4; padding: 15px; border-radius: 4px; max-height: 400px; overflow-y: auto;">{{ output }}</pre>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import {
  getServers, getK8sRollouts,
  k8sOnlinePreview, k8sOnlineExecute,
  k8sSyncPreview, k8sSyncExecute,
  k8sRollbackPreview, k8sRollbackExecute,
  k8sFullOnlinePreview, k8sFullOnlineExecute,
  k8sFullSyncPreview, k8sFullSyncExecute,
  k8sFullRollbackPreview, k8sFullRollbackExecute
} from '../api'
import { ElMessage } from 'element-plus'

const servers = ref([])
const serverId = ref(null)
const rollouts = ref([])
const selected = ref([])
const previewVisible = ref(false)
const previewData = ref(null)
const previewId = ref('')
const executing = ref(false)
const output = ref('')
const currentAction = ref('')

onMounted(async () => {
  try {
    servers.value = await getServers('k8s_master')
    if (servers.value.length > 0) {
      serverId.value = servers.value[0].id
      await loadData()
    }
  } catch (e) {
    console.error('Failed to load servers:', e)
  }
})

async function loadData() {
  if (!serverId.value) return
  try {
    rollouts.value = await getK8sRollouts(serverId.value)
  } catch (e) {
    ElMessage.error('加载数据失败')
  }
}

function handleSelectionChange(val) {
  selected.value = val
}

async function handleSingle(row, action) {
  const previewFn = {
    online: k8sOnlinePreview,
    sync: k8sSyncPreview,
    rollback: k8sRollbackPreview
  }[action]

  try {
    const res = await previewFn({
      server_id: serverId.value,
      projects: [{ name: row.name, namespace: row.namespace }]
    })
    previewData.value = res
    previewId.value = res.preview_id
    currentAction.value = action
    previewVisible.value = true
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '预览失败')
  }
}

async function handleBatch(action) {
  const previewFn = {
    online: k8sOnlinePreview,
    sync: k8sSyncPreview,
    rollback: k8sRollbackPreview
  }[action]

  try {
    const res = await previewFn({
      server_id: serverId.value,
      projects: selected.value.map(r => ({ name: r.name, namespace: r.namespace }))
    })
    previewData.value = res
    previewId.value = res.preview_id
    currentAction.value = action
    previewVisible.value = true
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '预览失败')
  }
}

async function handleFull(action) {
  const previewFn = {
    online: k8sFullOnlinePreview,
    sync: k8sFullSyncPreview,
    rollback: k8sFullRollbackPreview
  }[action]

  try {
    const res = await previewFn({ server_id: serverId.value })
    previewData.value = res
    previewId.value = res.preview_id
    currentAction.value = 'full_' + action
    previewVisible.value = true
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '预览失败')
  }
}

async function executePreview() {
  executing.value = true
  const executeFn = {
    online: k8sOnlineExecute,
    sync: k8sSyncExecute,
    rollback: k8sRollbackExecute,
    full_online: k8sFullOnlineExecute,
    full_sync: k8sFullSyncExecute,
    full_rollback: k8sFullRollbackExecute
  }[currentAction.value]

  try {
    const res = await executeFn({ preview_id: previewId.value })
    output.value = JSON.stringify(res.output, null, 2)
    ElMessage.success('执行成功')
    previewVisible.value = false
    await loadData()
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '执行失败')
    output.value = JSON.stringify(e.response?.data?.output, null, 2) || ''
  } finally {
    executing.value = false
  }
}
</script>
