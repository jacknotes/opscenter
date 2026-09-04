import { describe, it, expect, beforeEach, vi } from 'vitest'
import axios, { AxiosError, type AxiosResponse, type InternalAxiosRequestConfig } from 'axios'

vi.mock('@/utils/session', () => ({
  getToken: vi.fn(() => 'tok-123'),
  setToken: vi.fn(),
  clearSession: vi.fn(),
  getStoredUser: vi.fn(() => null),
  completeLogin: vi.fn(),
  initSession: vi.fn(),
}))

import client, { extractErrorMessage } from './client'
import { clearSession } from '@/utils/session'

function axiosError(status: number, data: unknown): AxiosError {
  return new AxiosError('Request failed', String(status), undefined, {}, {
    status,
    data,
    headers: {},
  } as unknown as AxiosResponse)
}

describe('api/client extractErrorMessage', () => {
  it('后端 {"error": "..."} 直接透出中文错误', () => {
    expect(extractErrorMessage(axiosError(400, { error: '参数错误' }))).toBe('参数错误')
  })

  it('LVS 执行失败附带 output 时仍取 error', () => {
    expect(
      extractErrorMessage(axiosError(500, { error: '执行失败', output: 'log...' })),
    ).toBe('执行失败')
  })

  it('超时（ECONNABORTED）映射为超时文案', () => {
    const err = new AxiosError('timeout of 30000ms exceeded', 'ECONNABORTED')
    expect(extractErrorMessage(err)).toBe('请求超时，请稍后重试')
  })

  it('非 {error} 结构返回 err.message', () => {
    expect(extractErrorMessage(axiosError(500, 'oops'), '网络异常')).toBe('Request failed')
  })

  it('非 axios 错误对象返回兜底文案', () => {
    expect(extractErrorMessage(undefined, '网络异常')).toBe('网络异常')
    expect(extractErrorMessage(new Error('boom'))).toBe('boom')
  })
})

describe('api/client 拦截器', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('请求自动注入 Bearer token', async () => {
    let captured: InternalAxiosRequestConfig | undefined
    client.defaults.adapter = async (config) => {
      captured = config
      return {
        data: { ok: true },
        status: 200,
        statusText: 'OK',
        headers: {},
        config,
      } as AxiosResponse
    }
    await client.get('/ping')
    expect(captured?.headers?.Authorization).toBe('Bearer tok-123')
  })

  it('401 → 清会话（并跳转登录）', async () => {
    client.defaults.adapter = async (config) => {
      throw new AxiosError(
        'Request failed',
        '401',
        config,
        {},
        {
          data: { error: '认证令牌已被撤销' },
          status: 401,
          statusText: 'Unauthorized',
          headers: {},
          config,
        } as AxiosResponse,
      )
    }
    await expect(client.get('/user/info')).rejects.toMatchObject({
      response: { status: 401, data: { error: '认证令牌已被撤销' } },
    })
    expect(clearSession).toHaveBeenCalled()
  })
})
