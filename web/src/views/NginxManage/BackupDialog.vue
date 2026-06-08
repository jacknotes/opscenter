<template>
  <el-dialog v-model="visible" title="备份列表" width="min(700px, 90vw)" class="cool-dialog" align-center>
    <el-table v-loading="loading" v-force-reflow :data="backupList" size="small" max-height="400" class="backup-table">
      <el-table-column label="文件名" min-width="200">
        <template #default="{ row }">{{ row.name }}</template>
      </el-table-column>
      <el-table-column label="备份时间" width="180">
        <template #default="{ row }">{{ row.time }}</template>
      </el-table-column>
      <el-table-column label="操作" width="100">
        <template #default="{ row }">
          <el-button type="danger" size="small" @click="$emit('rollback', row.name)">回滚</el-button>
        </template>
      </el-table-column>
    </el-table>
  </el-dialog>
</template>

<script setup>
defineProps({
  backupList: { type: Array, default: () => [] },
  loading: { type: Boolean, default: false },
})

defineEmits(['rollback'])

const visible = defineModel({ type: Boolean, default: false })
</script>

<style scoped>
:deep(.backup-table .el-table__header th) {
  background: var(--bg-elevated) !important;
  font-weight: 600;
}
</style>
