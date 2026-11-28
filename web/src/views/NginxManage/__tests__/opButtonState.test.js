import { describe, it, expect } from 'vitest'
import { resolveOpButtons } from '../opButtonState'

describe('resolveOpButtons', () => {
  it('未选中任何主机时两个按钮都禁用', () => {
    expect(resolveOpButtons([])).toEqual({ onlineDisabled: true, offlineDisabled: true })
  })

  it('非数组入参（null / undefined）按未选中处理', () => {
    expect(resolveOpButtons(null)).toEqual({ onlineDisabled: true, offlineDisabled: true })
    expect(resolveOpButtons(undefined)).toEqual({ onlineDisabled: true, offlineDisabled: true })
  })

  it('全部已 up：上线禁用，下线可点', () => {
    expect(resolveOpButtons([{ status: 'up' }, { status: 'up' }, { status: 'up' }])).toEqual({
      onlineDisabled: true,
      offlineDisabled: false,
    })
  })

  it('全部已 down：下线禁用，上线可点', () => {
    expect(resolveOpButtons([{ status: 'down' }, { status: 'down' }])).toEqual({
      onlineDisabled: false,
      offlineDisabled: true,
    })
  })

  it('up + down 混合：两个都可点', () => {
    expect(resolveOpButtons([{ status: 'up' }, { status: 'down' }])).toEqual({
      onlineDisabled: false,
      offlineDisabled: false,
    })
  })

  it('只差一台就能全 up：仍然后上线可点', () => {
    expect(resolveOpButtons([{ status: 'up' }, { status: 'down' }])).toEqual({
      onlineDisabled: false,
      offlineDisabled: false,
    })
  })

  it('单台 up', () => {
    expect(resolveOpButtons([{ status: 'up' }])).toEqual({ onlineDisabled: true, offlineDisabled: false })
  })

  it('单台 down', () => {
    expect(resolveOpButtons([{ status: 'down' }])).toEqual({ onlineDisabled: false, offlineDisabled: true })
  })
})
