import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { request, authRequest, TOKEN_KEY } from './api'

function mockFetchOnce({ ok = true, status = 200, body = {} } = {}) {
  global.fetch = vi.fn().mockResolvedValue({
    ok,
    status,
    text: async () => JSON.stringify(body),
  })
}

describe('api / apiFetch', () => {
  beforeEach(() => {
    localStorage.clear()
  })
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('devuelve los datos parseados cuando la respuesta es exitosa', async () => {
    mockFetchOnce({ body: { status: 'up' } })
    const data = await request('/health')
    expect(data).toEqual({ status: 'up' })
  })

  it('lanza un Error con el mensaje del backend cuando la respuesta no es ok', async () => {
    mockFetchOnce({ ok: false, status: 400, body: { error: 'Datos inválidos' } })
    await expect(request('/inquiries', { method: 'POST' })).rejects.toThrow('Datos inválidos')
  })

  it('expone el código y el status del error', async () => {
    mockFetchOnce({ ok: false, status: 409, body: { error: 'Tiene reservas activas', code: 'ACTIVE_RESERVATIONS', activeReservations: 2 } })
    const err = await request('/x').catch(e => e)
    expect(err.code).toBe('ACTIVE_RESERVATIONS')
    expect(err.status).toBe(409)
    expect(err.data.activeReservations).toBe(2)
  })

  it('convierte un fallo de red en un mensaje legible', async () => {
    global.fetch = vi.fn().mockRejectedValue(new TypeError('Failed to fetch'))
    await expect(request('/watches')).rejects.toThrow(/no se pudo conectar/i)
  })

  it('agrega el header Authorization en authRequest cuando hay token guardado', async () => {
    localStorage.setItem(TOKEN_KEY, 'abc123')
    mockFetchOnce({ body: [] })
    await authRequest('/admin/inquiries')
    const [, options] = global.fetch.mock.calls[0]
    expect(options.headers.Authorization).toBe('Bearer abc123')
  })

  it('en un 401 autenticado, limpia el token guardado', async () => {
    // Nos "paramos" en /admin/login para que apiFetch no intente redirigir
    // (jsdom no implementa navegación real; usamos el History API, que sí soporta).
    window.history.pushState({}, '', '/admin/login')
    localStorage.setItem(TOKEN_KEY, 'expirado')
    mockFetchOnce({ ok: false, status: 401, body: { error: 'No autenticado' } })
    await expect(authRequest('/admin/inquiries')).rejects.toThrow()
    expect(localStorage.getItem(TOKEN_KEY)).toBeNull()
  })
})
