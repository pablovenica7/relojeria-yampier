import { describe, expect, it, vi } from 'vitest'
import { renderHook, waitFor, act } from '@testing-library/react'
import { useAsync } from './useAsync'

describe('useAsync', () => {
  it('pasa de loading a success con los datos resueltos', async () => {
    const fn = vi.fn().mockResolvedValue([{ id: '1' }])
    const { result } = renderHook(() => useAsync(fn, []))

    expect(result.current.state).toBe('loading')

    await waitFor(() => expect(result.current.state).toBe('success'))
    expect(result.current.data).toEqual([{ id: '1' }])
  })

  it('pasa a error si la promesa rechaza', async () => {
    const fn = vi.fn().mockRejectedValue(new Error('falló'))
    const { result } = renderHook(() => useAsync(fn, []))

    await waitFor(() => expect(result.current.state).toBe('error'))
    expect(result.current.error.message).toBe('falló')
  })

  it('reload() vuelve a ejecutar la función y descarta respuestas obsoletas', async () => {
    const fn = vi.fn().mockResolvedValue('ok')
    const { result } = renderHook(() => useAsync(fn, []))

    await waitFor(() => expect(result.current.state).toBe('success'))
    expect(fn).toHaveBeenCalledTimes(1)

    act(() => result.current.reload())
    await waitFor(() => expect(fn).toHaveBeenCalledTimes(2))
  })

  it('setData permite actualizaciones optimistas (patrón prev => ...)', async () => {
    const fn = vi.fn().mockResolvedValue([{ id: '1' }, { id: '2' }])
    const { result } = renderHook(() => useAsync(fn, []))

    await waitFor(() => expect(result.current.state).toBe('success'))

    act(() => result.current.setData(prev => prev.filter(x => x.id !== '1')))
    expect(result.current.data).toEqual([{ id: '2' }])
  })
})
