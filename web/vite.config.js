import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
/// <reference types="vitest" />
import AutoImport from 'unplugin-auto-import/vite'
import Components from 'unplugin-vue-components/vite'
import { ElementPlusResolver } from 'unplugin-vue-components/resolvers'
import { resolve } from 'path'
import { appVersionPlugin } from './build/appVersion'

// Vite 开发服务器配置：监听 0.0.0.0:3000，/api 和 /ws 请求代理到 Go 后端 :18080
export default defineConfig({
  plugins: [
    // 版本号以虚拟模块 `virtual:app-version` 提供（dev/build 均生效）
    appVersionPlugin(),
    vue(),
    AutoImport({
      resolvers: [ElementPlusResolver()],
      imports: ['vue', 'vue-router', 'pinia'],
      // 项目以 JS 为主，生成 d.ts 会被 dev server 反复重写、污染 git 工作区，故关闭
      dts: false,
    }),
    Components({
      resolvers: [ElementPlusResolver()],
      dts: false,
    }),
  ],
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src'),
    },
  },
  server: {
    host: '0.0.0.0',
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:18080',
        changeOrigin: true,
        ws: true,
      },
    },
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks: {
          'vendor-vue': ['vue', 'vue-router', 'pinia'],
          'vendor-echarts': ['echarts', 'vue-echarts'],
        },
      },
    },
    chunkSizeWarningLimit: 600,
  },
  test: {
    environment: 'jsdom',
    globals: true,
  },
})
