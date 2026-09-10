<template>
  <div
    class="theme-toggle"
    :title="appStore.theme === 'dark' ? '切换到浅色模式' : '切换到深色模式'"
    @click="appStore.toggleTheme()"
  >
    <el-icon :size="size"><Sunny v-if="appStore.theme === 'dark'" /><Moon v-else /></el-icon>
  </div>
</template>

<script setup>
import { useAppStore } from '../stores/app'
import { Sunny, Moon } from '@element-plus/icons-vue'

defineProps({
  size: { type: Number, default: 18 },
})

const appStore = useAppStore()
</script>

<style scoped>
.theme-toggle {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border-radius: 8px;
  cursor: pointer;
  color: #94a3b8;
  transition:
    background 0.2s,
    color 0.2s;
}

.theme-toggle:hover {
  background: rgba(6, 182, 212, 0.1);
  color: #06b6d4;
}
</style>
