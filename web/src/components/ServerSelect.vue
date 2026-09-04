<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { serverApi } from '@/api'
import type { ServerResponse } from '@/api/types'
import { i18n } from '@/i18n'

const t = i18n.global.t

const serverId = defineModel<number | undefined>('serverId', { default: undefined })

const props = withDefaults(
  defineProps<{
    /** 按服务器类型过滤（lvs/nginx/kubernetes/preprod），空为全部 */
    type?: string
    /** 是否包含禁用服务器 */
    includeDisabled?: boolean
  }>(),
  {
    type: '',
    includeDisabled: false,
  },
)

const servers = defineModel<ServerResponse[]>('servers', { default: () => [] })
const loading = ref(false)

async function load(): Promise<void> {
  loading.value = true
  try {
    const list = await serverApi.list({
      ...(props.type ? { type: props.type } : {}),
      ...(props.includeDisabled ? { all: true } : {}),
    })
    servers.value = list
    // 当前选中项不在列表中时清空
    if (serverId.value !== undefined && !list.some((s) => s.id === serverId.value)) {
      serverId.value = undefined
    }
  } finally {
    loading.value = false
  }
}

onMounted(load)

watch(
  () => [props.type, props.includeDisabled],
  () => void load(),
)

defineExpose({ reload: load })
</script>

<template>
  <el-select
    v-model="serverId"
    :placeholder="t('common.serverPlaceholder')"
    :loading="loading"
    filterable
    class="server-select"
  >
    <el-option
      v-for="s in servers"
      :key="s.id"
      :value="s.id"
      :label="`${s.name} (${s.host})`"
      :disabled="!s.enabled"
    >
      <span>{{ s.name }}</span>
      <span class="opt-host mono">{{ s.host }}</span>
      <span v-if="s.env" class="opt-env">{{ s.env }}</span>
    </el-option>
  </el-select>
</template>

<style scoped>
.server-select {
/* el-select 默认 width:100%，在 page-head 弹性布局中会撑爆标题区 */
width: 260px;
max-width: 40vw;
}

.opt-host {
  color: var(--text-muted);
  margin-left: var(--space-2);
  font-size: var(--text-xs);
}

.opt-env {
  float: right;
  color: var(--text-faint);
  font-size: var(--text-xs);
}
</style>
