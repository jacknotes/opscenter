import { describe, it, expect } from 'vitest'
import { formatVersion, resolveVersion, appVersionPlugin, VIRTUAL_MODULE_ID } from './appVersion'

describe('formatVersion', () => {
  it('保留完整的 git describe 输出（tag + 提交数 + 哈希）', () => {
    expect(formatVersion('v2.0.1-7-g401385d', null, '1.0.0')).toBe('v2.0.1-7-g401385d')
  })

  it('tag 缺少 v 前缀时补全', () => {
    expect(formatVersion('2.0.1-7-g401385d', null, '1.0.0')).toBe('v2.0.1-7-g401385d')
  })

  it('正好打在 tag 上（无提交数和哈希）', () => {
    expect(formatVersion('v2.0.1', null, '1.0.0')).toBe('v2.0.1')
  })

  it('保留 dirty 标记', () => {
    expect(formatVersion('v2.0.1-7-g401385d-dirty', null, '1.0.0')).toBe('v2.0.1-7-g401385d-dirty')
  })

  it('两段式 tag 也能识别', () => {
    expect(formatVersion('v2.0-3-g401385d', null, '1.0.0')).toBe('v2.0-3-g401385d')
  })

  it('无可达 tag 的裸哈希：用 package.json 版本 + 总提交数合成同格式', () => {
    expect(formatVersion('401385d', 283, '2.0.0')).toBe('v2.0.0-283-g401385d')
  })

  it('无可达 tag 且工作区脏：保留 -dirty', () => {
    expect(formatVersion('401385d-dirty', 283, '2.0.0')).toBe('v2.0.0-283-g401385d-dirty')
  })

  it('裸哈希但拿不到提交数：退化为构建元数据形态', () => {
    expect(formatVersion('401385d', null, '1.0.0')).toBe('v1.0.0+401385d')
  })

  it('非 semver tag 原样保留，便于排查', () => {
    expect(formatVersion('release-x-7-g401385d', 100, '1.0.0')).toBe('v1.0.0+release-x-7-g401385d')
  })

  it('git 不可用（无 describe）时回退 package.json', () => {
    expect(formatVersion(null, null, '1.0.0')).toBe('v1.0.0')
    expect(formatVersion('', null, '1.0.0')).toBe('v1.0.0')
  })

  it('package.json 版本缺失时兜底 0.0.0', () => {
    expect(formatVersion(null, null, '')).toBe('v0.0.0')
  })
})

describe('resolveVersion', () => {
  it('APP_VERSION 环境变量优先于 git', () => {
    expect(resolveVersion({ cwd: process.cwd(), env: { APP_VERSION: 'v9.9.9' } })).toBe('v9.9.9')
  })

  it('VITE_APP_VERSION 环境变量同样生效', () => {
    expect(resolveVersion({ cwd: process.cwd(), env: { VITE_APP_VERSION: 'v8.8.8' } })).toBe('v8.8.8')
  })

  it('空字符串环境变量视为未设置', () => {
    expect(resolveVersion({ cwd: process.cwd(), env: { APP_VERSION: '   ' } })).toMatch(/^v\d+\.\d+/)
  })

  it('真实仓库解析出的版本符合 -N-g<hash> 或纯 tag 格式', () => {
    expect(resolveVersion({ env: {} })).toMatch(/^v\d+\.\d+\.\d+(?:-\d+-g[0-9a-f]+)?(?:-dirty)?$/)
  })

  it('非 git 目录（Docker 构建场景）回退到 package.json 版本', () => {
    expect(resolveVersion({ cwd: process.env.TEMP || '.', env: {} })).toMatch(/^v\d+\.\d+\.\d+/)
  })
})

describe('appVersionPlugin', () => {
  const plugin = appVersionPlugin('v2.0.1-7-g401385d')

  it('解析虚拟模块 id', () => {
    expect(plugin.resolveId(VIRTUAL_MODULE_ID)).toBe('\0' + VIRTUAL_MODULE_ID)
  })

  it('不劫持其他模块', () => {
    expect(plugin.resolveId('vue')).toBeNull()
    expect(plugin.load('/some/other/file.js')).toBeNull()
  })

  it('生成带版本号常量的模块源码', () => {
    const code = plugin.load('\0' + VIRTUAL_MODULE_ID)
    expect(code).toContain('export const APP_VERSION = "v2.0.1-7-g401385d"')
  })

  it('版本号经 JSON 序列化，不会因特殊字符破坏模块', () => {
    const code = appVersionPlugin('v1.0.0+"quote"').load('\0' + VIRTUAL_MODULE_ID)
    expect(code).toContain('export const APP_VERSION = "v1.0.0+\\"quote\\""')
  })
})
