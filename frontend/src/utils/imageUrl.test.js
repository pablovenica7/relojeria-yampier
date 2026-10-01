import { describe, expect, it } from 'vitest'
import { imageUrl } from './imageUrl'

describe('imageUrl', () => {
  it('devuelve string vacío si no hay path', () => {
    expect(imageUrl('')).toBe('')
    expect(imageUrl(null)).toBe('')
  })

  it('deja intacta una URL absoluta', () => {
    expect(imageUrl('https://cdn.example.com/foto.jpg')).toBe('https://cdn.example.com/foto.jpg')
  })

  it('arma la URL completa para una ruta relativa de uploads', () => {
    const result = imageUrl('/uploads/123.jpg')
    expect(result).toMatch(/\/uploads\/123\.jpg$/)
    expect(result).not.toBe('/uploads/123.jpg') // debe tener el origin anteponiendo
  })

  it('deja intacta una imagen estática del sitio', () => {
    expect(imageUrl('/images/relojes/casio/f-91w-1.webp')).toBe('/images/relojes/casio/f-91w-1.webp')
  })
})
