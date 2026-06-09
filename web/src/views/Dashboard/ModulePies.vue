<template>
  <div class="dash-row pies-row">
    <el-card v-for="mod in modules" :key="mod.key" class="chart-card" shadow="hover">
      <template #header
        ><span class="chart-title">{{ mod.title }}</span></template
      >
      <div class="chart-wrap">
        <v-chart v-if="!loading && mod.data" class="chart chart--sm" :option="mod.option" autoresize />
        <div v-else-if="loading" class="chart-loading"><el-skeleton :rows="3" animated /></div>
        <div v-else class="chart-error">
          <span>{{ error || '加载失败' }}</span>
          <el-button text type="primary" size="small" @click="$emit('retry')">重试</el-button>
        </div>
        <div v-if="loading && mod.data" class="chart-overlay">
          <el-icon class="is-loading" :size="24"><Loading /></el-icon>
        </div>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { Loading } from '@element-plus/icons-vue'
import VChart from 'vue-echarts'

const props = defineProps({
  loading: { type: Boolean, default: false },
  error: { type: String, default: '' },
  lvsStats: { type: Object, default: null },
  nginxStats: { type: Object, default: null },
  k8sStats: { type: Object, default: null },
  preprodStats: { type: Object, default: null },
  themeColors: { type: Object, required: true },
  colors: { type: Object, required: true },
  cardBg: { type: String, default: '#141722' },
})

defineEmits(['retry'])

function tooltipConf(extra = {}) {
  return {
    backgroundColor: props.themeColors.tooltipBg,
    borderColor: props.themeColors.tooltipBorder,
    textStyle: { color: props.themeColors.text, fontSize: 13 },
    ...extra,
  }
}

function makeRingOption(data, legendNames, colors, total, unit) {
  return {
    tooltip: tooltipConf({ trigger: 'item', formatter: '{b}: {c} ' + unit + ' ({d}%)' }),
    legend: { bottom: 0, textStyle: { color: props.themeColors.subText, fontSize: 11 } },
    color: colors,
    graphic: [
      {
        type: 'group',
        left: 'center',
        top: '36%',
        children: [
          {
            type: 'text',
            style: {
              text: String(total),
              textAlign: 'center',
              fill: props.themeColors.text,
              fontSize: 28,
              fontWeight: 700,
              fontFamily: 'system-ui, -apple-system, sans-serif',
            },
          },
          {
            type: 'text',
            top: 32,
            style: { text: '总计', textAlign: 'center', fill: props.themeColors.muted, fontSize: 12 },
          },
        ],
      },
    ],
    series: [
      {
        type: 'pie',
        radius: ['50%', '70%'],
        center: ['50%', '42%'],
        avoidLabelOverlap: false,
        label: { show: false },
        emphasis: {
          label: { show: true, fontSize: 13, fontWeight: 'bold', color: props.themeColors.text },
          itemStyle: { shadowBlur: 15, shadowColor: 'rgba(0,0,0,0.15)' },
        },
        itemStyle: { borderRadius: 4, borderColor: props.cardBg, borderWidth: 2 },
        data,
      },
    ],
  }
}

const lvsPie = computed(() => {
  if (!props.lvsStats) return null
  const total = (props.lvsStats.rs_online || 0) + (props.lvsStats.rs_offline || 0)
  return makeRingOption(
    [
      { name: '在线', value: props.lvsStats.rs_online || 0 },
      { name: '离线', value: props.lvsStats.rs_offline || 0 },
    ],
    ['在线', '离线'],
    [props.colors.online, props.colors.offline],
    total,
    'RS'
  )
})

const nginxPie = computed(() => {
  if (!props.nginxStats) return null
  const total = (props.nginxStats.server_online || 0) + (props.nginxStats.server_offline || 0)
  return makeRingOption(
    [
      { name: '在线', value: props.nginxStats.server_online || 0 },
      { name: '离线', value: props.nginxStats.server_offline || 0 },
    ],
    ['在线', '离线'],
    [props.colors.online, props.colors.offline],
    total,
    'Server'
  )
})

const k8sPie = computed(() => {
  if (!props.k8sStats) return null
  const other = (props.k8sStats.total_rollouts || 0) - (props.k8sStats.pending || 0) - (props.k8sStats.online || 0)
  return makeRingOption(
    [
      { name: '待发布', value: props.k8sStats.pending || 0 },
      { name: '已发布', value: props.k8sStats.online || 0 },
      { name: '其他', value: Math.max(0, other) },
    ],
    ['待发布', '已发布', '其他'],
    [props.colors.pending, props.colors.online, props.colors.other],
    props.k8sStats.total_rollouts || 0,
    '个'
  )
})

const preprodPie = computed(() => {
  if (!props.preprodStats) return null
  const total = (props.preprodStats.scaled_down || 0) + (props.preprodStats.expanded || 0) + (props.preprodStats.normal || 0)
  return makeRingOption(
    [
      { name: '已缩容', value: props.preprodStats.scaled_down || 0 },
      { name: '已扩容', value: props.preprodStats.expanded || 0 },
      { name: '正常', value: props.preprodStats.normal || 0 },
    ],
    ['已缩容', '已扩容', '正常'],
    [props.colors.scaledDown, props.colors.expanded, props.colors.normal],
    total,
    '个'
  )
})

const modules = computed(() => [
  { key: 'lvs', title: 'LVS RealServer', data: props.lvsStats, option: lvsPie.value },
  { key: 'nginx', title: 'Nginx Server', data: props.nginxStats, option: nginxPie.value },
  { key: 'k8s', title: 'K8S Rollout', data: props.k8sStats, option: k8sPie.value },
  { key: 'preprod', title: '预生产资源', data: props.preprodStats, option: preprodPie.value },
])
</script>

<style scoped>
.dash-row {
  display: grid;
  gap: 16px;
  margin-bottom: 16px;
}
.pies-row {
  grid-template-columns: repeat(4, 1fr);
}
.chart-card {
  min-width: 0;
}
.chart-title {
  font-weight: 600;
  font-size: 14px;
}
.chart {
  height: 220px;
}
.chart-wrap {
  position: relative;
}
.chart-loading,
.chart-error {
  height: 220px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}
.chart-overlay {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.15);
  border-radius: 8px;
  z-index: 5;
}

@media (max-width: 1200px) {
  .pies-row {
    grid-template-columns: repeat(2, 1fr);
  }
}
@media (max-width: 768px) {
  .pies-row {
    grid-template-columns: 1fr;
  }
}
</style>
