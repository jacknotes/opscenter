<script setup lang="ts">
import { computed } from 'vue'
import type { EChartsCoreOption } from 'echarts/core'
import type { K8sRemoteStat, LvsRemoteStat, NginxRemoteStat, PreprodRemoteStat } from '@/api/types'
import BaseChart from '@/components/BaseChart.vue'
import { useTheme } from '@/composables/useTheme'

const props = defineProps<{
  loading: boolean
  error: boolean
  lvs: LvsRemoteStat | null
  nginx: NginxRemoteStat | null
  k8s: K8sRemoteStat | null
  preprod: PreprodRemoteStat | null
}>()

const emit = defineEmits<{ retry: [] }>()

const { theme } = useTheme()

// 主题感知配色（与 index.vue 图表配色保持一致）
const palette = computed(() => {
  void theme.value
  const dark = theme.value === 'dark'
  return {
    text: dark ? '#e2e8f0' : '#1e293b',
    subText: dark ? '#94a3b8' : '#475569',
    muted: dark ? '#64748b' : '#64748b',
    cardBg: dark ? '#172133' : '#ffffff',
    online: '#34d399',
    offline: '#f87171',
    pending: '#fbbf24',
    other: '#94a3b8',
    scaledDown: '#f87171',
    expanded: '#fbbf24',
    normal: '#34d399',
  }
})

interface RingDatum {
  name: string
  value: number
}

function makeRingOption(data: RingDatum[], colors: string[], total: number, unit: string): EChartsCoreOption {
  const p = palette.value
  return {
    tooltip: {
      trigger: 'item',
      backgroundColor: p.cardBg,
      borderColor: p.subText,
      textStyle: { color: p.text, fontSize: 13 },
      confine: true,
      formatter: '{b}: {c} ' + unit + ' ({d}%)',
    },
    legend: { bottom: 0, textStyle: { color: p.subText, fontSize: 11 } },
    color: colors,
    graphic: [
      {
        type: 'group',
        left: 'center',
        top: '40%',
        children: [
          {
            type: 'text',
            style: {
              text: String(total),
              textAlign: 'center',
              fill: p.text,
              fontSize: 28,
              fontWeight: 700,
            },
          },
          {
            type: 'text',
            top: 32,
            style: { text: '总计', textAlign: 'center', fill: p.muted, fontSize: 12 },
          },
        ],
      },
    ],
    series: [
      {
        type: 'pie',
        radius: ['50%', '70%'],
        center: ['50%', '46%'],
        avoidLabelOverlap: false,
        label: { show: false },
        emphasis: {
          label: { show: true, fontSize: 13, fontWeight: 'bold', color: p.text },
          itemStyle: { shadowBlur: 15, shadowColor: 'rgba(0,0,0,0.15)' },
        },
        itemStyle: { borderRadius: 4, borderColor: p.cardBg, borderWidth: 2 },
        data,
      },
    ],
  }
}

const lvsPie = computed<EChartsCoreOption | null>(() => {
  if (!props.lvs) return null
  const total = (props.lvs.rs_online ?? 0) + (props.lvs.rs_offline ?? 0)
  return makeRingOption(
    [
      { name: '在线', value: props.lvs.rs_online ?? 0 },
      { name: '离线', value: props.lvs.rs_offline ?? 0 },
    ],
    [palette.value.online, palette.value.offline],
    total,
    'RS',
  )
})

const nginxPie = computed<EChartsCoreOption | null>(() => {
  if (!props.nginx) return null
  const total = (props.nginx.server_online ?? 0) + (props.nginx.server_offline ?? 0)
  return makeRingOption(
    [
      { name: '在线', value: props.nginx.server_online ?? 0 },
      { name: '离线', value: props.nginx.server_offline ?? 0 },
    ],
    [palette.value.online, palette.value.offline],
    total,
    'Server',
  )
})

const k8sPie = computed<EChartsCoreOption | null>(() => {
  if (!props.k8s) return null
  const other = (props.k8s.total_rollouts ?? 0) - (props.k8s.pending ?? 0) - (props.k8s.online ?? 0)
  return makeRingOption(
    [
      { name: '暂停中', value: props.k8s.pending ?? 0 },
      { name: '已上线', value: props.k8s.online ?? 0 },
      { name: '其他', value: Math.max(0, other) },
    ],
    [palette.value.pending, palette.value.online, palette.value.other],
    props.k8s.total_rollouts ?? 0,
    '个',
  )
})

const preprodPie = computed<EChartsCoreOption | null>(() => {
  if (!props.preprod) return null
  const total = (props.preprod.scaled_down ?? 0) + (props.preprod.expanded ?? 0) + (props.preprod.normal ?? 0)
  return makeRingOption(
    [
      { name: '已缩容', value: props.preprod.scaled_down ?? 0 },
      { name: '已扩容', value: props.preprod.expanded ?? 0 },
      { name: '正常', value: props.preprod.normal ?? 0 },
    ],
    [palette.value.scaledDown, palette.value.expanded, palette.value.normal],
    total,
    '个',
  )
})

const modules = computed(() => [
  { key: 'lvs', title: 'LVS RealServer', data: props.lvs, option: lvsPie.value },
  { key: 'nginx', title: 'Nginx Server', data: props.nginx, option: nginxPie.value },
  { key: 'k8s', title: 'K8S Rollout', data: props.k8s, option: k8sPie.value },
  { key: 'preprod', title: '预生产资源', data: props.preprod, option: preprodPie.value },
])
</script>

<template>
  <div class="pies-row">
    <div v-for="mod in modules" :key="mod.key" class="card pie-card reveal">
      <h3 class="pie-title">{{ mod.title }}</h3>
      <template v-if="mod.data && mod.option">
        <BaseChart :option="mod.option" height="220px" :loading="props.loading" />
      </template>
      <div v-else-if="props.loading" class="pie-state">加载中…</div>
      <div v-else class="pie-state">
        <span>{{ props.error ? '远程数据加载失败' : '暂无数据' }}</span>
        <el-button text type="primary" size="small" @click="emit('retry')">重试</el-button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.pies-row {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: var(--space-4);
  margin-bottom: var(--space-4);
}

@media (max-width: 1200px) {
  .pies-row {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 560px) {
  .pies-row {
    grid-template-columns: 1fr;
  }
}

.pie-card {
  padding: var(--space-4);
  min-width: 0;
}

.pie-title {
  margin: 0 0 var(--space-2);
  font-family: var(--font-display);
  font-size: var(--text-sm);
  color: var(--text-primary);
  font-weight: 600;
}

.pie-state {
  height: 220px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: var(--space-2);
  color: var(--text-muted);
  font-size: var(--text-sm);
}
</style>
