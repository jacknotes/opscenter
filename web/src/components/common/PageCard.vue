<template>
  <el-card class="main-card">
    <template #header>
      <slot name="header"></slot>
    </template>
    <slot></slot>
  </el-card>
</template>
