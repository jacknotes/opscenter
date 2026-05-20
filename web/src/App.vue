<template>
  <router-view v-slot="{ Component }">
    <transition name="page-fade">
      <div>
        <component :is="Component" :key="$route.fullPath" />
      </div>
    </transition>
  </router-view>
</template>

<script setup>
</script>

<style>
body {
  margin: 0;
  padding: 0;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
}
</style>
