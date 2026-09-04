<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { nginxApi, extractErrorMessage } from '@/api'
import { i18n } from '@/i18n'

const t = i18n.global.t

const visible = defineModel<boolean>('visible', { default: false })

const props = defineProps<{
  serverId: number
  configFile: string
}>()

const emit = defineEmits<{
  (e: 'confirm', backupFile: string): void
}>()

const backups = ref<string[]>([])
const loading = ref(false)

async function load(): Promise<void> {
  loading.value = true
  try {
    backups.value = await nginxApi.backups(props.serverId)
  } catch (err) {
    ElMessage.error(extractErrorMessage(err))
  } finally {
    loading.value = false
  }
}

function open(): void {
  visible.value = true
  void load()
}

onMounted(() => {
  if (visible.value) void load()
})

defineExpose({ open })
</script>

<template>
  <el-dialog v-model="visible" :title="t('nginx.backups')" width="620px" append-to-body>
    <div v-loading="loading">
      <el-empty v-if="!loading && backups.length === 0" :description="t('nginx.noBackup')" />
      <el-table v-else :data="backups.map((f) => ({ file: f }))" size="small" max-height="380">
        <el-table-column :label="t('nginx.configFile')" min-width="300">
          <template #default="{ row }">
            <span class="mono">{{ row.file }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="t('common.operation')" width="100" fixed="right">
          <template #default="{ row }">
            <el-button
              link
              type="warning"
              size="small"
              @click="emit('confirm', row.file as string); visible = false"
            >
              {{ t('nginx.rollback') }}
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>
  </el-dialog>
</template>
