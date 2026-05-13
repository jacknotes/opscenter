<template>
  <div>
    <el-card>
      <template #header>
        <div style="display: flex; align-items: center; justify-content: space-between;">
          <span>操作日志</span>
          <div style="display: flex; gap: 10px;">
            <el-select v-model="module" placeholder="按模块筛选" clearable style="width: 150px" @change="loadData">
              <el-option label="LVS" value="lvs" />
              <el-option label="Nginx" value="nginx" />
              <el-option label="K8s" value="k8s" />
              <el-option label="预生产" value="preprod" />
            </el-select>
          </div>
        </div>
      </template>

      <el-table :data="logs" stripe border>
        <el-table-column prop="created_at" label="时间" width="180" />
        <el-table-column prop="username" label="操作人" width="100" />
        <el-table-column prop="module" label="模块" width="100">
          <template #default="{ row }">
            <el-tag>{{ row.module }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="action" label="动作" width="120" />
        <el-table-column prop="target" label="目标" min-width="200" show-overflow-tooltip />
        <el-table-column prop="status" label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="row.status === 'success' ? 'success' : 'danger'">{{ row.status }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column type="expand" label="详情" width="80">
          <template #default="{ row }">
            <div style="padding: 10px;">
              <p><strong>命令：</strong></p>
              <pre style="background: #f5f5f5; padding: 10px; border-radius: 4px;">{{ row.detail }}</pre>
              <p style="margin-top: 10px;"><strong>输出：</strong></p>
              <pre style="background: #1e1e1e; color: #d4d4d4; padding: 10px; border-radius: 4px; max-height: 300px; overflow-y: auto;">{{ row.output }}</pre>
            </div>
          </template>
        </el-table-column>
      </el-table>

      <el-pagination
        style="margin-top: 15px; justify-content: flex-end;"
        v-model:current-page="page"
        v-model:page-size="pageSize"
        :total="total"
        :page-sizes="[20, 50, 100]"
        layout="total, sizes, prev, pager, next"
        @size-change="loadData"
        @current-change="loadData"
      />
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { getLogs } from '../api'
import { ElMessage } from 'element-plus'

const logs = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const module = ref('')

onMounted(() => {
  loadData()
})

async function loadData() {
  try {
    const res = await getLogs({ page: page.value, size: pageSize.value, module: module.value })
    logs.value = res.data || []
    total.value = res.total || 0
  } catch (e) {
    ElMessage.error('加载日志失败')
  }
}
</script>
