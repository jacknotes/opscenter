<script setup lang="ts">
import { ref } from 'vue'
import type { NginxBatchItem } from '@/api/types'
import { i18n } from '@/i18n'

const t = i18n.global.t

const visible = defineModel<boolean>('visible', { default: false })

const props = defineProps<{
  upstreamNames: string[]
}>()

const emit = defineEmits<{
  (e: 'confirm', items: NginxBatchItem[]): void
}>()

interface Row extends NginxBatchItem {}

const rows = ref<Row[]>([])

function open(): void {
  rows.value = [{ upstream_name: props.upstreamNames[0] ?? '', action: 'offline', backend_ip: '' }]
  visible.value = true
}

function addRow(): void {
  rows.value.push({ upstream_name: props.upstreamNames[0] ?? '', action: 'offline', backend_ip: '' })
}

function removeRow(idx: number): void {
  rows.value.splice(idx, 1)
}

function confirm(): void {
  const items = rows.value.filter((r) => r.upstream_name)
  if (items.length === 0) return
  emit('confirm', items)
  visible.value = false
}

defineExpose({ open })
</script>

<template>
  <el-dialog v-model="visible" :title="t('nginx.batch')" width="680px" append-to-body>
    <el-table :data="rows" size="small">
      <el-table-column :label="t('nginx.upstream')" min-width="170">
        <template #default="{ row }">
          <el-select v-model="row.upstream_name" filterable size="small">
            <el-option v-for="n in upstreamNames" :key="n" :value="n" :label="n" />
          </el-select>
        </template>
      </el-table-column>
      <el-table-column :label="t('nginx.batchItem.action')" width="130">
        <template #default="{ row }">
          <el-select v-model="row.action" size="small">
            <el-option value="online" :label="t('nginx.online')" />
            <el-option value="offline" :label="t('nginx.offline')" />
            <el-option value="toggle" :label="t('nginx.toggle')" />
          </el-select>
        </template>
      </el-table-column>
      <el-table-column :label="t('nginx.batchItem.ip')" min-width="160">
        <template #default="{ row }">
          <el-input
            v-model="row.backend_ip"
            size="small"
            :disabled="row.action === 'toggle'"
            :placeholder="row.action === 'toggle' ? '-' : '10.0.0.1'"
          />
        </template>
      </el-table-column>
      <el-table-column width="60">
        <template #default="{ $index }">
          <el-button link type="danger" size="small" @click="removeRow($index)">
            {{ t('common.delete') }}
          </el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-button class="add-btn" size="small" @click="addRow">{{ t('nginx.batchItem.addAction') }}</el-button>

    <template #footer>
      <el-button @click="visible = false">{{ t('common.cancel') }}</el-button>
      <el-button type="primary" :disabled="rows.length === 0" @click="confirm">
        {{ t('common.preview') }}
      </el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.add-btn {
  margin-top: var(--space-3);
}
</style>
