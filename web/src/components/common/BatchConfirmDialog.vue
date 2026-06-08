<template>
  <el-dialog v-model="visible" :title="title" width="460px" :close-on-click-modal="false" @close="handleClose">
    <div class="batch-confirm-content">
      <el-alert
        :title="`当前已选择 ${count} 项`"
        type="warning"
        :closable="false"
        show-icon
        style="margin-bottom: 16px"
      />
      <p style="color: var(--text-regular); margin-bottom: 12px">
        请输入 <strong style="color: var(--color-danger)">{{ confirmText }}</strong> 以确认执行：
      </p>
      <el-input v-model="inputValue" :placeholder="confirmText" @keyup.enter="handleConfirm" />
    </div>
    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" :disabled="inputValue !== confirmText" :loading="loading" @click="handleConfirm">
        确认执行
      </el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, watch } from 'vue'

const props = defineProps({
  title: { type: String, default: '批量操作确认' },
  count: { type: Number, default: 0 },
  confirmText: { type: String, default: '确认执行' },
  loading: { type: Boolean, default: false },
})

const emit = defineEmits(['confirm'])

const visible = defineModel({ type: Boolean, default: false })
const inputValue = ref('')

watch(visible, (val) => {
  if (val) inputValue.value = ''
})

function handleConfirm() {
  if (inputValue.value === props.confirmText) {
    emit('confirm')
  }
}

function handleClose() {
  inputValue.value = ''
}
</script>

<style scoped>
.batch-confirm-content {
  padding: 8px 0;
}
</style>
