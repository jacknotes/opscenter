<template>
  <el-dialog v-model="visible" :title="'配置文件 - ' + configFile" width="80%" top="5vh" class="cool-dialog">
    <pre class="terminal-pre terminal-lg" v-html="highlighted"></pre>
  </el-dialog>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  rawConfig: { type: String, default: '' },
  configFile: { type: String, default: '' },
})

const visible = defineModel({ type: Boolean, default: false })

const highlighted = computed(() => {
  if (!props.rawConfig) return ''
  const escapeHtml = (s) => s.replace(/[&<>]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;' })[c])
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

<style scoped>
.terminal-pre {
  background: var(--terminal-bg);
  color: var(--terminal-text);
  padding: 16px;
  border-radius: 8px;
  max-height: 400px;
  overflow-y: auto;
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
  white-space: pre;
  font-family: 'JetBrains Mono', 'Fira Code', 'Cascadia Code', 'Courier New', monospace;
  font-size: var(--font-base);
  line-height: 1.6;
  margin: 0;
  border: 1px solid var(--border-default);
}

.terminal-pre::selection,
.terminal-pre *::selection {
  background: rgba(34, 211, 238, 0.5) !important;
  color: #fff !important;
}

.terminal-lg {
  max-height: 600px;
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
  color: var(--text-placeholder);
}
</style>
