<template>
  <el-tag :type="computedType" size="small" effect="light">
    <span :class="['status-dot', `status-${status}`]"></span>
    {{ label || status }}
  </el-tag>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  status: { type: String, required: true },
  label: { type: String, default: '' },
  typeMap: {
    type: Object,
    default: () => ({
      up: 'success',
      down: 'danger',
      online: 'success',
      offline: 'danger',
      running: 'success',
      stopped: 'danger',
      healthy: 'success',
      unhealthy: 'danger',
    }),
  },
})

const computedType = computed(() => props.typeMap[props.status] || 'info')
</script>
