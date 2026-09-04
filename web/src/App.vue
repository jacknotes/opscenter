<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useTheme } from '@/composables/useTheme'
import { i18n } from '@/i18n'

// el-config-provider 同步 Element Plus 组件语言
const elementLocale = computed(() => {
  // 目前仅中文；扩展多语言时按 i18n.currentLocale 切换
  void i18n
  return undefined
})

useTheme()
</script>

<template>
  <el-config-provider :locale="elementLocale">
    <router-view />
  </el-config-provider>
</template>
