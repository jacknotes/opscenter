<template>
  <ChangePreviewDialog
    v-model="visible"
    :description="data?.description"
    :executing="executing"
    @confirm="$emit('execute')"
  >
    <div v-if="data" class="diff-container">
      <div class="diff-header">
        <span class="diff-filename">{{ configFile }}</span>
      </div>
      <div ref="diffBodyRef" class="diff-body">
        <div v-for="(line, index) in data.line_diffs" :key="index" :class="['diff-line', `diff-${line.type}`]">
          <span class="diff-line-num">{{ line.line_num }}</span>
          <span class="diff-line-prefix">{{ getLinePrefix(line.type) }}</span>
          <span class="diff-line-content">{{ line.content }}</span>
        </div>
      </div>
    </div>
  </ChangePreviewDialog>
</template>

<script setup>
import { ref, watch, nextTick } from 'vue'
import ChangePreviewDialog from '../../components/common/ChangePreviewDialog.vue'

const props = defineProps({
  data: { type: Object, default: null },
  configFile: { type: String, default: '' },
  executing: { type: Boolean, default: false },
})

defineEmits(['execute'])

const visible = defineModel({ type: Boolean, default: false })
const diffBodyRef = ref(null)

// 打开时自动滚到第一处变更，避免用户手动在长配置里找
watch(visible, (val) => {
  if (val && props.data?.line_diffs?.length) {
    nextTick(() => {
      const el = diffBodyRef.value?.querySelector('.diff-added, .diff-removed')
      if (el) el.scrollIntoView({ block: 'center' })
    })
  }
})

function getLinePrefix(type) {
  switch (type) {
    case 'added':
      return '+'
    case 'removed':
      return '-'
    default:
      return ' '
  }
}
</script>

<style scoped>
.diff-container {
  border: 1px solid var(--border-default);
  border-radius: 8px;
  overflow: hidden;
  font-family: 'JetBrains Mono', 'Fira Code', 'Cascadia Code', 'Courier New', monospace;
  font-size: 12.5px;
}

.diff-header {
  background: var(--bg-elevated);
  padding: 8px 14px;
  border-bottom: 1px solid var(--border-default);
}

.diff-filename {
  color: var(--color-primary);
  font-weight: var(--weight-semibold);
  font-size: 13px;
}

.diff-body {
  max-height: 340px;
  overflow: auto;
  background: var(--terminal-bg);
}

.diff-line {
  display: flex;
  padding: 1px 0;
  line-height: 1.65;
  white-space: pre;
}

.diff-line:hover {
  background: rgba(255, 255, 255, 0.03);
}

.diff-line-num {
  width: 40px;
  text-align: right;
  padding-right: 10px;
  color: var(--text-placeholder);
  user-select: none;
  flex-shrink: 0;
  font-size: 11.5px;
}

.diff-line-prefix {
  width: 18px;
  text-align: center;
  user-select: none;
  flex-shrink: 0;
  font-weight: 700;
}

.diff-line-content {
  flex: 1;
  overflow-x: auto;
  padding-right: 14px;
}

.diff-same {
  background: transparent;
}
.diff-same .diff-line-content {
  color: var(--text-regular);
}
.diff-added {
  background: rgba(34, 197, 94, 0.08);
}
.diff-added .diff-line-prefix {
  color: #22c55e;
}
.diff-added .diff-line-content {
  color: #86efac;
}
.diff-removed {
  background: rgba(239, 68, 68, 0.08);
}
.diff-removed .diff-line-prefix {
  color: #ef4444;
}
.diff-removed .diff-line-content {
  color: #fca5a5;
}
</style>
