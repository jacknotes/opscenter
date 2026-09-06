<script setup lang="ts">
import { ref, watch } from 'vue'
import { i18n } from '@/i18n'

const t = i18n.global.t

export interface SwapScopeItem {
  name: string
  total: number
  up: number
  down: number
  checked: boolean
}

const visible = defineModel<boolean>('visible', { default: false })

const props = defineProps<{
  offlineIp: string
  onlineIp: string
  affected: SwapScopeItem[]
}>()

const emit = defineEmits<{
  (e: 'confirm', names: string[]): void
}>()

const local = ref<SwapScopeItem[]>([])

watch(
  () => props.affected,
  (val) => {
    local.value = val.map((i) => ({ ...i }))
  },
  { immediate: true, deep: true },
)

function toggle(index: number): void {
  local.value[index].checked = !local.value[index].checked
}

function confirm(): void {
  emit(
    'confirm',
    local.value.filter((i) => i.checked).map((i) => i.name),
  )
}
</script>

<template>
  <el-dialog v-model="visible" title="切换服务器" width="min(560px, 92vw)" align-center append-to-body>
    <div class="swap-body">
      <div class="swap-pair">
        <el-tag type="danger" size="large">{{ offlineIp }} (下线)</el-tag>
        <span class="swap-arrow">⇅</span>
        <el-tag type="success" size="large">{{ onlineIp }} (上线)</el-tag>
      </div>
      <div class="swap-label">选择要执行切换的 Upstream 组：</div>
      <div class="swap-list">
        <div v-for="(item, idx) in local" :key="item.name" class="swap-item" @click="toggle(idx)">
          <el-checkbox :model-value="item.checked" @click.stop @change="toggle(idx)" />
          <span class="swap-name mono">{{ item.name }}</span>
          <span class="swap-badge">{{ item.total }} 台</span>
          <span class="swap-badge swap-badge-up">{{ item.up }} up</span>
          <span class="swap-badge swap-badge-down">{{ item.down }} down</span>
        </div>
        <div v-if="local.length === 0" class="swap-empty">未找到同时包含这两台服务器的 Upstream 组</div>
      </div>
    </div>
    <template #footer>
      <el-button @click="visible = false">{{ t('common.cancel') }}</el-button>
      <el-button type="primary" :disabled="local.every((i) => !i.checked)" @click="confirm">确认切换</el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.swap-body {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.swap-pair {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 16px;
}

.swap-arrow {
  font-size: 24px;
  font-weight: 700;
  color: var(--indigo-400);
  line-height: 1;
}

.swap-label {
  font-size: 13px;
  font-weight: 500;
  color: var(--text-secondary);
}

.swap-list {
  max-height: 380px;
  overflow-y: auto;
}

.swap-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md, 8px);
  margin-bottom: 8px;
  cursor: pointer;
  transition: background 0.15s;
}

.swap-item:hover {
  background: var(--bg-input);
}

.swap-name {
  font-weight: 600;
  font-size: 13px;
  color: var(--text-primary);
}

.swap-badge {
  display: inline-flex;
  align-items: center;
  padding: 0 8px;
  border-radius: 8px;
  font-size: 11px;
  font-weight: 500;
  line-height: 1.8;
  color: var(--text-secondary);
  background: var(--bg-input);
  border: 1px solid var(--border);
}

.swap-badge-up {
  color: var(--emerald-400);
}

.swap-badge-down {
  color: var(--rose-400);
}

.swap-empty {
  color: var(--text-secondary);
  padding: 20px 0;
  text-align: center;
  font-size: 13px;
}
</style>
