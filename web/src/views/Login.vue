<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import { extractErrorMessage } from '@/api/client'
import { useTheme } from '@/composables/useTheme'
import { i18n } from '@/i18n'

const t = i18n.global.t

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const { theme, toggle: toggleTheme } = useTheme()

const formRef = ref<FormInstance>()
const loading = ref(false)
const errorBox = ref('')

const form = reactive({
  username: '',
  password: '',
})

// 校验规则用 computed 包裹：文案集中走 i18n
const rules = computed<FormRules>(() => ({
  username: [{ required: true, message: t('login.usernamePlaceholder'), trigger: 'blur' }],
  password: [{ required: true, message: t('login.passwordPlaceholder'), trigger: 'blur' }],
}))

onMounted(() => {
  // 已登录直接进首页
  if (auth.isLoggedIn) {
    void router.replace('/dashboard')
  }
})

async function handleSubmit(): Promise<void> {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return

  loading.value = true
  errorBox.value = ''
  try {
    await auth.login(form.username.trim(), form.password)
    ElMessage.success(`${t('common.welcome')}，${auth.displayName}`)
    const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : ''
    await router.replace(redirect ? decodeURIComponent(redirect) : '/dashboard')
  } catch (err) {
    errorBox.value = `${t('login.error')}：${extractErrorMessage(err, t('login.error'))}`
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login-view">
    <!-- 右上角固定：主题切换 -->
    <div class="login-corner">
      <el-button circle text @click="toggleTheme" :title="t('common.theme')">
        <el-icon v-if="theme === 'dark'"><Sunny /></el-icon>
        <el-icon v-else><Moon /></el-icon>
      </el-button>
    </div>

    <div class="login-glow" aria-hidden="true" />

    <div class="login-card card reveal d-0">
      <div class="login-brand">
        <span class="brand-mark">✦</span>
        <h1 class="grad-text">OpsCenter</h1>
        <p class="mono tagline">{{ t('app.tagline') }}</p>
      </div>

      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        label-position="top"
        size="large"
        @keyup.enter="handleSubmit"
      >
        <el-form-item :label="t('login.username')" prop="username">
          <el-input v-model="form.username" :placeholder="t('login.usernamePlaceholder')" autocomplete="username">
            <template #prefix><el-icon><User /></el-icon></template>
          </el-input>
        </el-form-item>
        <el-form-item :label="t('login.password')" prop="password">
          <el-input
            v-model="form.password"
            type="password"
            show-password
            :placeholder="t('login.passwordPlaceholder')"
            autocomplete="current-password"
          >
            <template #prefix><el-icon><Lock /></el-icon></template>
          </el-input>
        </el-form-item>

        <div v-if="errorBox" class="error-box">{{ errorBox }}</div>

        <el-button type="primary" class="login-submit" :loading="loading" @click="handleSubmit">
          {{ t('login.submit') }}
        </el-button>
      </el-form>
    </div>

    <footer class="mono login-footer">{{ t('login.footer') }}</footer>
  </div>
</template>

<style scoped>
.login-view {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;
  padding: var(--space-6);
}

.login-corner {
  position: fixed;
  top: var(--space-5);
  right: var(--space-5);
  z-index: 10;
}

.login-glow {
  position: fixed;
  inset: 0;
  z-index: -1;
  background:
    radial-gradient(640px 320px at 50% 36%, rgba(99, 102, 241, 0.18), transparent 68%),
    radial-gradient(420px 240px at 50% 64%, rgba(139, 92, 246, 0.12), transparent 70%);
  pointer-events: none;
}

.login-card {
  width: min(400px, 92vw);
  padding: var(--space-8) var(--space-7);
}

.login-brand {
  text-align: center;
  margin-bottom: var(--space-6);
}

.brand-mark {
  font-size: var(--text-2xl);
  color: var(--indigo-400);
  text-shadow: 0 0 18px rgba(99, 102, 241, 0.6);
}

.login-brand h1 {
  font-family: var(--font-display);
  font-size: var(--text-3xl);
  margin: var(--space-2) 0 var(--space-1);
  letter-spacing: 0.03em;
}

.tagline {
  color: var(--text-muted);
  font-size: var(--text-xs);
  letter-spacing: 0.22em;
  margin: 0;
  text-transform: uppercase;
}

.error-box {
  background: rgba(248, 113, 113, 0.1);
  border: 1px solid rgba(248, 113, 113, 0.35);
  color: var(--rose-400);
  border-radius: var(--radius-sm);
  padding: var(--space-2) var(--space-3);
  font-size: var(--text-sm);
  margin-bottom: var(--space-4);
}

.login-submit {
  width: 100%;
  margin-top: var(--space-2);
}

.login-footer {
  position: fixed;
  bottom: var(--space-5);
  left: 0;
  right: 0;
  text-align: center;
  color: var(--text-faint);
  font-size: var(--text-xs);
  letter-spacing: 0.18em;
}
</style>
