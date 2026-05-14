<template>
  <div>
    <el-card>
      <template #header>
        <div style="display: flex; align-items: center; justify-content: flex-end;">
          <el-select v-model="serverId" placeholder="选择K8s服务器" style="width: 200px" @change="loadData">
            <el-option v-for="s in servers" :key="s.id" :label="s.name" :value="s.id" />
          </el-select>
        </div>
      </template>

      <el-steps :active="currentStep" finish-status="success" style="margin-bottom: 20px;">
        <el-step title="查看状态" />
        <el-step title="缩容" />
        <el-step title="部署验证" />
        <el-step title="扩容" />
      </el-steps>

      <!-- Step 1: Status -->
      <div v-if="currentStep === 0">
        <el-table :data="resources" stripe border>
          <el-table-column prop="category" label="类型" width="100">
            <template #default="{ row }">
              <el-tag :type="row.category === 'rollout' ? '' : row.category === 'deployment' ? 'success' : 'warning'">{{ row.category }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="name" label="名称" min-width="300" />
          <el-table-column prop="desired" label="期望副本" width="100" />
          <el-table-column prop="current" label="当前副本" width="100" />
          <el-table-column prop="available" label="可用副本" width="100" />
          <el-table-column prop="age" label="年龄" width="100" />
        </el-table>
        <el-button type="primary" style="margin-top: 15px;" @click="currentStep = 1">下一步：缩容</el-button>
      </div>

      <!-- Step 2: Scale Down -->
      <div v-if="currentStep === 1">
        <el-alert title="缩容操作将把所有资源副本数降至0" type="warning" :closable="false" style="margin-bottom: 15px;" />
        <el-button type="danger" @click="handleScaleDown">缩容预览</el-button>
        <el-button @click="currentStep = 0">上一步</el-button>
      </div>

      <!-- Step 3: Verify -->
      <div v-if="currentStep === 2">
        <el-alert title="请手动验证部署是否正确" type="info" :closable="false" style="margin-bottom: 15px;" />
        <el-button type="primary" @click="currentStep = 3">验证通过，下一步扩容</el-button>
        <el-button @click="currentStep = 1">上一步</el-button>
      </div>

      <!-- Step 4: Scale Up -->
      <div v-if="currentStep === 3">
        <el-alert title="扩容操作将把所有资源恢复到目标副本数" type="success" :closable="false" style="margin-bottom: 15px;" />
        <el-button type="success" @click="handleScaleUp">扩容预览</el-button>
        <el-button @click="currentStep = 2">上一步</el-button>
      </div>
    </el-card>

    <!-- Preview Dialog -->
    <el-dialog v-model="previewVisible" title="变更预览" width="600px">
      <div v-if="previewData">
        <p><strong>操作：</strong>{{ previewData.description }}</p>
        <p><strong>命令：</strong><code>{{ previewData.command }}</code></p>
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
import { getServers, getPreprodStatus, preprodScaleDownPreview, preprodScaleDownExecute, preprodScaleUpPreview, preprodScaleUpExecute } from '../api'
import { ElMessage } from 'element-plus'

const servers = ref([])
const serverId = ref(null)
const resources = ref([])
const currentStep = ref(0)
const previewVisible = ref(false)
const previewData = ref(null)
const previewId = ref('')
const executing = ref(false)
const output = ref('')
const currentAction = ref('')

onMounted(async () => {
  try {
    servers.value = await getServers('kubernetes')
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
    resources.value = await getPreprodStatus(serverId.value)
  } catch (e) {
    ElMessage.error('加载数据失败')
  }
}

async function handleScaleDown() {
  try {
    const res = await preprodScaleDownPreview({ server_id: serverId.value })
    previewData.value = res
    previewId.value = res.preview_id
    currentAction.value = 'scaledown'
    previewVisible.value = true
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '预览失败')
  }
}

async function handleScaleUp() {
  try {
    const res = await preprodScaleUpPreview({ server_id: serverId.value })
    previewData.value = res
    previewId.value = res.preview_id
    currentAction.value = 'scaleup'
    previewVisible.value = true
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '预览失败')
  }
}

async function executePreview() {
  executing.value = true
  const executeFn = currentAction.value === 'scaledown' ? preprodScaleDownExecute : preprodScaleUpExecute

  try {
    const res = await executeFn({ preview_id: previewId.value })
    output.value = res.output
    ElMessage.success('执行成功')
    previewVisible.value = false
    await loadData()
    if (currentAction.value === 'scaledown') {
      currentStep.value = 2
    } else {
      currentStep.value = 0
    }
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '执行失败')
    output.value = e.response?.data?.output || ''
  } finally {
    executing.value = false
  }
}
</script>
