<script setup lang="ts">
withDefaults(
  defineProps<{
    label: string
    value: string | number
    color?: string
    hint?: string
    delay?: number
    live?: boolean
    liveOffline?: boolean
  }>(),
  {
    color: 'var(--indigo-400)',
    hint: '',
    delay: 0,
    live: false,
    liveOffline: false,
  },
)
</script>

<template>
  <div class="stat-card card hoverable reveal" :style="{ '--d': delay, '--accent': color }">
    <div class="accent-bar" />
    <div class="stat-body">
      <div class="stat-label">
        <span v-if="live" class="dot-live" :class="{ 'is-offline': liveOffline }" />
        <span>{{ label }}</span>
      </div>
      <div class="stat-value mono">{{ value }}</div>
      <div v-if="hint" class="stat-hint">{{ hint }}</div>
    </div>
  </div>
</template>

<style scoped>
.stat-card {
  position: relative;
  padding: var(--space-5);
  overflow: hidden;
}

.accent-bar {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 3px;
  background: linear-gradient(90deg, var(--accent), transparent);
  opacity: 0.85;
}

.stat-label {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  color: var(--text-secondary);
  font-size: var(--text-sm);
}

.stat-value {
  font-size: var(--text-2xl);
  font-weight: 700;
  margin-top: var(--space-2);
  color: var(--text-primary);
  text-shadow: 0 0 24px color-mix(in srgb, var(--accent) 35%, transparent);
}

.stat-hint {
  margin-top: var(--space-1);
  color: var(--text-muted);
  font-size: var(--text-xs);
}
</style>
