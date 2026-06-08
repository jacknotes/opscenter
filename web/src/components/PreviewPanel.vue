<template>
  <Transition name="panel-slide">
    <div v-if="modelValue" class="preview-panel-overlay" @click.self="close">
      <div class="preview-panel" :style="{ width: panelWidth }">
        <!-- Header -->
        <div class="preview-panel-header">
          <div class="preview-panel-title">
            <el-icon class="preview-panel-icon"><View /></el-icon>
            <span>{{ title }}</span>
          </div>
          <el-icon class="preview-panel-close" @click="close"><Close /></el-icon>
        </div>

        <!-- Body -->
        <div class="preview-panel-body">
          <slot></slot>
        </div>

        <!-- Footer -->
        <div class="preview-panel-footer">
          <slot name="footer">
            <el-button @click="close">取消</el-button>
            <el-button type="primary" :loading="loading" @click="$emit('confirm')">确认执行</el-button>
          </slot>
        </div>
      </div>
    </div>
  </Transition>
</template>

<script setup>
import { onMounted, onUnmounted } from 'vue'
import { View, Close } from '@element-plus/icons-vue'

const props = defineProps({
  modelValue: { type: Boolean, default: false },
  title: { type: String, default: '变更预览' },
  loading: { type: Boolean, default: false },
  width: { type: String, default: '560px' },
})

const emit = defineEmits(['update:modelValue', 'confirm'])

const panelWidth = props.width

function close() {
  emit('update:modelValue', false)
}

function handleEsc(e) {
  if (e.key === 'Escape' && props.modelValue) {
    close()
  }
}

onMounted(() => document.addEventListener('keydown', handleEsc))
onUnmounted(() => document.removeEventListener('keydown', handleEsc))
</script>

<style scoped>
.preview-panel-overlay {
  position: fixed;
  inset: 0;
  z-index: 2000;
  background: rgba(0, 0, 0, 0.3);
  display: flex;
  justify-content: flex-end;
}

.preview-panel {
  height: 100vh;
  height: 100dvh;
  max-width: 100vw;
  background: var(--bg-card);
  display: flex;
  flex-direction: column;
  box-shadow: -8px 0 30px rgba(0, 0, 0, 0.3);
  border-left: 1px solid var(--border-default);
}

.preview-panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px;
  border-bottom: 1px solid var(--border-default);
  background: var(--bg-elevated);
  flex-shrink: 0;
}

.preview-panel-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 16px;
  font-weight: 600;
  color: var(--text-primary);
}

.preview-panel-icon {
  color: var(--color-primary);
}

.preview-panel-close {
  cursor: pointer;
  font-size: 18px;
  color: var(--text-secondary);
  transition: color 0.2s;
  border-radius: 6px;
  padding: 4px;
}

.preview-panel-close:hover {
  color: var(--text-primary);
  background: var(--color-primary-bg);
}

.preview-panel-body {
  flex: 1;
  overflow-y: auto;
  -webkit-overflow-scrolling: touch;
  padding: 20px;
}

.preview-panel-footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  padding: 12px 20px;
  border-top: 1px solid var(--border-default);
  background: var(--bg-elevated);
  flex-shrink: 0;
}

/* Slide transition */
.panel-slide-enter-active,
.panel-slide-leave-active {
  transition: opacity 0.25s ease;
}

.panel-slide-enter-active .preview-panel,
.panel-slide-leave-active .preview-panel {
  transition: transform 0.25s cubic-bezier(0.4, 0, 0.2, 1);
}

.panel-slide-enter-from {
  opacity: 0;
}
.panel-slide-enter-from .preview-panel {
  transform: translateX(100%);
}

.panel-slide-leave-to {
  opacity: 0;
}
.panel-slide-leave-to .preview-panel {
  transform: translateX(100%);
}
</style>
