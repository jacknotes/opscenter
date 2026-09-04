<script setup lang="ts">
import { ref, reactive, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { lvsApi, extractErrorMessage } from '@/api'
import type { LvsRSTag, LvsVSTag } from '@/api/types'
import { i18n } from '@/i18n'

const t = i18n.global.t

/** mode: 'rs' 编辑 RS 标签；'vs' 编辑 VS 标签 */
const props = defineProps<{
  mode: 'rs' | 'vs'
}>()

const visible = defineModel<boolean>('visible', { default: false })

const emit = defineEmits<{ (e: 'saved'): void }>()

const saving = ref(false)

const rsForm = reactive({
  rs_ip: '',
  vs_ip: '',
  tag: '',
  disabled: false,
  disabled_reason: '',
})

const vsForm = reactive({
  vs_ip: '',
  tag: '',
})

type TagInput =
  | Pick<LvsRSTag, 'rs_ip' | 'vs_ip' | 'tag' | 'disabled' | 'disabled_reason'>
  | Pick<LvsVSTag, 'vs_ip' | 'tag'>

async function open(payload: TagInput): Promise<void> {
  visible.value = true
  if (props.mode === 'rs') {
    const p = payload as Pick<LvsRSTag, 'rs_ip' | 'vs_ip' | 'tag' | 'disabled' | 'disabled_reason'>
    Object.assign(rsForm, { rs_ip: p.rs_ip, vs_ip: p.vs_ip, tag: p.tag ?? '', disabled: p.disabled ?? false, disabled_reason: p.disabled_reason ?? '' })
  } else {
    const p = payload as Pick<LvsVSTag, 'vs_ip' | 'tag'>
    Object.assign(vsForm, { vs_ip: p.vs_ip, tag: p.tag ?? '' })
  }
}

async function save(): Promise<void> {
  saving.value = true
  try {
    if (props.mode === 'rs') {
      if (rsForm.disabled && !rsForm.disabled_reason.trim()) {
        ElMessage.warning(t('lvs.tagDialog.reasonRequired'))
        return
      }
      await lvsApi.saveRSTag({ ...rsForm })
    } else {
      await lvsApi.saveVSTag({ ...vsForm })
    }
    ElMessage.success(t('common.execSuccess'))
    visible.value = false
    emit('saved')
  } catch (err) {
    ElMessage.error(extractErrorMessage(err))
  } finally {
    saving.value = false
  }
}

defineExpose({ open })

watch(visible, (v) => {
  if (!v) return
})
</script>

<template>
  <el-dialog
    v-model="visible"
    :title="mode === 'rs' ? t('lvs.tagDialog.title') : t('lvs.vsTagDialog.title')"
    width="480px"
    append-to-body
  >
    <el-form label-position="top">
      <el-form-item :label="mode === 'rs' ? 'RS IP' : 'VS IP'">
        <el-input :model-value="mode === 'rs' ? rsForm.rs_ip : vsForm.vs_ip" class="mono" disabled />
      </el-form-item>
      <el-form-item :label="t('lvs.tag')">
        <el-input v-if="mode === 'rs'" v-model="rsForm.tag" :placeholder="t('lvs.tag')" />
        <el-input v-else v-model="vsForm.tag" :placeholder="t('lvs.tag')" />
      </el-form-item>
      <template v-if="mode === 'rs'">
        <el-form-item :label="t('lvs.tagDialog.disabled')">
          <el-switch v-model="rsForm.disabled" />
        </el-form-item>
        <el-form-item v-if="rsForm.disabled" :label="t('lvs.tagDialog.disabledReason')" required>
          <el-input v-model="rsForm.disabled_reason" type="textarea" :rows="2" />
        </el-form-item>
      </template>
    </el-form>
    <template #footer>
      <el-button @click="visible = false">{{ t('common.cancel') }}</el-button>
      <el-button type="primary" :loading="saving" @click="save">{{ t('common.save') }}</el-button>
    </template>
  </el-dialog>
</template>
