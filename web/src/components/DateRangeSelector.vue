<template>
  <div class="date-range-selector">
    <el-radio-group :model-value="activePreset" size="small" @update:model-value="onPresetChange">
      <el-radio-button value="today">今天</el-radio-button>
      <el-radio-button value="7d">7天</el-radio-button>
      <el-radio-button value="14d">14天</el-radio-button>
      <el-radio-button value="30d">30天</el-radio-button>
    </el-radio-group>
    <el-date-picker
      :model-value="modelValue"
      type="daterange"
      range-separator="至"
      start-placeholder="开始日期"
      end-placeholder="结束日期"
      format="YYYY-MM-DD"
      value-format="YYYY-MM-DD"
      size="small"
      :disabled-date="disabledDate"
      style="width: 260px"
      @update:model-value="onDateChange"
    />
  </div>
</template>

<script setup>
import { ref, watch, onMounted } from 'vue'

const props = defineProps({
  modelValue: {
    type: Array,
    default: null,
  },
  defaultPreset: {
    type: String,
    default: '7d',
    validator: (v) => ['today', '7d', '14d', '30d'].includes(v),
  },
})

const emit = defineEmits(['update:modelValue'])

const activePreset = ref(props.defaultPreset)

// 快捷按钮计算逻辑
const presetMap = {
  today: () => {
    const d = new Date()
    const ds = formatDate(d)
    return [ds, ds]
  },
  '7d': () => {
    const e = new Date()
    const s = new Date()
    s.setDate(s.getDate() - 6)
    return [formatDate(s), formatDate(e)]
  },
  '14d': () => {
    const e = new Date()
    const s = new Date()
    s.setDate(s.getDate() - 13)
    return [formatDate(s), formatDate(e)]
  },
  '30d': () => {
    const e = new Date()
    const s = new Date()
    s.setDate(s.getDate() - 29)
    return [formatDate(s), formatDate(e)]
  },
}

function formatDate(d) {
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}

// 反向匹配：给定日期范围，判断是否对应某个 preset，不匹配返回 null（自定义日期）。
// 用于在其它实例修改共享 modelValue 后，同步本实例的快捷按钮高亮。
function matchPreset(range) {
  if (!range || range.length !== 2 || !range[0] || !range[1]) return null
  for (const key of Object.keys(presetMap)) {
    const [s, e] = presetMap[key]()
    if (range[0] === s && range[1] === e) return key
  }
  return null
}

function onPresetChange(preset) {
  activePreset.value = preset
  const range = presetMap[preset]()
  emit('update:modelValue', range)
}

function onDateChange(val) {
  activePreset.value = null
  emit('update:modelValue', val)
}

function disabledDate(date) {
  // 如果已有起始日，限制结束日距起始日不超过 30 天
  if (props.modelValue && props.modelValue.length === 2 && props.modelValue[0]) {
    const start = new Date(props.modelValue[0])
    const diff = Math.abs(date.getTime() - start.getTime())
    const days = diff / (1000 * 60 * 60 * 24)
    return days > 30
  }
  return false
}

// 监听外部 modelValue 变化（如其它图表实例切换快捷按钮导致父级共享 ref 更新），
// 同步本实例的快捷按钮高亮，避免数据同步但高亮不一致。
// 不使用 immediate，避免覆盖 onMounted 的初始化。
watch(
  () => props.modelValue,
  (val) => {
    activePreset.value = matchPreset(val)
  }
)

// 初始化时根据 defaultPreset 设置日期范围
onMounted(() => {
  if (!props.modelValue && props.defaultPreset) {
    const range = presetMap[props.defaultPreset]()
    activePreset.value = props.defaultPreset
    emit('update:modelValue', range)
  }
})
</script>

<style scoped>
.date-range-selector {
  display: flex;
  align-items: center;
  gap: 8px;
}
</style>
