<template>
  <el-dialog v-model="visible" title="变更预览" width="90%" top="5vh" class="cool-dialog">
    <div v-if="data">
      <div class="preview-desc">{{ data.description }}</div>
      <div class="diff-container">
        <div class="diff-header">
          <span class="diff-filename">{{ configFile }}</span>
        </div>
        <div class="diff-body">
          <div v-for="(line, index) in data.line_diffs" :key="index" :class="['diff-line', `diff-${line.type}`]">
            <span class="diff-line-num">{{ line.line_num }}</span>
            <span class="diff-line-prefix">{{ getLinePrefix(line.type) }}</span>
            <span class="diff-line-content">{{ line.content }}</span>
          </div>
        </div>
      </div>
    </div>
    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" :loading="executing" @click="$emit('execute')">确认执行</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
defineProps({
  data: { type: Object, default: null },
  configFile: { type: String, default: '' },
  executing: { type: Boolean, default: false },
})

defineEmits(['execute'])

const visible = defineModel({ type: Boolean, default: false })

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
.preview-desc {
  font-size: 14px;
  color: var(--text-primary);
  margin-bottom: 16px;
  padding: 10px 14px;
  background: var(--bg-elevated);
  border-radius: 8px;
  border-left: 3px solid #06b6d4;
}

.diff-container {
  border: 1px solid var(--border-default);
  border-radius: 8px;
  overflow: hidden;
  font-family: 'JetBrains Mono', 'Fira Code', 'Cascadia Code', 'Courier New', monospace;
  font-size: 13px;
}

.diff-header {
  background: var(--bg-elevated);
  padding: 10px 16px;
  border-bottom: 1px solid var(--border-default);
}

.diff-filename {
  color: var(--color-primary);
  font-weight: var(--weight-semibold);
  font-size: var(--font-md);
}

.diff-body {
  max-height: 500px;
  overflow-y: auto;
  background: var(--terminal-bg);
}

.diff-line {
  display: flex;
  padding: 1px 0;
  line-height: 1.6;
}

.diff-line:hover {
  background: rgba(255, 255, 255, 0.03);
}

.diff-line-num {
  width: 50px;
  text-align: right;
  padding-right: 12px;
  color: var(--text-placeholder);
  user-select: none;
  flex-shrink: 0;
  font-size: var(--font-sm);
}

.diff-line-prefix {
  width: 24px;
  text-align: center;
  user-select: none;
  flex-shrink: 0;
  font-weight: 700;
}

.diff-line-content {
  flex: 1;
  white-space: pre;
  overflow-x: auto;
  padding-right: 16px;
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

.diff-body::selection,
.diff-line-content::selection,
.diff-body *::selection {
  background: rgba(34, 211, 238, 0.5) !important;
  color: #fff !important;
}
</style>
