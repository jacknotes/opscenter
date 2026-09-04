<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { ElMessage } from 'element-plus'
import type { TableInstance } from 'element-plus'
import type { NginxBatchItem, NginxServer, NginxUpstream } from '@/api/types'
import { i18n } from '@/i18n'

const t = i18n.global.t

const visible = defineModel<boolean>('visible', { default: false })

const props = defineProps<{
  upstreams: NginxUpstream[]
}>()

const emit = defineEmits<{
  (e: 'confirm', items: NginxBatchItem[]): void
}>()

type BatchAction = '' | 'online' | 'offline' | 'toggle'

interface BatchRow {
  upstreamName: string
  enabled: boolean
  action: BatchAction
  backendIPs: string[]
  servers: NginxServer[]
  upCount: number
  downCount: number
  hasBoth: boolean
  hasMultipleUp: boolean
}

const rows = ref<BatchRow[]>([])
const batchSearch = ref('')
const syntaxHint = ref('')
const isSyntaxMode = ref(false)
const batchAllExpanded = ref(false)
const batchTableRef = ref<TableInstance>()

function open(): void {
  rows.value = props.upstreams.map((u) => {
    const upServers = u.servers.filter((s) => s.status === 'up')
    const downServers = u.servers.filter((s) => s.status === 'down')
    const hasBoth = upServers.length > 0 && downServers.length > 0
    return {
      upstreamName: u.name,
      enabled: false,
      action: (hasBoth ? 'toggle' : '') as BatchAction,
      backendIPs: [],
      servers: u.servers,
      upCount: upServers.length,
      downCount: downServers.length,
      hasBoth,
      hasMultipleUp: upServers.length >= 2,
    }
  })
  batchSearch.value = ''
  isSyntaxMode.value = false
  syntaxHint.value = ''
  batchAllExpanded.value = false
  visible.value = true
}

defineExpose({ open })

// ---------- 工具 ----------
function serverKey(s: NginxServer): string {
  return s.port ? `${s.ip}:${s.port}` : s.ip
}

/** 归一化 IP 键：去掉默认端口 :80，与后端 backend_ip 格式保持一致 */
function normalizeIPKey(s: NginxServer): string {
  const key = serverKey(s)
  return key.endsWith(':80') ? key.slice(0, -3) : key
}

function getSelectableServers(item: BatchRow): NginxServer[] {
  if (item.action === 'online') return item.servers.filter((s) => s.status === 'down')
  if (item.action === 'offline') return item.servers.filter((s) => s.status === 'up')
  return []
}

/** 下线时保留第 1 台在线服务器 */
function getOfflineSafeServers(item: BatchRow): NginxServer[] {
  return item.servers.filter((s) => s.status === 'up').slice(1)
}

function getAvailableActions(item: BatchRow): { label: string; value: Exclude<BatchAction, ''> }[] {
  const actions: { label: string; value: Exclude<BatchAction, ''> }[] = []
  if (item.hasBoth) actions.push({ label: '切换（反转全部）', value: 'toggle' })
  if (item.hasMultipleUp) actions.push({ label: t('nginx.offline'), value: 'offline' })
  if (item.downCount > 0) actions.push({ label: t('nginx.online'), value: 'online' })
  return actions
}

function onBatchActionChange(row: BatchRow): void {
  row.backendIPs = []
}

function isBatchIPAllSelected(item: BatchRow): boolean {
  const selectable = getSelectableServers(item)
  if (selectable.length === 0) return false
  if (item.action === 'offline') {
    const safe = getOfflineSafeServers(item)
    return safe.length > 0 && safe.every((s) => item.backendIPs.includes(normalizeIPKey(s)))
  }
  return selectable.every((s) => item.backendIPs.includes(normalizeIPKey(s)))
}

function toggleBatchIPSelectAll(item: BatchRow): void {
  if (isBatchIPAllSelected(item)) {
    item.backendIPs = []
  } else {
    item.backendIPs = (item.action === 'offline' ? getOfflineSafeServers(item) : getSelectableServers(item)).map((s) =>
      normalizeIPKey(s),
    )
  }
}

// ---------- 批量搜索语法：{端口|端口}{上线|下线|切换|索引} ----------
interface ParsedSyntax {
  ports: string[]
  action: 'online' | 'offline' | 'toggle'
  index: number
}

const SYNTAX_ACTION_MAP: Record<string, 'online' | 'offline' | 'toggle'> = {
  上线: 'online',
  下线: 'offline',
  切换: 'toggle',
}

function parseBatchSearchSyntax(input: string): ParsedSyntax | null {
  const trimmed = input.trim()
  const match = trimmed.match(/^\{([^}]+)\}\{([^}]+)\}$/)
  if (!match) return null

  const ports = match[1].split(/\s*\|\s*/).map((p) => p.trim()).filter(Boolean)
  if (ports.length === 0) return null

  const parts = match[2].split(/\s*\|\s*/)
  const action = SYNTAX_ACTION_MAP[parts[0]?.trim() ?? '']
  if (!action) return null

  let index = 1
  if (action !== 'toggle') {
    const rawIndex = parts[1]?.trim()
    if (rawIndex !== undefined && rawIndex !== '') {
      const parsed = parseInt(rawIndex, 10)
      if (Number.isNaN(parsed) || parsed === 0) return null
      index = parsed
    }
  }

  return { ports, action, index }
}

/** 按语法结果批量生成操作项（toggle 需组内同时有 up/down，online/offline 按 index 定位目标服务器） */
function applyBatchSearchSyntax(parsed: ParsedSyntax): number {
  let matchedCount = 0
  for (const item of rows.value) {
    const portMatch = item.servers.some((s) => parsed.ports.includes(s.port))
    if (!portMatch) {
      item.enabled = false
      continue
    }
    item.action = parsed.action
    if (parsed.action === 'toggle') {
      if (!item.hasBoth) {
        item.enabled = false
        continue
      }
      item.enabled = true
      item.backendIPs = []
    } else {
      const selectable = getSelectableServers(item)
      if (selectable.length === 0) {
        item.enabled = false
        continue
      }
      let idx = parsed.index
      if (idx === -1) idx = selectable.length
      else if (idx > selectable.length) {
        item.enabled = false
        continue
      }
      const target = selectable[idx - 1]
      if (!target) {
        item.enabled = false
        continue
      }
      item.enabled = true
      item.backendIPs = [normalizeIPKey(target)]
    }
    matchedCount++
  }
  return matchedCount
}

function getSyntaxHint(input: string): string {
  const parsed = parseBatchSearchSyntax(input)
  if (!parsed) return ''
  if (parsed.action === 'toggle') {
    const count = rows.value.filter(
      (item) => item.servers.some((s) => parsed.ports.includes(s.port)) && item.hasBoth,
    ).length
    return count > 0 ? `已选中 ${count} 个upstream组，切换全部` : ''
  }
  const actionLabel = parsed.action === 'online' ? '上线' : '下线'
  const indexLabel = parsed.index === -1 ? '最后 1' : `第 ${parsed.index}`
  const count = rows.value.filter((item) => {
    if (!item.servers.some((s) => parsed.ports.includes(s.port))) return false
    return getSelectableServers(item).length > 0
  }).length
  return count > 0 ? `已选中 ${count} 个upstream组，${actionLabel} ${indexLabel} 个服务器` : ''
}

watch(batchSearch, (val) => {
  const trimmed = val.trim()
  if (!trimmed) {
    syntaxHint.value = ''
    isSyntaxMode.value = false
    return
  }
  const parsed = parseBatchSearchSyntax(trimmed)
  if (parsed) {
    isSyntaxMode.value = true
    applyBatchSearchSyntax(parsed)
    syntaxHint.value = getSyntaxHint(trimmed)
  } else if (trimmed.startsWith('{')) {
    // 以 { 开头但格式不对 → 语法错误状态
    isSyntaxMode.value = false
    syntaxHint.value = '语法格式错误'
  } else {
    isSyntaxMode.value = false
    syntaxHint.value = ''
  }
})

// ---------- 全选 / 展开 ----------
const filteredBatchItems = computed<BatchRow[]>(() => {
  const q = batchSearch.value.trim().toLowerCase()
  if (!q) return rows.value
  if (isSyntaxMode.value) return rows.value
  return rows.value.filter((item) => {
    if (item.upstreamName.toLowerCase().includes(q)) return true
    return item.servers.some((s) => s.ip.includes(q) || s.port.includes(q))
  })
})

const isBatchAllSelected = computed(() => {
  const eligible = filteredBatchItems.value.filter((i) => i.hasBoth || i.hasMultipleUp)
  return eligible.length > 0 && eligible.every((i) => i.enabled)
})

function toggleBatchSelectAll(): void {
  const newState = !isBatchAllSelected.value
  filteredBatchItems.value.forEach((i) => {
    if (i.hasBoth || i.hasMultipleUp) {
      i.enabled = newState
    }
  })
}

function toggleBatchExpand(row: BatchRow): void {
  batchTableRef.value?.toggleRowExpansion(row)
}

function toggleBatchExpandAll(): void {
  const newState = !batchAllExpanded.value
  filteredBatchItems.value.forEach((row) => {
    batchTableRef.value?.toggleRowExpansion(row, newState)
  })
  batchAllExpanded.value = newState
}

// ---------- 提交 ----------
const batchValidCount = computed(() =>
  rows.value
    .filter((i) => {
      if (!i.enabled || !i.action) return false
      if (i.action === 'toggle') return true
      return i.backendIPs.length > 0
    })
    .reduce((sum, i) => (i.action === 'toggle' ? sum + 1 : sum + i.backendIPs.length), 0),
)

function confirm(): void {
  // 下线安全校验：每个组至少保留 1 台在线服务器
  for (const i of rows.value) {
    if (!i.enabled || i.action !== 'offline') continue
    const upServers = i.servers.filter((s) => s.status === 'up')
    if (i.backendIPs.length >= upServers.length) {
      ElMessage.warning(`upstream [${i.upstreamName}] 中所有在线服务器都将被下线，至少需要保留一台在线服务器`)
      return
    }
  }
  const items: NginxBatchItem[] = []
  for (const i of rows.value) {
    if (!i.enabled || !i.action) continue
    if (i.action === 'toggle') {
      items.push({ upstream_name: i.upstreamName, action: 'toggle', backend_ip: '' })
    } else {
      for (const ip of i.backendIPs) {
        items.push({ upstream_name: i.upstreamName, action: i.action, backend_ip: ip })
      }
    }
  }
  if (items.length === 0) {
    ElMessage.warning('请至少配置一个操作')
    return
  }
  emit('confirm', items)
  visible.value = false
}
</script>

<template>
  <el-dialog v-model="visible" width="min(800px, 90vw)" align-center append-to-body>
    <template #header>
      <div class="batch-dialog-header">
        <span class="batch-dialog-title">{{ t('nginx.batch') }}</span>
        <span class="batch-hint-text">为每个 Upstream 组选择操作类型，支持上线、下线、切换（反转全部状态）混合操作</span>
      </div>
    </template>

    <div class="batch-hint">
      <el-button size="small" @click="toggleBatchSelectAll">{{ isBatchAllSelected ? '取消' : '全选' }}</el-button>
      <el-button size="small" @click="toggleBatchExpandAll">{{ batchAllExpanded ? '折叠' : '展开' }}</el-button>
      <div class="batch-search-wrap">
        <el-input
          v-model="batchSearch"
          placeholder="搜索 IP / 端口 或 {端口}{上线|下线|切换|索引}"
          size="small"
          clearable
          style="width: 260px"
        />
        <span v-if="syntaxHint" class="batch-syntax-hint" :class="{ 'hint-error': syntaxHint === '语法格式错误' }">
          {{ syntaxHint }}
        </span>
      </div>
    </div>

    <el-table
      ref="batchTableRef"
      :data="filteredBatchItems"
      size="small"
      max-height="500"
      class="batch-table"
      row-key="upstreamName"
    >
      <el-table-column type="expand" width="1">
        <template #default="{ row }">
          <div class="batch-expand-servers">
            <div v-for="s in row.servers" :key="serverKey(s)" class="batch-server-item">
              <span class="status-dot" :class="s.status === 'up' ? 'status-up' : 'status-down'" />
              <span class="batch-server-ip">{{ s.ip }}</span>
              <span class="batch-server-port">:{{ s.port }}</span>
              <span v-if="s.weight" class="batch-server-weight">w={{ s.weight }}</span>
            </div>
          </div>
        </template>
      </el-table-column>

      <el-table-column :label="t('common.enabled')" width="60" align="center">
        <template #default="{ row }">
          <el-checkbox v-model="row.enabled" :disabled="!row.hasBoth && !row.hasMultipleUp" />
        </template>
      </el-table-column>

      <el-table-column :label="t('nginx.upstream')" min-width="150" sortable :sort-method="(a, b) => a.upstreamName.localeCompare(b.upstreamName)">
        <template #default="{ row }">
          <span class="batch-upstream-name" @mousedown.prevent @click="toggleBatchExpand(row as BatchRow)">{{ row.upstreamName }}</span>
        </template>
      </el-table-column>

      <el-table-column :label="t('logs.status')" width="130" sortable :sort-method="(a, b) => a.upCount - b.upCount">
        <template #default="{ row }">
          <span class="badge badge-success">{{ row.upCount }} up</span>
          <span class="badge badge-danger">{{ row.downCount }} down</span>
        </template>
      </el-table-column>

      <el-table-column :label="t('nginx.batchItem.action')" width="180">
        <template #default="{ row }">
          <el-select
            v-model="row.action"
            placeholder="选择操作"
            size="small"
            :disabled="!row.enabled"
            style="width: 100%"
            @change="onBatchActionChange(row as BatchRow)"
          >
            <el-option v-for="a in getAvailableActions(row as BatchRow)" :key="a.value" :label="a.label" :value="a.value" />
          </el-select>
        </template>
      </el-table-column>

      <el-table-column :label="t('nginx.backendIp')" min-width="200">
        <template #default="{ row }">
          <template v-if="row.action === 'online' || row.action === 'offline'">
            <el-select
              v-model="row.backendIPs"
              placeholder="选择服务器（可多选）"
              size="small"
              multiple
              collapse-tags
              collapse-tags-tooltip
              style="width: 100%"
            >
              <template #header>
                <el-button size="small" type="primary" text style="padding: 0 4px; font-size: 12px" @click.stop="toggleBatchIPSelectAll(row as BatchRow)">
                  {{ isBatchIPAllSelected(row as BatchRow) ? '取消全选' : '全选' }}
                </el-button>
              </template>
              <el-option
                v-for="s in getSelectableServers(row as BatchRow)"
                :key="serverKey(s)"
                :label="serverKey(s)"
                :value="normalizeIPKey(s)"
              />
            </el-select>
            <div v-if="row.action === 'offline'" class="batch-offline-hint">至少保留 1 台在线服务器</div>
          </template>
          <span v-else-if="row.action === 'toggle'" class="batch-toggle-hint">全部反转</span>
        </template>
      </el-table-column>
    </el-table>

    <div v-if="filteredBatchItems.length === 0" class="batch-empty">暂无可执行的批量操作</div>

    <template #footer>
      <el-button @click="visible = false">{{ t('common.cancel') }}</el-button>
      <el-button type="primary" :disabled="batchValidCount === 0" @click="confirm">
        {{ t('common.preview') }}（{{ batchValidCount }} 项）
      </el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.batch-dialog-header {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  flex-wrap: wrap;
}

.batch-dialog-title {
  font-size: var(--text-base);
  font-weight: 600;
}

.batch-hint-text {
  font-size: var(--text-sm);
  color: var(--text-secondary);
}

.batch-hint {
  display: flex;
  align-items: center;
  font-size: var(--text-sm);
  color: var(--text-secondary);
  margin-bottom: var(--space-3);
  padding: var(--space-2) var(--space-3);
  background: var(--bg-deep);
  border-radius: var(--radius-sm);
}

.batch-search-wrap {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  margin-left: var(--space-3);
}

.batch-syntax-hint {
  font-size: var(--text-xs);
  color: var(--emerald-400);
  white-space: nowrap;
}

.batch-syntax-hint.hint-error {
  color: var(--rose-400);
}

.batch-table {
  width: 100%;
}

.batch-empty {
  text-align: center;
  color: var(--text-secondary);
  padding: var(--space-6) 0;
  font-size: var(--text-sm);
}

.batch-offline-hint {
  font-size: var(--text-xs);
  color: var(--amber-400);
  margin-top: 2px;
  line-height: 1.2;
}

.batch-toggle-hint {
  color: var(--text-secondary);
  font-size: var(--text-xs);
}

.batch-upstream-name {
  color: var(--sky-400);
  cursor: pointer;
  font-weight: 500;
}

.batch-upstream-name:hover {
  text-decoration: underline;
}

.batch-expand-servers {
  padding: var(--space-2) var(--space-3) var(--space-2) 76px;
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-3);
}

.batch-server-item {
  display: inline-flex;
  align-items: center;
  font-size: var(--text-xs);
}

.batch-server-ip {
  font-family: var(--font-mono);
}

.batch-server-weight {
  color: var(--text-secondary);
  margin-left: var(--space-2);
}

.badge {
  display: inline-flex;
  align-items: center;
  padding: 0 8px;
  border-radius: 8px;
  font-size: 11px;
  font-weight: 500;
  line-height: 1.4;
  margin-right: 4px;
}

.badge-success {
  background: rgba(34, 197, 94, 0.12);
  color: #22c55e;
}

.badge-danger {
  background: rgba(239, 68, 68, 0.12);
  color: #ef4444;
}

.status-dot {
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  margin-right: 6px;
  vertical-align: middle;
}

.status-up {
  background: #22c55e;
}

.status-down {
  background: #ef4444;
}
</style>
