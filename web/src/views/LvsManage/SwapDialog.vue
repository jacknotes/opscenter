<script setup lang="ts">
import { ref, computed } from 'vue'
import type { VirtualServer } from '@/api/types'
import { i18n } from '@/i18n'

const t = i18n.global.t

const visible = defineModel<boolean>('visible', { default: false })

const props = defineProps<{
  /** 当前 VS（切换的两台 RS 必须属于同一 VS） */
  vs: VirtualServer | null
}>()

const emit = defineEmits<{
  (e: 'confirm', payload: { rs_ip1: string; rs_ip2: string }): void
}>()

const rs1 = ref('')
const rs2 = ref('')

const rsOptions = computed(() => props.vs?.real_servers ?? [])

function open(): void {
  rs1.value = ''
  rs2.value = ''
  visible.value = true
}

function confirm(): void {
  if (!rs1.value || !rs2.value || rs1.value === rs2.value) return
  emit('confirm', { rs_ip1: rs1.value, rs_ip2: rs2.value })
}

defineExpose({ open })
</script>

<template>
  <el-dialog v-model="visible" :title="`${t('lvs.swap')} · ${vs?.ip ?? ''}`" width="480px" append-to-body>
    <el-form label-position="top">
      <el-form-item :label="t('lvs.stateOff')">
        <el-select v-model="rs1" filterable>
          <el-option v-for="rs in rsOptions" :key="rs.ip" :value="rs.ip" :label="`${rs.ip}:${rs.port} (${rs.status})`" />
        </el-select>
      </el-form-item>
      <el-form-item :label="t('lvs.stateOn')">
        <el-select v-model="rs2" filterable>
          <el-option v-for="rs in rsOptions" :key="rs.ip" :value="rs.ip" :label="`${rs.ip}:${rs.port} (${rs.status})`" />
        </el-select>
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="visible = false">{{ t('common.cancel') }}</el-button>
      <el-button type="primary" :disabled="!rs1 || !rs2 || rs1 === rs2" @click="confirm">
        {{ t('common.preview') }}
      </el-button>
    </template>
  </el-dialog>
</template>
