import { describe, it, expect } from 'vitest'
import { resolveStatus } from '../preprodStatus'

describe('resolveStatus', () => {
  it('规则1: current/ready/ready_desired 全为 0 → 已缩容', () => {
    expect(resolveStatus({ current: 0, ready: 0, ready_desired: 0, target_replicas: 0 })).toEqual({
      label: '已缩容',
      tagType: 'info',
    })
  })

  it('规则2: 目标副本为 0 但 Pod 未退完 → 缩容中', () => {
    expect(resolveStatus({ current: 2, ready: 1, ready_desired: 0, target_replicas: 0 })).toEqual({
      label: '缩容中',
      tagType: 'primary',
    })
  })

  it('规则3: current > target（发布 surge 回收旧 Pod）→ 发布中', () => {
    // 用户截图场景：当前 3 > 目标 2，就绪 3/2
    expect(
      resolveStatus({ current: 3, ready: 3, ready_desired: 2, target_replicas: 2, category: 'rollout', up_to_date: 2 })
    ).toEqual({ label: '发布中', tagType: 'warning' })
  })

  it('规则4: current < target → 扩容中', () => {
    expect(
      resolveStatus({ current: 1, ready: 1, ready_desired: 2, target_replicas: 2, category: 'rollout', up_to_date: 1 })
    ).toEqual({ label: '扩容中', tagType: 'primary' })
  })

  it('规则5a: rollout 副本到位但 up_to_date 未满 → 发布中', () => {
    expect(
      resolveStatus({ current: 2, ready: 1, ready_desired: 2, target_replicas: 2, category: 'rollout', up_to_date: 1 })
    ).toEqual({ label: '发布中', tagType: 'warning' })
  })

  it('规则5b: rollout 副本到位、版本已全是新版但未就绪 → 恢复中', () => {
    expect(
      resolveStatus({ current: 2, ready: 1, ready_desired: 2, target_replicas: 2, category: 'rollout', up_to_date: 2 })
    ).toEqual({ label: '恢复中', tagType: 'warning' })
  })

  it('规则5c: 非 rollout 类型就绪不满 → 启动中（不依赖 up_to_date）', () => {
    expect(
      resolveStatus({
        current: 2,
        ready: 1,
        ready_desired: 2,
        target_replicas: 2,
        category: 'deployment',
        up_to_date: 0,
      })
    ).toEqual({ label: '启动中', tagType: 'warning' })
  })

  it('规则6: 副本到位且全部就绪 → 正常', () => {
    expect(
      resolveStatus({ current: 2, ready: 2, ready_desired: 2, target_replicas: 2, category: 'rollout', up_to_date: 2 })
    ).toEqual({ label: '正常', tagType: 'success' })
  })

  it('规则7: 就绪数超过目标就绪数等数据不一致 → 异常（带提示）', () => {
    expect(
      resolveStatus({ current: 2, ready: 3, ready_desired: 2, target_replicas: 2, category: 'rollout', up_to_date: 2 })
    ).toEqual({
      label: '异常',
      tagType: 'danger',
      title: '副本数据不一致，请检查控制器状态',
    })
  })

  it('边界: rollout 缺 up_to_date 字段且就绪不满 → 恢复中（0 >= ready_desired 不成立时视为版本已一致）', () => {
    expect(resolveStatus({ current: 2, ready: 1, ready_desired: 2, target_replicas: 2, category: 'rollout' })).toEqual({
      label: '恢复中',
      tagType: 'warning',
    })
  })
})
