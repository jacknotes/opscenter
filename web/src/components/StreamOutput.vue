<template>
  <div>
    <div class="stream-header">
      <div class="stream-status">
        <el-tag v-if="status === 'streaming'" type="warning" size="small">执行中...</el-tag>
        <el-tag v-else-if="status === 'done'" type="success" size="small">执行完成</el-tag>
        <el-tag v-else-if="status === 'error'" type="danger" size="small">执行失败</el-tag>
        <el-tag v-else-if="status === 'connecting'" type="info" size="small">连接中...</el-tag>
      </div>
      <el-button v-if="showCancel && status === 'streaming'" type="danger" size="small" @click="$emit('cancel')"
        >取消</el-button
      >
    </div>
    <div ref="container" class="stream-output" @scroll="onScroll">
      <div
        v-for="line in visibleLines"
        :key="line.id"
        :class="['output-line', line.stream === 'stderr' ? 'stderr' : 'stdout']"
      >
        {{ line.text }}
      </div>
      <div v-if="status === 'streaming'" class="cursor">_</div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, nextTick, onMounted } from 'vue'

const MAX_RENDERED_LINES = 300

const props = defineProps({
  lines: { type: Array, default: () => [] },
  status: { type: String, default: 'idle' },
  showCancel: { type: Boolean, default: true },
})

defineEmits(['cancel'])

// 只渲染最后 N 行，减少 DOM 节点数量
const visibleLines = computed(() => {
  const arr = props.lines
  return arr.length > MAX_RENDERED_LINES ? arr.slice(-MAX_RENDERED_LINES) : arr
})

const container = ref(null)
const userScrolled = ref(false)
let scrollTimer = null

function scrollToBottom() {
  const el = container.value
  if (el) {
    el.scrollTop = el.scrollHeight
  }
}

function onScroll() {
  const el = container.value
  if (!el) return
  const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 30
  userScrolled.value = !atBottom
}

// 组件挂载时（包括页面切换回来），滚动到底部显示最新内容
onMounted(async () => {
  await nextTick()
  scrollToBottom()
})

// 防抖滚动：高频消息到来时合并多次滚动为一次
watch(
  () => props.lines.length,
  () => {
    if (userScrolled.value) return
    if (scrollTimer) return
    scrollTimer = requestAnimationFrame(() => {
      scrollTimer = null
      scrollToBottom()
    })
  }
)
</script>

<style scoped>
.stream-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}

.stream-output {
  background: var(--terminal-bg, #0b0d13);
  color: var(--terminal-text, #22d3ee);
  padding: 15px;
  border-radius: 8px;
  border: 1px solid var(--border-default, rgba(255, 255, 255, 0.06));
  max-height: 500px;
  overflow-y: auto;
  font-family: 'JetBrains Mono', 'Fira Code', 'Cascadia Code', 'Courier New', monospace;
  font-size: var(--font-base, 13px);
  line-height: 1.6;
  /* 优化渲染性能（不使用 strict，避免 contain: size 影响 scrollHeight 计算） */
  contain: layout style;
}

.stream-output::selection,
.stream-output *::selection {
  background: rgba(34, 211, 238, 0.5) !important;
  color: #fff !important;
}

.output-line {
  white-space: pre-wrap;
  word-break: break-all;
}

.output-line.stderr {
  color: var(--color-danger, #fb7185);
}

.output-line.stdout {
  color: var(--terminal-text, #22d3ee);
}

.cursor {
  display: inline-block;
  animation: blink 1s step-end infinite;
  color: var(--terminal-text, #22d3ee);
}

@keyframes blink {
  50% {
    opacity: 0;
  }
}

@media (prefers-reduced-motion: reduce) {
  .cursor {
    animation: none;
  }
}
</style>
