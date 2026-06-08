<template>
  <el-dialog v-model="visible" title="Keepalived 配置状态" width="min(900px, 95vw)" align-center>
    <div
      v-if="groups.length > 0"
      style="max-height: 600px; overflow-y: auto; display: flex; flex-wrap: wrap; gap: 16px"
    >
      <div
        v-for="group in groups"
        :key="group.vs_ip + ':' + group.vs_port"
        style="width: calc(50% - 8px); box-sizing: border-box"
      >
        <div
          style="
            font-weight: bold;
            font-size: 14px;
            margin-bottom: 8px;
            padding-bottom: 4px;
            border-bottom: 2px solid #06b6d4;
            color: var(--text-primary);
          "
        >
          {{ group.vs_ip }}:{{ group.vs_port }}
        </div>
        <el-table :data="group.real_servers" stripe size="small" border>
          <el-table-column prop="ip" label="Real Server IP" min-width="120" />
          <el-table-column prop="port" label="端口" width="65" align="center" />
          <el-table-column label="状态" width="70" align="center">
            <template #default="{ row }">
              <el-tag :type="row.status === 'up' ? 'success' : 'danger'" size="small">{{ row.status }}</el-tag>
            </template>
          </el-table-column>
        </el-table>
      </div>
    </div>
    <div v-else>
      <pre
        style="
          background: #1a1d2e;
          padding: 10px;
          border-radius: 4px;
          max-height: 500px;
          overflow-y: auto;
          font-size: 13px;
        "
        >{{ raw }}</pre
      >
    </div>
  </el-dialog>
</template>

<script setup>
defineProps({
  groups: { type: Array, default: () => [] },
  raw: { type: String, default: '' },
})

const visible = defineModel({ type: Boolean, default: false })
</script>

<style scoped>
@media (max-width: 768px) {
  :deep(.el-table) {
    overflow-x: auto;
    -webkit-overflow-scrolling: touch;
    touch-action: pan-x pinch-zoom;
  }
  :deep(.el-table .el-table__inner-wrapper) {
    min-width: 300px;
  }
}
</style>
