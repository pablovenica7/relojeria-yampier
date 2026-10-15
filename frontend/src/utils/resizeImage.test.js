import { describe, expect, it } from 'vitest'
import { MAX_IMAGE_SIDE, resizeImageFile, targetSize } from './resizeImage'

describe('targetSize', () => {
  it('achica manteniendo la proporción', () => {
    expect(targetSize(4000, 3000)).toEqual({ width: MAX_IMAGE_SIDE, height: 1200 })
    expect(targetSize(3000, 4000)).toEqual({ width: 1200, height: MAX_IMAGE_SIDE })
  })
  it('nunca agranda una imagen chica', () => {
    expect(targetSize(800, 600)).toEqual({ width: 800, height: 600 })
  })
})

describe('resizeImageFile', () => {
  it('devuelve el original si no es una imagen admitida', async () => {
    const f = new File(['hola'], 'nota.txt', { type: 'text/plain' })
    expect(await resizeImageFile(f)).toBe(f)
  })
  it('devuelve el original si el navegador no puede procesarla', async () => {
    const f = new File(['no es png'], 'foto.png', { type: 'image/png' })
    expect(await resizeImageFile(f)).toBe(f)
  })
})
