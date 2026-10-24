import { afterEach, describe, expect, it, vi } from 'vitest'

async function loadRewrites() {
  vi.resetModules()
  const { default: config } = await import('./next.config')
  return config.rewrites?.()
}

describe('next.config の rewrites', () => {
  afterEach(() => {
    vi.unstubAllEnvs()
  })

  it('/api/* を既定のバックエンドに転送する', async () => {
    vi.stubEnv('BACKEND_URL', undefined)
    expect(await loadRewrites()).toEqual([
      { source: '/api/:path*', destination: 'http://localhost:8080/api/:path*' },
    ])
  })

  it('転送先は BACKEND_URL で変えられる', async () => {
    vi.stubEnv('BACKEND_URL', 'http://api:8080')
    expect(await loadRewrites()).toEqual([
      { source: '/api/:path*', destination: 'http://api:8080/api/:path*' },
    ])
  })
})
