<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { lvsApi, preprodApi, extractErrorMessage } from '@/api'
import type { CommandExecuteResult, VirtualServer, LvsRealServer } from '@/api/types'
import { useAuthStore } from '@/stores/auth'
import { usePreviewExecute } from '@/composables/usePreviewExecute'
import { i18n } from '@/i18n'
import ServerSelect from '@/components/ServerSelect.vue'
import PreviewDialog from '@/components/PreviewDialog.vue'
import OutputDialog from '@/components/OutputDialog.vue'
import StatusDialog from './StatusDialog.vue'
import SwapDialog from './SwapDialog.vue'
import TagDialog from './TagDialog.vue'
import BindingsDialog from './BindingsDialog.vue'

const t = i18n.global.t
const auth = useAuthStore()

// ---------- 数据加载 ----------
const serverSelectRef = ref<InstanceType<typeof ServerSelect>>()
const serverId = ref<number>()
const vsList = ref<VirtualServer[]>([])
const listLoading = ref(false)
const keyword = ref('')

async function loadList(): Promise<void> {
  if (!serverId.value) {
    vsList.value = []
    return
  }
  listLoading.value = true
  try {
    vsList.value = await lvsApi.list(serverId.value)
  } catch (err) {
    ElMessage.error(extractErrorMessage(err))
  } finally {
    listLoading.value = false
  }
}

watch(serverId, loadList)

const filteredVs = computed(() => {
  const kw = keyword.value.trim().toLowerCase()
  if (!kw) return vsList.value
  return vsList.value.filter(
    (vs) =>
      vs.ip.includes(kw) ||
      (vs.tag ?? '').toLowerCase().includes(kw) ||
      vs.real_servers.some((rs) => rs.ip.includes(kw) || (rs.tag ?? '').toLowerCase().includes(kw)),
  )
})

const rsCount = (vs: VirtualServer): string =>
  `${vs.real_servers.filter((r) => r.status === 'up').length}/${vs.real_servers.length}`

// ---------- 预览 → 执行（op / swap 共用） ----------
const previewVisible = ref(false)
const outputVisible = ref(false)
const outputResult = ref<string | string[]>('')
const outputStatus = ref<'success' | 'failed'>('success')

interface PreviewShape {
  preview_id: string
  current_status: string
  command: string
  description: string
}

const pe = usePreviewExecute<PreviewShape>()

interface PendingOp {
  kind: 'op' | 'swap'
  vs: VirtualServer
  rs?: LvsRealServer
  state?: 'on' | 'off'
  swap?: { rs_ip1: string; rs_ip2: string }
}

const pending = ref<PendingOp | null>(null)

function openOutput(res: CommandExecuteResult, ok: boolean): void {
  outputResult.value = res.output
  outputStatus.value = ok ? 'success' : 'failed'
  outputVisible.value = true
}

async function requestOp(vs: VirtualServer, rs: LvsRealServer, state: 'on' | 'off'): Promise<void> {
  if (rs.disabled) {
    ElMessage.warning(`${t('lvs.disabledWarn')}${rs.disabled_reason ? `: ${rs.disabled_reason}` : ''}`)
    return
  }
  if (!serverId.value) return

  // 上线前检查预生产资源副本（need_warning 时人工确认）
  if (state === 'on') {
    try {
      const check = await preprodApi.checkLvsOnline({ vs_ip: vs.ip, rs_ip: rs.ip })
      if (check.need_warning && check.warnings?.length) {
        const lines = check.warnings
          .map((w) => `${w.name} (${w.category}): ${w.current} → ${t('lvs.targetReplicas')} ${w.target}`)
          .join('\n')
        await ElMessageBox.confirm(lines, t('common.confirm'), { type: 'warning' })
      }
    } catch {
      return // 用户取消确认框
    }
  }

  pending.value = { kind: 'op', vs, rs, state }
  const sid = serverId.value
  const ok = await pe.preview(() => lvsApi.opPreview({ server_id: sid, vs_ip: vs.ip, rs_ip: rs.ip, state }))
  if (ok) previewVisible.value = true
}

function requestSwap(vs: VirtualServer, payload: { rs_ip1: string; rs_ip2: string }): void {
  if (!serverId.value) return
  pending.value = { kind: 'swap', vs, swap: payload }
  const sid = serverId.value
  void pe
    .preview(() =>
      lvsApi.swapPreview({ server_id: sid, vs_ip: vs.ip, rs_ip1: payload.rs_ip1, rs_ip2: payload.rs_ip2 }),
    )
    .then((ok) => {
      if (ok) previewVisible.value = true
    })
}

async function executePreview(): Promise<void> {
  const p = pending.value
  if (!p) return
  const result =
    p.kind === 'op'
      ? await pe.execute((id) => lvsApi.opExecute({ preview_id: id }))
      : await pe.execute((id) => lvsApi.swapExecute({ preview_id: id }))

  if (result.ok && result.result) {
    previewVisible.value = false
    openOutput(result.result, true)
    ElMessage.success(t('common.execSuccess'))
    pe.reset()
    void loadList()
  } else if (result.error) {
    if (pe.expired.value) {
      ElMessage.warning(result.error)
    } else {
      ElMessage.error(result.error)
    }
  }
}

function reproview(): void {
  const p = pending.value
  if (!p) return
  previewVisible.value = false
  pe.reset()
  if (p.kind === 'op' && p.rs && p.state) {
    void requestOp(p.vs, p.rs, p.state)
  } else if (p.kind === 'swap' && p.swap) {
    requestSwap(p.vs, p.swap)
  }
}

// ---------- 状态 / 切换 / 标签 / 绑定 ----------
const statusDialogRef = ref<InstanceType<typeof StatusDialog>>()
const swapDialogRef = ref<InstanceType<typeof SwapDialog>>()
const tagDialogRef = ref<InstanceType<typeof TagDialog>>()
const bindingsVisible = ref(false)
const swapVs = ref<VirtualServer | null>(null)
const tagMode = ref<'rs' | 'vs'>('rs')

function openSwap(vs: VirtualServer): void {
  swapVs.value = vs
  swapDialogRef.value?.open()
}

function openRsTag(vs: VirtualServer, rs: LvsRealServer): void {
  tagMode.value = 'rs'
  tagDialogRef.value?.open({
    rs_ip: rs.ip,
    vs_ip: vs.ip,
    tag: rs.tag ?? '',
    disabled: rs.disabled ?? false,
    disabled_reason: rs.disabled_reason ?? '',
  })
}

function openVsTag(vs: VirtualServer): void {
  tagMode.value = 'vs'
  tagDialogRef.value?.open({ vs_ip: vs.ip, tag: vs.tag ?? '' })
}
</script>

<template>
  <div class="page">
    <div class="page-head">
      <div>
        <h1 class="page-title grad-text">{{ t('lvs.title') }}</h1>
        <p class="page-subtitle">{{ t('lvs.subtitle') }}</p>
      </div>
      <div class="page-actions head-actions">
        <ServerSelect ref="serverSelectRef" v-model:server-id="serverId" type="lvs" />
        <el-input v-model="keyword" :placeholder="t('logs.keyword')" clearable style="width: 180px" />
        <el-button :loading="listLoading" :disabled="!serverId" @click="loadList">
          {{ t('common.refresh') }}
        </el-button>
        <el-button :disabled="!serverId" @click="statusDialogRef?.open(serverId!)">
          {{ t('lvs.status') }}
        </el-button>
        <el-button v-if="auth.isAdmin" @click="bindingsVisible = true">{{ t('lvs.bindings') }}</el-button>
      </div>
    </div>

    <div v-loading="listLoading" class="card table-card reveal d-1">
      <el-empty v-if="!serverId" :description="t('common.serverPlaceholder')" />
      <el-empty v-else-if="filteredVs.length === 0 && !listLoading" :description="t('common.empty')" />
      <el-table v-else :data="filteredVs" row-key="ip">
        <el-table-column type="expand">
          <template #default="{ row }">
            <div class="rs-wrap">
              <el-table :data="row.real_servers" size="small">
                <el-table-column :label="`RS : ${t('lvs.port')}`" min-width="170">
                  <template #default="{ row: rs }">
                    <span class="mono">{{ rs.ip }}:{{ rs.port }}</span>
                  </template>
                </el-table-column>
                <el-table-column :label="t('logs.status')" width="100">
                  <template #default="{ row: rs }">
                    <span class="dot-live" :class="{ 'is-offline': rs.status !== 'up' }" />
                    <span class="status-text" :class="rs.status === 'up' ? 'ok' : 'bad'">
                      {{ rs.status === 'up' ? t('common.online') : t('common.offline') }}
                    </span>
                  </template>
                </el-table-column>
                <el-table-column prop="weight" :label="t('lvs.weight')" width="80" />
                <el-table-column prop="active_conn" :label="t('lvs.activeConn')" width="100" sortable />
                <el-table-column prop="inact_conn" :label="t('lvs.inactConn')" width="110" sortable />
                <el-table-column :label="t('lvs.tag')" min-width="140">
                  <template #default="{ row: rs }">
                    <el-tag v-if="rs.tag" size="small" type="info" effect="plain" class="mr">{{
                      rs.tag
                    }}</el-tag>
                    <el-tooltip v-if="rs.disabled" :content="rs.disabled_reason" placement="top">
                      <el-tag size="small" type="danger" effect="plain">{{ t('common.disabled') }}</el-tag>
                    </el-tooltip>
                  </template>
                </el-table-column>
                <el-table-column :label="t('common.operation')" width="220" fixed="right">
                  <template #default="{ row: rs }">
                    <el-button
                      link
                      type="success"
                      size="small"
                      :disabled="rs.status === 'up' || Boolean(rs.disabled)"
                      @click="requestOp(row as VirtualServer, rs as LvsRealServer, 'on')"
                    >
                      {{ t('lvs.online') }}
                    </el-button>
                    <el-button
                      link
                      type="danger"
                      size="small"
                      :disabled="rs.status !== 'up' || Boolean(rs.disabled)"
                      @click="requestOp(row as VirtualServer, rs as LvsRealServer, 'off')"
                    >
                      {{ t('lvs.offline') }}
                    </el-button>
                    <el-button link size="small" @click="openSwap(row as VirtualServer)">{{ t('lvs.swap') }}</el-button>
                    <el-button v-if="auth.isAdmin" link size="small" @click="openRsTag(row as VirtualServer, rs as LvsRealServer)">
                      {{ t('common.edit') }}
                    </el-button>
                  </template>
                </el-table-column>
              </el-table>
            </div>
          </template>
        </el-table-column>

        <el-table-column label="VS : Port" min-width="160">
          <template #default="{ row }">
            <span class="mono vs-ip">{{ row.ip }}:{{ row.port }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="protocol" :label="t('lvs.protocol')" width="90" />
        <el-table-column prop="scheduler" :label="t('lvs.scheduler')" width="100" />
        <el-table-column :label="t('lvs.role')" width="80">
          <template #default="{ row }">
            <el-tag v-if="row.role" size="small" :type="row.role === 'master' ? 'primary' : 'info'" effect="plain">
              {{ row.role === 'master' ? t('lvs.master') : t('lvs.backup') }}
            </el-tag>
            <span v-else>-</span>
          </template>
        </el-table-column>
        <el-table-column :label="t('lvs.tag')" min-width="140">
          <template #default="{ row }">
            <el-tag v-if="row.tag" size="small" effect="plain" class="mr">{{ row.tag }}</el-tag>
            <el-button v-if="auth.isAdmin" link size="small" @click="openVsTag(row as VirtualServer)">
              {{ row.tag ? t('common.edit') : t('common.add') }}
            </el-button>
          </template>
        </el-table-column>
        <el-table-column :label="`RS (${t('common.online')})`" width="120">
          <template #default="{ row }">
            <span class="mono">{{ rsCount(row as VirtualServer) }}</span>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <!-- 预览执行 -->
    <PreviewDialog
      v-model:visible="previewVisible"
      :description="pe.previewData.value?.description ?? ''"
      :current-status="pe.previewData.value?.current_status ?? ''"
      :commands="pe.previewData.value ? [pe.previewData.value.command] : []"
      :countdown="pe.countdown.value"
      :expired="pe.expired.value"
      :executing="pe.executing.value"
      @execute="executePreview"
      @repreview="reproview"
    />

    <OutputDialog v-model:visible="outputVisible" :output="outputResult" :status="outputStatus" />

    <!-- 状态 / 切换 / 标签 / 绑定 -->
    <StatusDialog ref="statusDialogRef" />
    <SwapDialog ref="swapDialogRef" :vs="swapVs" @confirm="requestSwap(swapVs!, $event)" />
    <TagDialog ref="tagDialogRef" :mode="tagMode" @saved="loadList" />
    <BindingsDialog v-model:visible="bindingsVisible" @changed="loadList" />
  </div>
</template>

<style scoped>
.head-actions {
  flex-wrap: wrap;
}

.table-card {
  padding: var(--space-3);
  min-height: 200px;
}

.vs-table :deep(.el-table__expanded-cell) {
  padding: 0;
}

.rs-wrap {
  padding: var(--space-3) var(--space-6);
  background: var(--bg-deep);
}

.status-text {
  margin-left: var(--space-2);
  font-size: var(--text-xs);
}

.status-text.ok {
  color: var(--emerald-400);
}

.status-text.bad {
  color: var(--rose-400);
}

.vs-ip {
  font-weight: 600;
}

.mr {
  margin-right: var(--space-2);
}
</style>
