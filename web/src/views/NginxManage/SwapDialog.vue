<template>
  <el-dialog v-model="visible" title="切换服务器" width="min(600px, 90vw)" class="cool-dialog" align-center>
    <div v-if="offlineIP" class="swap-dialog-body">
      <div class="swap-ip-pair">
        <el-tag type="danger" size="large">{{ offlineIP }} (下线)</el-tag>
        <span class="swap-arrow">⇅</span>
        <el-tag type="success" size="large">{{ onlineIP }} (上线)</el-tag>
      </div>
      <div class="swap-upstream-list">
        <div class="swap-label">选择要执行切换的 Upstream 组：</div>
        <div v-for="(item, index) in localUpstreams" :key="item.name" class="swap-upstream-item">
          <el-checkbox :model-value="item.checked" @change="toggleChecked(index)" />
          <span class="upstream-name">{{ item.name }}</span>
          <span class="badge badge-info">{{ item.totalCount }} 台</span>
          <span class="badge badge-success">{{ item.upCount }} up</span>
          <span class="badge badge-danger">{{ item.downCount }} down</span>
        </div>
        <div
          v-if="localUpstreams.length === 0"
          style="color: var(--text-secondary); padding: 20px 0; text-align: center"
        >
          未找到同时包含这两台服务器的 Upstream 组
        </div>
      </div>
    </div>
    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button
        type="primary"
        :disabled="localUpstreams.filter((i) => i.checked).length === 0"
        @click="$emit('confirm')"
        >确认切换</el-button
      >
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, watch } from 'vue'

const props = defineProps({
  offlineIP: { type: String, default: '' },
  onlineIP: { type: String, default: '' },
  affectedUpstreams: { type: Array, default: () => [] },
})

const emit = defineEmits(['confirm', 'update:affectedUpstreams'])

const visible = defineModel({ type: Boolean, default: false })

// 本地副本，避免直接修改 prop
const localUpstreams = ref([])

watch(
  () => props.affectedUpstreams,
  (val) => {
    localUpstreams.value = val.map((item) => ({ ...item }))
  },
  { immediate: true, deep: true }
)

function toggleChecked(index) {
  localUpstreams.value[index].checked = !localUpstreams.value[index].checked
  emit('update:affectedUpstreams', localUpstreams.value.map((item) => ({ ...item })))
}
</script>

<style scoped>
.swap-dialog-body {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.swap-ip-pair {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 16px;
}

.swap-label {
  font-size: 13px;
  color: var(--text-regular);
  font-weight: 500;
  margin-bottom: 10px;
}

.swap-arrow {
  font-size: 28px;
  color: #06b6d4;
  font-weight: 700;
  line-height: 1;
}

.swap-upstream-list {
  max-height: 400px;
  overflow-y: auto;
}

.swap-upstream-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  border: 1px solid var(--border-default);
  border-radius: 8px;
  margin-bottom: 8px;
  transition: background 0.15s;
}

.swap-upstream-item:hover {
  background: var(--bg-elevated);
}

.upstream-name {
  font-weight: 700;
  color: var(--text-primary);
  font-size: 14px;
  letter-spacing: 0.3px;
}

.badge {
  display: inline-flex;
  align-items: center;
  padding: 0 8px;
  border-radius: 8px;
  font-size: 11px;
  font-weight: 500;
  line-height: 1;
}

.badge-info {
  background: rgba(6, 182, 212, 0.12);
  color: #06b6d4;
}
.badge-success {
  background: rgba(34, 197, 94, 0.12);
  color: #22c55e;
}
.badge-danger {
  background: rgba(239, 68, 68, 0.12);
  color: #ef4444;
}
</style>
