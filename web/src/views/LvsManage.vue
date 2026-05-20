<template>
  <div>
    <el-card class="main-card">
      <template #header>
        <div style="display: flex; align-items: center; gap: 10px;">
          <span class="filter-label">服务器:</span>
          <el-select v-model="serverId" placeholder="选择LVS服务器" style="width: 200px" @change="loadData">
            <el-option v-for="s in servers" :key="s.id" :label="s.name" :value="s.id" />
          </el-select>
        </div>
      </template>

      <el-table :data="lvsData" stripe border>
        <el-table-column prop="ip" label="Virtual Server" width="150">
          <template #default="{ row }">{{ row.ip }}:{{ row.port }}</template>
        </el-table-column>
        <el-table-column prop="protocol" label="协议" width="80" />
        <el-table-column label="Real Servers">
          <template #default="{ row }">
            <div v-for="rs in row.real_servers" :key="rs.ip" style="display: flex; align-items: center; gap: 10px; margin: 5px 0;">
              <span>{{ rs.ip }}:{{ rs.port }}</span>
              <el-tag :type="rs.status === 'up' ? 'success' : 'danger'" size="small">{{ rs.status }}</el-tag>
              <span>Weight: {{ rs.weight }}</span>
              <span>Conn: {{ rs.active_conn }}</span>
              <el-button-group size="small">
                <el-button type="success" @click="handleOp(row.ip, rs.ip, 'on')">上线</el-button>
                <el-button type="danger" @click="handleOp(row.ip, rs.ip, 'off')">下线</el-button>
              </el-button-group>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="切换" width="100">
          <template #default="{ row }">
            <el-button v-if="row.real_servers?.length >= 2" type="primary" size="small" @click="handleSwap(row)">切换</el-button>
          </template>
        </el-table-column>
      </el-table>
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
      <pre class="terminal-pre">{{ output }}</pre>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { getServers, getLvsList, lvsOpPreview, lvsOpExecute, lvsSwapPreview, lvsSwapExecute } from '../api'
import { ElMessage } from 'element-plus'

const servers = ref([])
const serverId = ref(null)
const lvsData = ref([])
const previewVisible = ref(false)
const previewData = ref(null)
const previewId = ref('')
const executing = ref(false)
const output = ref('')
const currentAction = ref('')

onMounted(async () => {
  try {
    servers.value = (await getServers('lvs')) || []
    if (servers.value.length > 0) {
      const saved = localStorage.getItem('lvs_server')
      if (saved && servers.value.some(s => s.id === Number(saved))) {
        serverId.value = Number(saved)
      } else {
        serverId.value = servers.value[0].id
      }
      await loadData()
    }
  } catch (e) {
    console.error('Failed to load servers:', e)
  }
})

async function loadData() {
  if (!serverId.value) return
  localStorage.setItem('lvs_server', serverId.value)
  try {
    lvsData.value = await getLvsList(serverId.value)
  } catch (e) {
    ElMessage.error('加载数据失败')
  }
}

async function handleOp(vsIp, rsIp, state) {
  try {
    const res = await lvsOpPreview({ server_id: serverId.value, vs_ip: vsIp, rs_ip: rsIp, state })
    previewData.value = res
    previewId.value = res.preview_id
    currentAction.value = 'op'
    previewVisible.value = true
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '预览失败')
  }
}

async function handleSwap(row) {
  if (row.real_servers.length < 2) return
  const rs1 = row.real_servers[0]
  const rs2 = row.real_servers[1]
  try {
    const res = await lvsSwapPreview({ server_id: serverId.value, vs_ip: row.ip, rs_ip1: rs1.ip, rs_ip2: rs2.ip })
    previewData.value = res
    previewId.value = res.preview_id
    currentAction.value = 'swap'
    previewVisible.value = true
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '预览失败')
  }
}

async function executePreview() {
  executing.value = true
  try {
    const executeFn = currentAction.value === 'swap' ? lvsSwapExecute : lvsOpExecute
    const res = await executeFn({ preview_id: previewId.value })
    output.value = res.output
    ElMessage.success('执行成功')
    previewVisible.value = false
    await loadData()
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '执行失败')
    output.value = e.response?.data?.output || ''
  } finally {
    executing.value = false
  }
}
</script>
