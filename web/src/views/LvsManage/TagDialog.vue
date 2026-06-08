<template>
  <el-dialog v-model="visible" title="设置 RS 标签" width="min(400px, 90vw)" align-center>
    <el-form label-width="80px">
      <el-form-item label="RS IP">
        <el-input :model-value="form.rs_ip" disabled />
      </el-form-item>
      <el-form-item label="标签">
        <el-select v-model="localForm.tag" filterable allow-create clearable placeholder="选择或输入标签" style="width: 100%">
          <el-option v-for="opt in tagOptions" :key="opt.value" :label="opt.label" :value="opt.value" />
        </el-select>
      </el-form-item>
      <el-form-item label="禁用操作">
        <el-switch v-model="localForm.disabled" />
      </el-form-item>
      <el-form-item v-if="localForm.disabled" label="禁用原因" required>
        <el-input v-model="localForm.disabled_reason" type="textarea" :rows="2" placeholder="请输入禁用原因（必填）" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button v-if="localForm.tag || localForm.disabled" type="danger" :loading="saving" @click="$emit('delete', localForm)">删除配置</el-button>
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
  tagOptions: {
    type: Array,
    default: () => [
      { label: '生产环境', value: '生产环境' },
      { label: '预生产环境', value: '预生产环境' },
    ],
  },
})

defineEmits(['save', 'delete'])

const visible = defineModel({ type: Boolean, default: false })

const localForm = ref({ ...props.form })

watch(() => props.form, (val) => { localForm.value = { ...val } }, { deep: true })
watch(visible, (val) => { if (val) localForm.value = { ...props.form } })
</script>
