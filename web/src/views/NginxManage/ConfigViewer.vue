<script setup lang="ts">
import { computed } from 'vue'
import { escapeHtml } from '@/utils/message'
import { i18n } from '@/i18n'

const t = i18n.global.t

const props = defineProps<{
  rawConfig: string
  configFile: string
}>()

const visible = defineModel<boolean>('visible', { default: false })

/** 轻量 nginx 配置语法高亮：注释 / 指令 / 花括号 / IP / 分号 */
const highlighted = computed(() => {
  if (!props.rawConfig) return ''
  return escapeHtml(props.rawConfig)
    .replace(/(#.*)$/gm, '<span class="hl-comment">$1</span>')
    .replace(/^(\s*)([\w_]+(?:\s+[\w_]+)*)(?=\s)/gm, (match, indent, directive) => {
      if (directive.startsWith('#') || directive.startsWith('server')) return match
      return `${indent}<span class="hl-directive">${directive}</span>`
    })
    .replace(/\{/g, '<span class="hl-brace">{</span>')
    .replace(/\}/g, '<span class="hl-brace">}</span>')
    .replace(/(\d+\.\d+\.\d+\.\d+(?::\d+)?)/g, '<span class="hl-ip">$1</span>')
    .replace(/(;)/g, '<span class="hl-semicolon">$1</span>')
})
</script>

<template>
  <el-dialog v-model="visible" :title="t('nginx.config') + ' - ' + configFile" width="80%" top="5vh" append-to-body>
    <pre class="config-pre mono" v-html="highlighted" />
  </el-dialog>
</template>

<style scoped>
.config-pre {
  background: var(--bg-deep);
  color: var(--text-primary);
  padding: var(--space-4);
  border-radius: var(--radius-sm);
  border: 1px solid var(--border-faint);
  max-height: 600px;
  overflow: auto;
  white-space: pre;
  font-size: var(--text-xs);
  line-height: 1.7;
  margin: 0;
}

:deep(.hl-comment) {
  color: #6a9955;
  font-style: italic;
}

:deep(.hl-directive) {
  color: #22d3ee;
  font-weight: 600;
}

:deep(.hl-brace) {
  color: #f59e0b;
  font-weight: 700;
}

:deep(.hl-ip) {
  color: #ce9178;
}

:deep(.hl-semicolon) {
  color: var(--text-secondary);
}
</style>
