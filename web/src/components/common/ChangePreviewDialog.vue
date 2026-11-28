<template>
  <el-dialog
    :model-value="modelValue"
    :title="title"
    :width="dialogWidth"
    align-center
    class="change-preview-dialog"
    append-to-body
    destroy-on-close
    @update:model-value="$emit('update:modelValue', $event)"
  >
    <div v-if="description" class="cp-desc">{{ description }}</div>
    <slot></slot>

    <!-- 确认门槛：风险未消除或数量超过阈值时，要求手输确认文字，避免误点 -->
    <div v-if="gateText" class="cp-gate">
      <p class="cp-gate-hint">
        <span class="cp-gate-reason">{{ gateReason }}</span>
        <span>请输入 <b>{{ confirmText }}</b> 以继续：</span>
      </p>
      <input v-model="gateInput" class="cp-gate-input" :placeholder="confirmText" @keyup.enter="tryConfirm" />
    </div>

    <template #footer>
      <slot name="footer">
        <el-button @click="$emit('update:modelValue', false)">取消</el-button>
        <el-button type="primary" :loading="executing" :disabled="confirmDisabled" @click="tryConfirm">
          确认执行
        </el-button>
      </slot>
    </template>
  </el-dialog>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
/**
 * 变更预览统一弹窗。
 *
 * 四个业务模块（Nginx / LVS / K8s / 预生产）原本各写一套 el-dialog，
 * 宽度从 90% 到 600/650/700/800px 不等、高度全不固定，观感割裂且长内容会把
 * 「确认执行」挤出屏幕。这里统一成固定尺寸：内容区独立滚动，头尾始终固定。
 */
const props = defineProps({
  modelValue: { type: Boolean, default: false },
  title: { type: String, default: '变更预览' },
  /** 预览描述，展示在内容区顶部的高亮块；不需要可传空串 */
  description: { type: String, default: '' },
  /** 执行中：确认按钮显示 loading */
  executing: { type: Boolean, default: false },
  /** 是否要求手输确认文字才能执行（风险/大批量场景） */
  requireGate: { type: Boolean, default: false },
  /** 需要门槛的原因说明，如「本次涉及 12 项（超过 10 项阈值）」 */
  gateReason: { type: String, default: '' },
  /** 需输入的文字 */
  confirmText: { type: String, default: '确认执行' },
})

const emit = defineEmits(['update:modelValue', 'confirm'])

// 尺寸集中在这里定义，改一处即可同步四个模块
const dialogWidth = 'min(720px, 92vw)'

const gateInput = ref('')

// 门槛生效时才有输入框；输入匹配才允许确认
const gateText = computed(() => props.requireGate)
const confirmDisabled = computed(() => props.requireGate && gateInput.value !== props.confirmText)

// 每次打开清空输入，避免上次输入残留直接放行
watch(
  () => props.modelValue,
  (open) => {
    if (open) gateInput.value = ''
  }
)

function tryConfirm() {
  if (confirmDisabled.value) return
  emit('confirm')
}
</script>

<!-- 非 scoped：需要穿透 el-dialog 内部结构才能固定高度并让内容区滚动 -->
<style>
.change-preview-dialog.el-dialog {
  display: flex;
  flex-direction: column;
  height: min(640px, 78vh);
  padding: 0;
  overflow: hidden;
  /* 不写 margin: 0 —— align-center 依赖 overlay 的 flex 居中，保留自动外边距 */
}

.change-preview-dialog .el-dialog__header {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 0;
  padding: 16px 20px;
}

.change-preview-dialog .el-dialog__title {
  font-size: 16px;
  font-weight: 600;
  display: flex;
  align-items: center;
  gap: 8px;
}

/* 标题前的青色竖条，与 Demo 一致 */
.change-preview-dialog .el-dialog__title::before {
  content: '';
  width: 3px;
  height: 14px;
  border-radius: 2px;
  background: var(--color-primary, #06b6d4);
}

.change-preview-dialog .el-dialog__headerbtn {
  position: static;
  width: 26px;
  height: 26px;
  border-radius: 6px;
  transition:
    color 0.2s,
    background 0.2s;
}

.change-preview-dialog .el-dialog__headerbtn:hover {
  background: var(--color-primary-bg, rgba(6, 182, 212, 0.1));
}

.change-preview-dialog .el-dialog__headerbtn:hover .el-dialog__close {
  color: var(--color-primary, #06b6d4);
}

/* 内容区：占满剩余空间并独立滚动，长配置不会撑高弹窗 */
.change-preview-dialog .el-dialog__body {
  flex: 1 1 auto;
  min-height: 0;
  overflow-y: auto;
  padding: 16px 20px;
}

.change-preview-dialog .el-dialog__footer {
  flex-shrink: 0;
  padding: 12px 20px;
}

/* ---------- 内容块统一样式（供四个模块的插槽内容使用） ---------- */

.change-preview-dialog .cp-desc {
  font-size: 13px;
  line-height: 1.6;
  color: var(--text-primary);
  margin-bottom: 14px;
  padding: 10px 14px;
  background: var(--bg-elevated);
  border-radius: 8px;
  border-left: 3px solid var(--color-primary, #06b6d4);
}

.change-preview-dialog .cp-label {
  font-size: 12px;
  font-weight: 500;
  color: var(--text-secondary);
  margin: 0 0 8px;
}

.change-preview-dialog .cp-label:not(:first-child) {
  margin-top: 16px;
}

.change-preview-dialog .cp-divider {
  height: 1px;
  background: var(--border-default);
  margin: 14px 0;
}

.change-preview-dialog .cp-cmds {
  margin: 0;
  padding: 0 0 0 18px;
  color: var(--text-regular);
  font-size: 13px;
  line-height: 1.9;
}

.change-preview-dialog .cp-cmds code,
.change-preview-dialog .cp-inline-code {
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  font-size: 12.5px;
  color: var(--color-primary, #06b6d4);
  background: var(--color-primary-bg, rgba(6, 182, 212, 0.1));
  padding: 1px 5px;
  border-radius: 4px;
  word-break: break-all;
}

/* 表格（LVS 批量预览） */
.change-preview-dialog .cp-table-scroll {
  max-height: 100%;
  overflow: auto;
  border: 1px solid var(--border-default);
  border-radius: 8px;
}

.change-preview-dialog .cp-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 12.5px;
}

.change-preview-dialog .cp-table th {
  position: sticky;
  top: 0;
  text-align: left;
  padding: 8px 10px;
  font-weight: 600;
  font-size: 12px;
  color: var(--text-secondary);
  background: var(--bg-elevated);
  border-bottom: 1px solid var(--border-default);
}

.change-preview-dialog .cp-table td {
  padding: 8px 10px;
  border-bottom: 1px solid var(--border-default);
  color: var(--text-regular);
  vertical-align: top;
}

.change-preview-dialog .cp-table tr:last-child td {
  border-bottom: none;
}

.change-preview-dialog .cp-table tbody tr:hover td {
  background: var(--color-primary-bg, rgba(6, 182, 212, 0.05));
}

.change-preview-dialog .cp-table code {
  font-family: 'JetBrains Mono', monospace;
  font-size: 11.5px;
  color: var(--color-primary, #06b6d4);
  word-break: break-all;
}

/* 预览内的 pre 限高，避免与外层滚动条叠加 */
.change-preview-dialog .cp-pre {
  margin: 0;
  max-height: 220px;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-all;
  background: var(--terminal-bg);
  color: var(--terminal-text);
  padding: 10px;
  border-radius: 6px;
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  font-size: 12.5px;
  line-height: 1.6;
  border: 1px solid rgba(255, 255, 255, 0.04);
  box-shadow:
    inset 0 1px 3px rgba(0, 0, 0, 0.3),
    inset 0 0 0 1px rgba(255, 255, 255, 0.03);
}

/* ---------- 风险提示条 ---------- */
.change-preview-dialog .cp-risk,
.change-preview-dialog .cp-risk-ok {
  display: flex;
  gap: 8px;
  align-items: flex-start;
  padding: 9px 12px;
  border-radius: 8px;
  font-size: 12.5px;
  line-height: 1.6;
  margin-bottom: 14px;
}

.change-preview-dialog .cp-risk {
  color: var(--color-warning);
  background: rgba(245, 158, 11, 0.1);
  border: 1px solid rgba(245, 158, 11, 0.3);
}

.change-preview-dialog .cp-risk-ok {
  color: var(--color-success);
  background: rgba(34, 197, 94, 0.08);
  border: 1px solid rgba(34, 197, 94, 0.28);
}

.change-preview-dialog .cp-risk b {
  color: var(--color-warning);
}

/* 被跳过（已缩容/已扩容）的资源行 */
.change-preview-dialog .cp-table tr.cp-row-skip td {
  color: var(--text-placeholder);
  text-decoration: line-through;
}

/* ---------- 确认门槛 ---------- */
.change-preview-dialog .cp-gate {
  margin-top: 14px;
  padding: 12px;
  border-radius: 8px;
  background: var(--bg-card);
  border: 1px dashed var(--border-strong);
}

.change-preview-dialog .cp-gate-hint {
  margin: 0 0 8px;
  font-size: 12.5px;
  color: var(--text-secondary);
  line-height: 1.6;
}

.change-preview-dialog .cp-gate-hint b {
  color: var(--color-danger);
}

.change-preview-dialog .cp-gate-reason {
  display: block;
  margin-bottom: 4px;
  color: var(--color-warning);
}

.change-preview-dialog .cp-gate-input {
  width: 100%;
  padding: 7px 10px;
  border-radius: 6px;
  border: 1px solid var(--border-strong);
  background: var(--bg-elevated);
  color: var(--text-primary);
  font-size: 13px;
  font-family: inherit;
}

.change-preview-dialog .cp-gate-input:focus {
  outline: none;
  border-color: var(--color-primary, #06b6d4);
}
</style>
