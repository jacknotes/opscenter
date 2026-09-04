<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { i18n } from '@/i18n'
import DiffViewer from './DiffViewer.vue'
import StreamOutput from './StreamOutput.vue'
import type { LineDiff } from '@/api/types'

const t = i18n.global.t

const visible = defineModel<boolean>('visible', { default: false })

const props = withDefaults(
  defineProps<{
    /** 变更说明 */
    description?: string
    /** 脚本 list 原始输出（LVS/K8s/Preprod） */
    currentStatus?: string
    /** 将执行命令（LVS/Preprod 单条；K8s 数组） */
    commands?: string[]
    /** Nginx 预览的 before/after/diff */
    before?: string
    after?: string
    lineDiffs?: LineDiff[]
    /** 过期倒计时秒数（0 表示无预览） */
    countdown?: number
    expired?: boolean
    executing?: boolean
    /** preprod 走 WebSocket 流式执行 */
    streaming?: boolean
    /** streaming 模式下的 preview_id */
    streamPreviewId?: string
  }>(),
  {
    description: '',
    currentStatus: '',
    commands: () => [],
    before: '',
    after: '',
    lineDiffs: () => [],
    countdown: 0,
    expired: false,
    executing: false,
    streaming: false,
    streamPreviewId: '',
  },
)

const emit = defineEmits<{
  (e: 'execute'): void
  (e: 'repreview'): void
  (e: 'streamDone'): void
  (e: 'streamFailed', message: string): void
}>()

const confirmed = ref(false)

watch(visible, (v) => {
  if (!v) confirmed.value = false
})

const hasDiff = computed(() => props.lineDiffs.length > 0 || (props.before !== '' && props.after !== ''))
const isDiffMode = computed(() => props.lineDiffs.length > 0 || props.before !== '')

const countdownText = computed(() => {
  if (props.countdown <= 0) return ''
  const m = Math.floor(props.countdown / 60)
  const s = props.countdown % 60
  return t('preview.countdown', { seconds: `${m}:${String(s).padStart(2, '0')}` })
})
</script>

<template>
  <el-dialog
    v-model="visible"
    :title="t('preview.title')"
    width="820px"
    append-to-body
    :close-on-click-modal="!executing"
    :close-on-press-escape="!executing"
    :show-close="!executing"
  >
    <!-- 预览过期 -->
    <el-alert v-if="expired" type="warning" :title="t('preview.expired')" :closable="false" class="mb">
      <template #default>
        <el-button size="small" type="primary" @click="emit('repreview')">{{ t('common.refresh') }}</el-button>
      </template>
    </el-alert>

    <template v-else>
      <!-- 变更说明 + 倒计时 -->
      <div class="preview-head">
        <div class="desc">
          <span class="label">{{ t('preview.desc') }}</span>
          <span class="desc-text">{{ description }}</span>
        </div>
        <span v-if="countdownText" class="mono countdown" :class="{ urgent: countdown < 60 }">{{ countdownText }}</span>
      </div>

      <!-- 将执行命令 -->
      <div v-if="commands.length" class="section">
        <div class="section-title">{{ t('preview.commands') }}</div>
        <pre class="cmd-pre mono">{{ commands.join('\n') }}</pre>
      </div>

      <!-- 当前状态（脚本输出） -->
      <div v-if="currentStatus" class="section">
        <div class="section-title">{{ t('preview.currentStatus') }}</div>
        <pre class="status-pre mono">{{ currentStatus }}</pre>
      </div>

      <!-- Nginx diff -->
      <div v-if="hasDiff && isDiffMode" class="section">
        <div class="section-title">{{ t('preview.diff') }}</div>
        <DiffViewer v-if="lineDiffs.length" :diffs="lineDiffs" />
        <div v-else class="diff-raw">
          <div>
            <div class="diff-side-title">{{ t('preview.before') }}</div>
            <pre class="status-pre mono">{{ before }}</pre>
          </div>
          <div>
            <div class="diff-side-title">{{ t('preview.after') }}</div>
            <pre class="status-pre mono">{{ after }}</pre>
          </div>
        </div>
      </div>

      <!-- 流式执行输出 -->
      <div v-if="streaming && streamPreviewId" class="section">
        <StreamOutput
          :preview-id="streamPreviewId"
          @done="emit('streamDone')"
          @failed="(m) => emit('streamFailed', m)"
        />
      </div>

      <!-- 确认执行（非流式；流式模式下启动 WS 执行前仍需确认） -->
      <div v-if="!streaming || !streamPreviewId" class="confirm-area">
        <el-checkbox v-model="confirmed">{{ t('preview.confirmExec') }}</el-checkbox>
        <el-button
          type="primary"
          :disabled="!confirmed"
          :loading="executing"
          @click="emit('execute')"
        >
          {{ executing ? t('preview.executing') : t('common.execute') }}
        </el-button>
      </div>
    </template>
  </el-dialog>
</template>

<style scoped>
.mb {
  margin-bottom: var(--space-4);
}

.preview-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--space-4);
  padding: var(--space-3) var(--space-4);
  background: var(--el-color-primary-light-9);
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  margin-bottom: var(--space-4);
}

.desc {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
}

.label {
  color: var(--text-muted);
  font-size: var(--text-xs);
}

.desc-text {
  color: var(--text-primary);
  font-weight: 600;
}

.countdown {
  color: var(--text-secondary);
  font-size: var(--text-xs);
  flex-shrink: 0;
}

.countdown.urgent {
  color: var(--amber-400);
}

.section {
  margin-bottom: var(--space-4);
}

.section-title {
  color: var(--text-secondary);
  font-size: var(--text-sm);
  margin-bottom: var(--space-2);
}

.cmd-pre,
.status-pre {
  background: var(--bg-deep);
  border: 1px solid var(--border-faint);
  border-radius: var(--radius-sm);
  padding: var(--space-2) var(--space-3);
  margin: 0;
  max-height: 220px;
  overflow: auto;
  font-size: var(--text-xs);
  line-height: 1.7;
  white-space: pre-wrap;
  word-break: break-all;
  color: var(--text-primary);
}

.cmd-pre {
  color: var(--emerald-400);
}

.diff-raw {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: var(--space-3);
}

@media (max-width: 700px) {
  .diff-raw {
    grid-template-columns: 1fr;
  }
}

.diff-side-title {
  color: var(--text-muted);
  font-size: var(--text-xs);
  margin-bottom: var(--space-1);
}

.confirm-area {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-4);
  padding-top: var(--space-3);
  border-top: 1px solid var(--border-faint);
}
</style>
