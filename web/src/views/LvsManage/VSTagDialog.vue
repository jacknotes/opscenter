<template>
  <el-dialog v-model="visible" title="设置 VS 标签" width="min(400px, 90vw)" align-center>
    <el-form label-width="80px">
      <el-form-item label="VS IP">
        <el-input :model-value="form.vs_ip" disabled />
      </el-form-item>
      <el-form-item label="标签">
        <el-input v-model="localForm.tag" placeholder="请输入标签，如：1号lvs" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button v-if="localForm.tag" type="danger" :loading="saving" @click="$emit('delete', localForm)">删除标签</el-button>
      <span style="flex: 1;"></span>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" :loading="saving" @click="$emit('save', localForm)">保存</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, watch } from 'vue'

const props = defineProps({
  form: { type: Object, required: true },
  saving: { type: Boolean, default: false },
})

defineEmits(['save', 'delete'])

const visible = defineModel({ type: Boolean, default: false })

const localForm = ref({ ...props.form })

watch(() => props.form, (val) => { localForm.value = { ...val } }, { deep: true })
watch(visible, (val) => { if (val) localForm.value = { ...props.form } })
</script>
