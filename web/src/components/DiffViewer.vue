<script setup lang="ts">
import { i18n } from '@/i18n'
import type { LineDiff } from '@/api/types'

const t = i18n.global.t

defineProps<{
  diffs: LineDiff[]
}>()
</script>

<template>
  <div class="diff-viewer mono">
    <div
      v-for="(line, idx) in diffs"
      :key="idx"
      class="diff-line"
      :class="`type-${line.type}`"
    >
      <span class="line-num">{{ line.line_num }}</span>
      <span class="line-type">{{ line.type === 'added' ? '+' : line.type === 'removed' ? '-' : ' ' }}</span>
      <span class="line-content">{{ line.content }}</span>
    </div>
  </div>
</template>

<style scoped>
.diff-viewer {
  background: var(--bg-deep);
  border: 1px solid var(--border-faint);
  border-radius: var(--radius-sm);
  padding: var(--space-2) 0;
  max-height: 420px;
  overflow: auto;
  font-size: var(--text-xs);
  line-height: 1.8;
}

.diff-line {
  display: flex;
  gap: var(--space-2);
  padding: 0 var(--space-3);
  white-space: pre;
}

.line-num {
  color: var(--text-faint);
  min-width: 3.5em;
  text-align: right;
  user-select: none;
  flex-shrink: 0;
}

.line-type {
  user-select: none;
  font-weight: 700;
  flex-shrink: 0;
}

.line-content {
  overflow-x: auto;
}

.type-added {
  background: rgba(52, 211, 153, 0.1);
  color: var(--emerald-400);
}

.type-removed {
  background: rgba(248, 113, 113, 0.1);
  color: var(--rose-400);
}

.type-same {
  color: var(--text-secondary);
}
</style>
