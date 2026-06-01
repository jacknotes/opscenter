<template>
  <el-card style="margin-top: 20px;">
    <template #header>
      <div style="display: flex; justify-content: space-between; align-items: center;">
        <span>执行结果</span>
        <div style="display: flex; align-items: center; gap: 10px;">
          <el-tag v-if="status === 'streaming'" type="warning" size="small">执行中...</el-tag>
          <el-tag v-else-if="status === 'done'" type="success" size="small">执行完成</el-tag>
          <el-tag v-else-if="status === 'error'" type="danger" size="small">执行失败</el-tag>
          <el-tag v-else-if="status === 'connecting'" type="info" size="small">连接中...</el-tag>
          <el-button v-if="showCancel && status === 'streaming'" type="danger" size="small" @click="$emit('cancel')">取消</el-button>
        </div>
      </div>
    </template>
    <div ref="container" class="stream-output" @scroll="onScroll">
      <div v-for="(line, idx) in lines" :key="idx" :class="['output-line', line.stream === 'stderr' ? 'stderr' : 'stdout']">
        {{ line.text }}
      </div>
      <div v-if="status === 'streaming'" class="cursor">_</div>
    </div>
  </el-card>
</template>

<script setup>
import { ref, watch, nextTick } from 'vue'

const props = defineProps({
  lines: { type: Array, default: () => [] },
  status: { type: String, default: 'idle' },
  showCancel: { type: Boolean, default: true },
})

defineEmits(['cancel'])

const container = ref(null)
const userScrolled = ref(false)

function onScroll() {
  const el = container.value
  if (!el) return
  const atBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 30
  userScrolled.value = !atBottom
}

watch(
  () => props.lines.length,
  async () => {
    if (userScrolled.value) return
    await nextTick()
    const el = container.value
    if (el) {
      el.scrollTop = el.scrollHeight
    }
  }
)
</script>

<style scoped>
.stream-output {
  background: #0B0D13;
  color: #22D3EE;
  padding: 15px;
  border-radius: 8px;
  border: 1px solid rgba(255, 255, 255, 0.06);
  max-height: 500px;
  overflow-y: auto;
  font-family: 'Courier New', Consolas, monospace;
  font-size: 13px;
  line-height: 1.6;
}

.output-line {
  white-space: pre-wrap;
  word-break: break-all;
}

.output-line.stderr {
  color: #FB7185;
}

.output-line.stdout {
  color: #22D3EE;
}

.cursor {
  display: inline-block;
  animation: blink 1s step-end infinite;
  color: #22D3EE;
}

@keyframes blink {
  50% { opacity: 0; }
}
</style>
