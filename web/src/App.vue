<template>
  <el-config-provider :locale="zhCn">
    <!-- Login 页面不做缓存和过渡动画，避免与 keep-alive + transition 冲突 -->
    <router-view v-if="$route.path === '/login'" />
    <!-- 其他页面使用 keep-alive + 过渡动画 -->
    <router-view v-else v-slot="{ Component }">
      <transition name="page-slide" mode="out-in">
        <keep-alive>
          <component :is="Component" />
        </keep-alive>
      </transition>
    </router-view>
  </el-config-provider>
</template>

<script setup>
import zhCn from 'element-plus/dist/locale/zh-cn.mjs'
</script>

<style>
body {
  margin: 0;
  padding: 0;
  font-family:
    'DM Sans',
    'Noto Sans SC',
    -apple-system,
    BlinkMacSystemFont,
    'Segoe UI',
    Roboto,
    'Helvetica Neue',
    Arial,
    sans-serif;
}

/* 页面切换动效（不使用 transform，避免破坏 fixed 定位） */
.page-slide-enter-active,
.page-slide-leave-active {
  transition: opacity 0.15s ease;
}
.page-slide-enter-from,
.page-slide-leave-to {
  opacity: 0;
}

/* 尊重用户减少动效偏好 */
@media (prefers-reduced-motion: reduce) {
  .page-slide-enter-active,
  .page-slide-leave-active {
    transition: none;
  }
}
</style>
