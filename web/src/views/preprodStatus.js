/**
 * 预生产资源状态判定（设计文档：docs/superpowers/specs/2026-09-10-preprod-status-design.md）。
 * 规则按优先级排列，命中即返回；字段含义见 internal/service/preprod.go 的 PreprodResource。
 *
 * 返回 { label, tagType, title? }，title 为可选悬浮提示文案（仅"异常"状态有）。
 */
export function resolveStatus(row) {
  const { current, ready, ready_desired, target_replicas, category, up_to_date } = row

  if (current === 0 && ready === 0 && ready_desired === 0) {
    return { label: '已缩容', tagType: 'info' }
  }
  if (target_replicas === 0) {
    return { label: '缩容中', tagType: 'primary' }
  }
  if (current > target_replicas) {
    // 发布 surge：新版本已就绪、多余旧 Pod 待回收（如 3 > 2，就绪 3/2）
    return { label: '发布中', tagType: 'warning' }
  }
  if (current < target_replicas) {
    return { label: '扩容中', tagType: 'primary' }
  }
  // 以下 current === target_replicas
  if (ready < ready_desired) {
    if (category === 'rollout') {
      // UP-TO-DATE 未追平目标：新版本尚未替换完（发布中）；已追平：版本一致、Pod 重建中（恢复中）
      return up_to_date < ready_desired
        ? { label: '发布中', tagType: 'warning' }
        : { label: '恢复中', tagType: 'warning' }
    }
    // deployment/statefulset 的 UP-TO-DATE 列可能缺列（为 0），不做发布/恢复细分
    return { label: '启动中', tagType: 'warning' }
  }
  if (ready === ready_desired) {
    return { label: '正常', tagType: 'success' }
  }
  // ready > ready_desired：就绪数超过目标数，数据不一致
  return { label: '异常', tagType: 'danger', title: '副本数据不一致，请检查控制器状态' }
}
