<template>
  <div class="empty-state">
    <el-icon class="empty-state-icon" :size="48">
      <component :is="icon" />
    </el-icon>
    <p class="empty-state-text">{{ text }}</p>
    <slot></slot>
  </div>
</template>

<script setup>
defineProps({
  icon: { type: [Object, String], default: 'Document' },
  text: { type: String, default: '暂无数据' },
})
</script>
