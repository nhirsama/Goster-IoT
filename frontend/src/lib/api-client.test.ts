import { describe, it, expect, vi, beforeEach } from 'vitest'
import { api } from '../lib/api-client'
import { paths } from '../lib/api-types'

describe('API Client', () => {
  beforeEach(() => {
    global.fetch = vi.fn()
  })

  it('should include request-id header', async () => {
    const mockResponse = { data: { success: true }, code: 0 }
    vi.mocked(fetch).mockResolvedValue({
      ok: true,
      json: () => Promise.resolve(mockResponse)
    } as Response)

    await api.get('/api/v1/auth/me' as keyof paths)

    const fetchCall = vi.mocked(fetch).mock.calls[0]
    const options = fetchCall[1] as RequestInit
    const headerValue =
      options.headers instanceof Headers
        ? options.headers.get('X-Request-Id')
        : (options.headers as Record<string, string>)?.['X-Request-Id']

    expect(headerValue).toBeDefined()
    expect(headerValue).toMatch(/^req_\d+/)
  })

  it('should handle API errors', async () => {
    const mockError = { message: 'Invalid token', code: 401 }
    vi.mocked(fetch).mockResolvedValue({
      ok: false,
      json: () => Promise.resolve(mockError)
    } as Response)

    await expect(api.get('/api/v1/auth/me' as keyof paths)).rejects.toThrow('Invalid token')
  })
})
