import { AxiosError, AxiosHeaders, InternalAxiosRequestConfig } from 'axios'
import { afterEach, describe, expect, it } from 'vitest'
import { apiClient } from './apiClient'
import { ApiError, getErrorMessage, isProblem } from './errorUtils'

const originalAdapter = apiClient.getAxiosInstance().defaults.adapter

afterEach(() => {
  apiClient.getAxiosInstance().defaults.adapter = originalAdapter
})

// 実際の HTTP は送らず、axios の adapter で応答を差し替える
function respondWith(adapter: (config: InternalAxiosRequestConfig) => Promise<never>) {
  apiClient.getAxiosInstance().defaults.adapter = adapter
}

function httpError(config: InternalAxiosRequestConfig, status: number, data: unknown): AxiosError {
  return new AxiosError('Request failed', 'ERR_BAD_RESPONSE', config, null, {
    status,
    statusText: '',
    data,
    headers: {},
    config: { ...config, headers: new AxiosHeaders() },
  })
}

describe('apiClient のエラー', () => {
  it('Problem Details の応答は、その項目を持つ ApiError で reject する', async () => {
    respondWith((config) =>
      Promise.reject(
        httpError(config, 409, {
          type: 'about:blank',
          title: 'Conflict',
          status: 409,
          detail: 'このメールアドレスは既に使用されています',
          errors: [{ field: 'email', message: 'already exists' }],
        })
      )
    )

    const error = await apiClient.get('/api/users').catch((e: unknown) => e)

    expect(error).toBeInstanceOf(ApiError)
    expect(error).toBeInstanceOf(Error)
    expect(error).toMatchObject({
      name: 'ApiError',
      title: 'Conflict',
      status: 409,
      detail: 'このメールアドレスは既に使用されています',
      errors: [{ field: 'email', message: 'already exists' }],
      message: 'このメールアドレスは既に使用されています',
    })
    expect((error as ApiError).cause).toBeInstanceOf(AxiosError)
    expect(isProblem(error)).toBe(true)
    expect(getErrorMessage(error)).toBe('このメールアドレスは既に使用されています')
  })

  it('Problem Details ではない失敗（ネットワークエラーなど）も ApiError にそろえる', async () => {
    respondWith((config) => Promise.reject(new AxiosError('Network Error', 'ERR_NETWORK', config)))

    const error = await apiClient.get('/api/users').catch((e: unknown) => e)

    expect(error).toBeInstanceOf(ApiError)
    expect(error).toMatchObject({ title: 'Internal Server Error', status: 500, detail: 'Network Error' })
  })

  it('detail が無ければ title をメッセージにする', () => {
    const error = new ApiError({ type: 'about:blank', title: 'Not Found', status: 404 })

    expect(error.message).toBe('Not Found')
    expect(getErrorMessage(error)).toBe('Not Found')
  })
})
