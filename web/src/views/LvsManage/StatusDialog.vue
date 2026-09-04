<script setup lang="ts">
import { ref } from 'vue'
import { lvsApi, extractErrorMessage } from '@/api'
import type { LvsStatusGroup } from '@/api/types'
import { i18n } from '@/i18n'

const t = i18n.global.t

const visible = defineModel<boolean>('visible', { default: false })

const loading = ref(false)
const output = ref('')
const groups = ref<LvsStatusGroup[]>([])

async function open(serverId: number): Promise<void> {
  visible.value = true
  loading.value = true
  try {
    const res = await lvsApi.status(serverId)
    output.value = res.output
    groups.value = res.groups ?? []
  } catch (err) {
    extractErrorMessage(err)
  } finally {
    loading.value = false
  }
}

defineExpose({ open })
</script>

<template>
  <el-dialog v-model="visible" :title="t('lvs.statusDialog.title')" width="760px" append-to-body>
    <div v-loading="loading">
      <pre class="status-pre mono">{{ output || t('common.empty') }}</pre>

      <el-table v-if="groups.length" :data="groups" size="small" class="groups-table">
        <el-table-column prop="vs_ip" label="VS" min-width="130">
          <template #default="{ row }">
            <span class="mono">{{ row.vs_ip }}:{{ row.vs_port }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="t('lvs.rs')" min-width="260">
          <template #default="{ row }">
            <span
              v-for="rs in row.real_servers"
              :key="`${rs.ip}:${rs.port}`"
              class="rs-chip mono"
              :class="rs.status === 'up' ? 'up' : 'down'"
            >
              {{ rs.ip }}:{{ rs.port }}
            </span>
          </template>
        </el-table-column>
      </el-table>
    </div>
  </el-dialog>
</template>

<style scoped>
.status-pre {
  background: var(--bg-deep);
  border: 1px solid var(--border-faint);
  border-radius: var(--radius-sm);
  padding: var(--space-3);
  margin: 0 0 var(--space-3);
  max-height: 320px;
  overflow: auto;
  font-size: var(--text-xs);
  line-height: 1.7;
  white-space: pre-wrap;
  word-break: break-all;
}

.groups-table {
  margin-top: var(--space-2);
}

.rs-chip {
  display: inline-block;
  margin: 2px 6px 2px 0;
  padding: 0 8px;
  border-radius: var(--radius-pill);
  font-size: var(--text-xs);
  border: 1px solid var(--border);
}

.rs-chip.up {
  color: var(--emerald-400);
  border-color: rgba(52, 211, 153, 0.4);
  background: rgba(52, 211, 153, 0.08);
}

.rs-chip.down {
  color: var(--rose-400);
  border-color: rgba(248, 113, 113, 0.4);
  background: rgba(248, 113, 113, 0.08);
}
</style>
