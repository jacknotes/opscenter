<template>
  <div class="login-container">
    <div class="login-card">
      <div class="login-brand">
        <div class="login-logo">OC</div>
        <h1 class="login-title">OpsCenter</h1>
        <p class="login-subtitle">运维发布管理平台</p>
      </div>
      <el-form :model="form" class="login-form" @submit.prevent="handleLogin">
        <el-form-item>
          <el-input v-model="form.username" placeholder="用户名" size="large">
            <template #prefix
              ><el-icon><User /></el-icon
            ></template>
          </el-input>
        </el-form-item>
        <el-form-item>
          <el-input v-model="form.password" type="password" placeholder="密码" show-password size="large">
            <template #prefix
              ><el-icon><Lock /></el-icon
            ></template>
          </el-input>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" style="width: 100%" :loading="loading" native-type="submit" size="large"
            >登 录</el-button
          >
        </el-form-item>
      </el-form>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useUserStore } from '../stores/user'
import { login } from '../api'
import { ElMessage } from 'element-plus'
import { User, Lock } from '@element-plus/icons-vue'

const router = useRouter()
const userStore = useUserStore()
const loading = ref(false)
const form = ref({ username: '', password: '' })

async function handleLogin() {
  if (!form.value.username || !form.value.password) {
    ElMessage.warning('请输入用户名和密码')
    return
  }

  loading.value = true
  try {
    const res = await login(form.value)
    userStore.setToken(res.token)
    userStore.setUserInfo(res.user)
    ElMessage.success('登录成功')
    router.push('/dashboard')
  } catch (e) {
    ElMessage.error(e.response?.data?.error || '登录失败')
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-container {
  height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--bg-base);
  position: relative;
  overflow: hidden;
}

/* 背景装饰 — 缓慢呼吸动画 */
.login-container::before {
  content: '';
  position: absolute;
  width: 600px;
  height: 600px;
  border-radius: 50%;
  background: radial-gradient(circle, rgba(6, 182, 212, 0.1) 0%, transparent 70%);
  top: -200px;
  right: -200px;
  pointer-events: none;
  animation: login-blob-1 12s ease-in-out infinite;
}
.login-container::after {
  content: '';
  position: absolute;
  width: 500px;
  height: 500px;
  border-radius: 50%;
  background: radial-gradient(circle, rgba(139, 92, 246, 0.06) 0%, transparent 70%);
  bottom: -150px;
  left: -100px;
  pointer-events: none;
}

.login-card {
  width: 400px;
  max-width: 90vw;
  background: var(--card-bg);
  -webkit-backdrop-filter: blur(20px);
  backdrop-filter: blur(20px);
  border-radius: 16px;
  padding: 40px;
  border: var(--card-border);
  box-shadow:
    var(--card-shadow),
    0 25px 60px rgba(0, 0, 0, 0.15);
  transition:
    transform 0.3s ease,
    box-shadow 0.3s ease;
  position: relative;
  z-index: 1;
}
.login-card:hover {
  transform: translateY(-2px);
  box-shadow:
    var(--card-shadow),
    0 30px 70px rgba(0, 0, 0, 0.2);
}

.login-brand {
  text-align: center;
  margin-bottom: 32px;
}

.login-logo {
  width: 64px;
  height: 64px;
  background: linear-gradient(135deg, #06b6d4 0%, #0891b2 100%);
  border-radius: 16px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  font-size: var(--font-2xl);
  font-weight: var(--weight-bold);
  margin-bottom: 16px;
  box-shadow: 0 8px 24px rgba(6, 182, 212, 0.3);
}

.login-title {
  font-size: var(--font-2xl);
  font-weight: var(--weight-bold);
  color: var(--text-primary);
  margin: 0 0 8px 0;
}

.login-subtitle {
  font-size: var(--font-md);
  color: var(--text-secondary);
  margin: 0;
}

.login-form {
  margin-top: 24px;
}

.login-form :deep(.el-button--primary) {
  border-radius: 8px;
  height: 44px;
  font-size: var(--font-lg);
  font-weight: var(--weight-semibold);
  letter-spacing: 4px;
}

/* 移动端适配 */
@media (max-width: 480px) {
  .login-card {
    padding: 24px 20px;
  }
  .login-logo {
    width: 48px;
    height: 48px;
    font-size: var(--font-lg);
  }
  .login-title {
    font-size: var(--font-xl);
  }
  .login-form :deep(.el-button--primary) {
    height: 40px;
    font-size: var(--font-md);
    letter-spacing: 2px;
  }
}

/* 背景光晕呼吸动画 */
@keyframes login-blob-1 {
  0%,
  100% {
    transform: translate(0, 0) scale(1);
    opacity: 0.8;
  }
  33% {
    transform: translate(-30px, 20px) scale(1.1);
    opacity: 1;
  }
  66% {
    transform: translate(20px, -15px) scale(0.95);
    opacity: 0.6;
  }
}
</style>
