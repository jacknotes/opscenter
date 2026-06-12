<template>
  <div class="dash-row stat-row">
    <div class="number-card number-card--cyan" style="cursor: pointer" @click="emit('show-online-users')">
      <div class="number-info">
        <div class="number-label">在线用户数</div>
        <div class="number-value">{{ animatedOnlineUsers }}</div>
      </div>
      <el-icon class="number-deco" :size="48"><User /></el-icon>
    </div>
    <div class="number-card number-card--green">
      <div class="number-info">
        <div class="number-label">今日登录成功</div>
        <div class="number-value">{{ animatedLoginSuccess }}</div>
      </div>
      <el-icon class="number-deco" :size="48"><CircleCheck /></el-icon>
    </div>
    <div class="number-card number-card--red">
      <div class="number-info">
        <div class="number-label">今日登录失败</div>
        <div class="number-value">{{ animatedLoginFailed }}</div>
      </div>
      <el-icon class="number-deco" :size="48"><CircleClose /></el-icon>
    </div>
  </div>
</template>

<script setup>
import { ref, watch } from 'vue'
import { User, CircleCheck, CircleClose } from '@element-plus/icons-vue'

const props = defineProps({
  onlineUsers: { type: Number, default: 0 },
  loginSuccess: { type: Number, default: 0 },
  loginFailed: { type: Number, default: 0 },
})

const emit = defineEmits(['show-online-users'])

// countUp 动画：使用 ref + watch 实现，确保 Vue 能追踪中间帧的变化
function useCountUp(getter, duration = 800) {
  const display = ref(0)
  let animFrame = null

  watch(getter, (newVal) => {
    if (animFrame) cancelAnimationFrame(animFrame)
    const start = display.value
    const diff = newVal - start
    if (diff === 0) return
    const startTime = performance.now()
    function step(now) {
      const elapsed = now - startTime
      const progress = Math.min(elapsed / duration, 1)
      const eased = 1 - Math.pow(1 - progress, 3)
      display.value = Math.round(start + diff * eased)
      if (progress < 1) animFrame = requestAnimationFrame(step)
    }
    animFrame = requestAnimationFrame(step)
  })

  return display
}

const animatedOnlineUsers = useCountUp(() => props.onlineUsers)
const animatedLoginSuccess = useCountUp(() => props.loginSuccess)
const animatedLoginFailed = useCountUp(() => props.loginFailed)
</script>

<style scoped>
.dash-row {
  display: grid;
  gap: 16px;
  margin-bottom: 16px;
}
.stat-row {
  grid-template-columns: repeat(3, 1fr);
}

.number-card {
  border-radius: 12px;
  padding: 20px 24px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  position: relative;
  overflow: hidden;
  color: #fff;
  transition:
    transform 0.2s ease,
    box-shadow 0.2s ease;
  animation: fadeSlideUp 0.4s ease both;
}
.number-card:nth-child(2) {
  animation-delay: 0.06s;
}
.number-card:nth-child(3) {
  animation-delay: 0.12s;
}

@keyframes fadeSlideUp {
  from {
    opacity: 0;
    transform: translateY(12px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}
.number-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.12);
}
.number-card--cyan {
  background: linear-gradient(135deg, #0e7490, #06b6d4);
}
.number-card--green {
  background: linear-gradient(135deg, #15803d, #22c55e);
}
.number-card--red {
  background: linear-gradient(135deg, #b91c1c, #ef4444);
}

.number-label {
  font-size: 12px;
  opacity: 0.85;
  margin-bottom: 4px;
  letter-spacing: 0.3px;
}
.number-value {
  font-size: 32px;
  font-weight: 800;
  line-height: 1.1;
  font-family:
    system-ui,
    -apple-system,
    sans-serif;
  font-variant-numeric: tabular-nums;
}
.number-deco {
  opacity: 0.12;
  flex-shrink: 0;
}

@media (max-width: 768px) {
  .stat-row {
    grid-template-columns: 1fr;
  }
  .number-value {
    font-size: 26px;
  }
  .number-deco {
    display: none;
  }
}
</style>
