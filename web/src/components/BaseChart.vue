<script setup lang="ts">
import { ref, watch, onMounted, onBeforeUnmount } from 'vue'
import * as echarts from 'echarts/core'
import { LineChart, BarChart, PieChart } from 'echarts/charts'
import { GridComponent, TooltipComponent, LegendComponent, GraphicComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import type { EChartsCoreOption } from 'echarts/core'

echarts.use([
  LineChart,
  BarChart,
  PieChart,
  GridComponent,
  TooltipComponent,
  LegendComponent,
  GraphicComponent,
  CanvasRenderer,
])

const props = withDefaults(
  defineProps<{
    option: EChartsCoreOption
    height?: string
    loading?: boolean
  }>(),
  {
    height: '320px',
    loading: false,
  },
)

const elRef = ref<HTMLElement>()
let chart: echarts.ECharts | null = null
let resizeObserver: ResizeObserver | null = null

function render(): void {
  if (!elRef.value) return
  if (!chart) {
    chart = echarts.init(elRef.value)
  }
  chart.setOption(props.option, true)
}

onMounted(() => {
  render()
  resizeObserver = new ResizeObserver(() => chart?.resize())
  if (elRef.value) resizeObserver.observe(elRef.value)
})

watch(
  () => props.option,
  () => render(),
  { deep: true },
)

onBeforeUnmount(() => {
  resizeObserver?.disconnect()
  resizeObserver = null
  chart?.dispose()
  chart = null
})

defineExpose({ resize: () => chart?.resize() })
</script>

<template>
  <div v-loading="loading" class="base-chart" :style="{ height }">
    <div ref="elRef" class="chart-el" />
  </div>
</template>

<style scoped>
.base-chart {
  width: 100%;
}

.chart-el {
  width: 100%;
  height: 100%;
}
</style>
