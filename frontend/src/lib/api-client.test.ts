import { describe, it, expect, vi, beforeEach } from 'vitest'
import { api, ApiError } from '../lib/api-client'
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
      headers: new Headers({ 'content-type': 'application/json' }),
      json: () => Promise.resolve(mockError)
    } as Response)

    await expect(api.get('/api/v1/auth/me' as keyof paths)).rejects.toThrow('Invalid token')
  })

  it('should not set content-type header for GET requests', async () => {
    vi.mocked(fetch).mockResolvedValue({
      ok: true,
      headers: new Headers({ 'content-type': 'application/json' }),
      json: () => Promise.resolve({ data: { success: true }, code: 0 })
    } as Response)

    await api.get('/api/v1/auth/me' as keyof paths)

    const fetchCall = vi.mocked(fetch).mock.calls[0]
    const options = fetchCall[1] as RequestInit
    const contentType =
      options.headers instanceof Headers
        ? options.headers.get('Content-Type')
        : (options.headers as Record<string, string>)?.['Content-Type']

    expect(contentType).toBeNull()
  })

  it('should set content-type header for POST requests', async () => {
    vi.mocked(fetch).mockResolvedValue({
      ok: true,
      headers: new Headers({ 'content-type': 'application/json' }),
      json: () => Promise.resolve({ data: { success: true }, code: 0 })
    } as Response)

    await api.post('/api/v1/auth/login' as keyof paths, { username: 'admin', password: 'secret' })

    const fetchCall = vi.mocked(fetch).mock.calls[0]
    const options = fetchCall[1] as RequestInit
    const contentType =
      options.headers instanceof Headers
        ? options.headers.get('Content-Type')
        : (options.headers as Record<string, string>)?.['Content-Type']

    expect(contentType).toBe('application/json')
  })

  it('should map network errors to ApiError with code -1', async () => {
    vi.mocked(fetch).mockRejectedValue(new Error('network down'))

    await expect(api.get('/api/v1/auth/me' as keyof paths)).rejects.toMatchObject({
      name: 'ApiError',
      code: -1
    } satisfies Partial<ApiError>)
  })

  it('should normalize browser fetch failure message', async () => {
    vi.mocked(fetch).mockRejectedValue(new TypeError('Failed to fetch'))

    await expect(api.get('/api/v1/auth/me' as keyof paths)).rejects.toMatchObject({
      message: 'Network request failed',
      code: -1
    } satisfies Partial<ApiError>)
  })

  it('should surface envelope business error when http is ok', async () => {
    vi.mocked(fetch).mockResolvedValue({
      ok: true,
      headers: new Headers({ 'content-type': 'application/json' }),
      json: () => Promise.resolve({
        code: 1234,
        message: 'business failed',
        error: { type: 'business_error', reason: 'business failed' }
      })
    } as Response)

    await expect(api.get('/api/v1/auth/me' as keyof paths)).rejects.toThrow('business failed')
  })

  it('should fallback to plain text for non-json error response', async () => {
    vi.mocked(fetch).mockResolvedValue({
      ok: false,
      status: 503,
      statusText: 'Service Unavailable',
      headers: new Headers({ 'content-type': 'text/plain' }),
      text: () => Promise.resolve('backend unavailable')
    } as Response)

    await expect(api.get('/api/v1/auth/me' as keyof paths)).rejects.toThrow('backend unavailable')
  })
})
