<script setup lang="ts">
import { ref, computed } from 'vue'
import type { NginxUpstream, NginxUpstreamPayload, NginxSwapPayload } from '@/api/types'
import { i18n } from '@/i18n'

const t = i18n.global.t

const visible = defineModel<boolean>('visible', { default: false })

const props = defineProps<{
  action: 'online' | 'offline' | 'swap' | 'toggle'
  upstreams: NginxUpstream[]
  /** 已选 upstream 名（从表格行带入） */
  initialUpstream?: string
}>()

const emit = defineEmits<{
  (e: 'confirm', payload: NginxUpstreamPayload | NginxSwapPayload): void
}>()

const upstreamName = ref('')
const backendIps = ref<string[]>([])
const offlineIp = ref('')
const onlineIp = ref('')

const current = computed(() => props.upstreams.find((u) => u.name === upstreamName.value))
const upServers = computed(() => current.value?.servers.filter((s) => s.status === 'up') ?? [])
const downServers = computed(() => current.value?.servers.filter((s) => s.status === 'down') ?? [])

const canConfirm = computed(() => {
  if (!upstreamName.value) return false
  switch (props.action) {
    case 'online':
      return backendIps.value.length > 0
    case 'offline':
      return backendIps.value.length > 0
    case 'swap':
      return Boolean(offlineIp.value && onlineIp.value)
    case 'toggle':
      return true
  }
})

function open(): void {
  upstreamName.value = props.initialUpstream ?? ''
  backendIps.value = []
  offlineIp.value = ''
  onlineIp.value = ''
  visible.value = true
}

function confirm(): void {
  if (!canConfirm.value) return
  if (props.action === 'swap') {
    emit('confirm', {
      upstream_names: [upstreamName.value],
      offline_ip: offlineIp.value,
      online_ip: onlineIp.value,
    } as NginxSwapPayload)
  } else if (props.action === 'toggle') {
    emit('confirm', { upstream_names: [upstreamName.value], backend_ip: '' } as NginxUpstreamPayload)
  } else {
    // 契约：backend_ip 支持逗号分隔多个 IP
    emit('confirm', {
      upstream_names: [upstreamName.value],
      backend_ip: backendIps.value.join(','),
    } as NginxUpstreamPayload)
  }
  visible.value = false
}

defineExpose({ open })
</script>

<template>
  <el-dialog
    v-model="visible"
    :title="`${t(`nginx.${action}`)} · ${t('nginx.upstream')}`"
    width="520px"
    append-to-body
  >
    <el-form label-position="top">
      <el-form-item :label="t('nginx.upstream')">
        <el-select v-model="upstreamName" filterable>
          <el-option v-for="u in upstreams" :key="u.name" :value="u.name" :label="u.name" />
        </el-select>
      </el-form-item>

      <el-form-item v-if="action === 'online' || action === 'offline'" :label="t('nginx.backendIp')">
        <el-select v-model="backendIps" multiple filterable allow-create default-first-option>
          <el-option
            v-for="s in action === 'online' ? downServers : upServers"
            :key="`${s.ip}:${s.port}`"
            :value="s.ip"
            :label="`${s.ip}:${s.port} (${s.status === 'up' ? t('common.online') : t('common.offline')})`"
          />
        </el-select>
        <div class="hint">{{ t('nginx.backendIpHint') }}</div>
      </el-form-item>

      <template v-if="action === 'swap'">
        <el-form-item :label="t('nginx.offlineIp')">
          <el-select v-model="offlineIp" filterable>
            <el-option v-for="s in upServers" :key="s.ip" :value="s.ip" :label="`${s.ip}:${s.port}`" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('nginx.onlineIp')">
          <el-select v-model="onlineIp" filterable>
            <el-option v-for="s in downServers" :key="s.ip" :value="s.ip" :label="`${s.ip}:${s.port}`" />
          </el-select>
        </el-form-item>
      </template>

      <el-alert
        v-if="action === 'toggle'"
        type="info"
        :closable="false"
        :title="t('nginx.toggle')"
      />
    </el-form>
    <template #footer>
      <el-button @click="visible = false">{{ t('common.cancel') }}</el-button>
      <el-button type="primary" :disabled="!canConfirm" @click="confirm">
        {{ t('common.preview') }}
      </el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.hint {
  color: var(--text-muted);
  font-size: var(--text-xs);
  margin-top: var(--space-1);
}

.el-select {
  width: 100%;
}
</style>
