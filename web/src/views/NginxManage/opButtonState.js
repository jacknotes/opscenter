/**
 * 上线 / 下线按钮的可选状态判定。
 *
 * 只判断「有没有选中」是不够的：全选已 up 的主机时上线毫无意义，全选已 down 的主机时下线也没有意义。
 * 按钮应当提前禁用，而不是等用户点了再弹提示。
 *
 * 混合选中（既有 up 又有 down）时两个按钮都可用 —— 执行时会各自过滤出目标状态的子集。
 *
 * @param {{ status: string }[]} selected 选中的主机列表
 * @returns {{ onlineDisabled: boolean, offlineDisabled: boolean }}
 */
export function resolveOpButtons(selected) {
  const list = Array.isArray(selected) ? selected : []
  if (list.length === 0) {
    return { onlineDisabled: true, offlineDisabled: true }
  }
  return {
    onlineDisabled: list.every((s) => s.status === 'up'),
    offlineDisabled: list.every((s) => s.status === 'down'),
  }
}
