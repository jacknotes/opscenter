<script setup lang="ts">
import { ref, reactive, onMounted, onBeforeUnmount } from 'vue'
import { ElMessage } from 'element-plus'
import { authApi, extractErrorMessage } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { i18n } from '@/i18n'

const t = i18n.global.t
const auth = useAuthStore()

const visible = ref(false)
const saving = ref(false)

const form = reactive({
  old_password: '',
  new_password: '',
  confirm: '',
})

function onOpenEvent(): void {
  Object.assign(form, { old_password: '', new_password: '', confirm: '' })
  visible.value = true
}

onMounted(() => window.addEventListener('opscenter:change-password', onOpenEvent))
onBeforeUnmount(() => window.removeEventListener('opscenter:change-password', onOpenEvent))

async function save(): Promise<void> {
  if (!form.old_password || !form.new_password) return
  if (form.new_password !== form.confirm) {
    ElMessage.warning(t('users.passwordStrength'))
    return
  }
  if (!auth.user) return
  saving.value = true
  try {
    await authApi.changeMyPassword(auth.user.id, {
      old_password: form.old_password,
      new_password: form.new_password,
    })
    ElMessage.success(t('common.execSuccess'))
    visible.value = false
  } catch (err) {
    ElMessage.error(extractErrorMessage(err))
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <el-dialog v-model="visible" :title="t('users.changePasswordTitle')" width="440px" append-to-body>
    <el-form label-position="top" @keyup.enter="save">
      <el-form-item :label="t('users.oldPassword')" required>
        <el-input v-model="form.old_password" type="password" show-password />
      </el-form-item>
      <el-form-item :label="t('users.newPassword')" required>
        <el-input v-model="form.new_password" type="password" show-password :placeholder="t('users.passwordStrength')" />
      </el-form-item>
      <el-form-item :label="t('common.confirm')" required>
        <el-input v-model="form.confirm" type="password" show-password />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="visible = false">{{ t('common.cancel') }}</el-button>
      <el-button type="primary" :loading="saving" @click="save">{{ t('common.save') }}</el-button>
    </template>
  </el-dialog>
</template>
