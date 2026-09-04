<template>
  <div class="date-range-selector">
    <el-radio-group :model-value="activePreset ?? undefined" size="small" @update:model-value="onPresetChange">
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

<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'

type Preset = 'today' | '7d' | '14d' | '30d'

const props = withDefaults(
  defineProps<{
    /** [start, end]，格式 YYYY-MM-DD；null 表示未初始化 */
    modelValue?: string[] | null
    defaultPreset?: Preset
  }>(),
  { modelValue: null, defaultPreset: '7d' },
)

const emit = defineEmits<{ 'update:modelValue': [value: string[] | null] }>()

const activePreset = ref<Preset | null>(props.defaultPreset)

function formatDate(d: Date): string {
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}

// 快捷按钮计算逻辑
const presetMap: Record<Preset, () => string[]> = {
  today: () => {
    const ds = formatDate(new Date())
    return [ds, ds]
  },
  '7d': () => rangeBack(6),
  '14d': () => rangeBack(13),
  '30d': () => rangeBack(29),
}

function rangeBack(days: number): string[] {
  const e = new Date()
  const s = new Date()
  s.setDate(s.getDate() - days)
  return [formatDate(s), formatDate(e)]
}

// 反向匹配：给定日期范围，判断是否对应某个 preset，不匹配返回 null（自定义日期）。
// 用于在其它实例修改共享 modelValue 后，同步本实例的快捷按钮高亮。
function matchPreset(range: string[] | null | undefined): Preset | null {
  if (!range || range.length !== 2 || !range[0] || !range[1]) return null
  for (const key of Object.keys(presetMap) as Preset[]) {
    const [s, e] = presetMap[key]()
    if (range[0] === s && range[1] === e) return key
  }
  return null
}

function onPresetChange(preset: string | number | boolean | undefined) {
  activePreset.value = preset as Preset
  emit('update:modelValue', presetMap[preset as Preset]())
}

function onDateChange(val: string[] | null) {
  activePreset.value = null
  emit('update:modelValue', val)
}

function disabledDate(date: Date): boolean {
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
  },
)

// 初始化时根据 defaultPreset 设置日期范围
onMounted(() => {
  if (!props.modelValue && props.defaultPreset) {
    activePreset.value = props.defaultPreset
    emit('update:modelValue', presetMap[props.defaultPreset]())
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
