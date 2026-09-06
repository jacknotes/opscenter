<script setup lang="ts">
import { i18n } from '@/i18n'

const t = i18n.global.t

const visible = defineModel<boolean>('visible', { default: false })

withDefaults(
  defineProps<{
    title?: string
    output?: string | string[]
    status?: 'success' | 'failed'
    /** 元信息条：操作/对象/配置文件/时间（对齐 v1 输出元信息） */
    meta?: string
  }>(),
  {
    title: '',
    output: '',
    status: 'success',
    meta: '',
  },
)
</script>

<template>
  <el-dialog v-model="visible" :title="title || t('preview.result')" width="720px" append-to-body>
    <el-alert
      :type="status === 'success' ? 'success' : 'error'"
      :title="status === 'success' ? t('common.execSuccess') : t('common.execFailed')"
      :closable="false"
      class="result-alert"
    />
    <div v-if="meta" class="meta-line mono">{{ meta }}</div>
    <pre class="output-pre mono">{{ Array.isArray(output) ? output.join('\n') : output }}</pre>
  </el-dialog>
</template>

<style scoped>
.result-alert {
  margin-bottom: var(--space-4);
}

.meta-line {
  color: var(--text-muted);
  font-size: var(--text-xs);
  margin-bottom: var(--space-2);
  padding-bottom: var(--space-2);
  border-bottom: 1px dashed var(--border);
}

.output-pre {
  background: var(--bg-deep);
  border: 1px solid var(--border-faint);
  border-radius: var(--radius-sm);
  padding: var(--space-3);
  margin: 0;
  max-height: 480px;
  overflow: auto;
  font-size: var(--text-xs);
  line-height: 1.7;
  white-space: pre-wrap;
  word-break: break-all;
  color: var(--text-primary);
}
</style>
