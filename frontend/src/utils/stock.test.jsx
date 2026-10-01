import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import { AVAILABILITY, availabilityFor, availabilityLabel, isAvailable } from './stock'
import { mailtoUrl, normalizeWhatsAppNumber, whatsappUrl } from './siteConfig'
import StockBadge from '../components/ui/StockBadge'

describe('disponibilidad derivada de stockQuantity', () => {
  it('0 (o dato ausente) es sin stock', () => {
    expect(availabilityFor(0)).toBe(AVAILABILITY.OUT_OF_STOCK)
    expect(availabilityFor(undefined)).toBe(AVAILABILITY.OUT_OF_STOCK)
    expect(availabilityFor(-2)).toBe(AVAILABILITY.OUT_OF_STOCK)
  })

  it('1 es última unidad y 2 o más es en stock', () => {
    expect(availabilityFor(1)).toBe(AVAILABILITY.LAST_UNIT)
    expect(availabilityFor(2)).toBe(AVAILABILITY.IN_STOCK)
    expect(availabilityFor(40)).toBe(AVAILABILITY.IN_STOCK)
  })

  it('solo 0 unidades se considera no disponible', () => {
    expect(isAvailable(1)).toBe(true)
    expect(isAvailable(0)).toBe(false)
  })

  it('tiene etiquetas en castellano', () => {
    expect(availabilityLabel(AVAILABILITY.LAST_UNIT)).toBe('Última unidad')
  })
})

describe('StockBadge', () => {
  it.each([
    [0, 'Sin stock'],
    [1, 'Última unidad'],
    [5, 'En stock'],
  ])('con %i unidades muestra "%s"', (quantity, label) => {
    render(<StockBadge quantity={quantity} />)
    expect(screen.getByText(label)).toBeInTheDocument()
  })
})

describe('enlaces de contacto', () => {
  it('normaliza el número de WhatsApp a solo dígitos', () => {
    expect(normalizeWhatsAppNumber('+54 9 351 000-0000')).toBe('5493510000000')
  })

  it('no arma un link de WhatsApp sin número configurado', () => {
    expect(whatsappUrl('Hola', '')).toBe('')
  })

  it('incluye el mensaje codificado en el link de WhatsApp', () => {
    expect(whatsappUrl('Hola, Casio F-91W-1', '5493510000000'))
      .toBe('https://wa.me/5493510000000?text=Hola%2C%20Casio%20F-91W-1')
  })

  it('arma un mailto con asunto y cuerpo, o vacío sin email', () => {
    expect(mailtoUrl('Consulta', 'Hola', '')).toBe('')
    expect(mailtoUrl('Consulta F-91W', 'Hola', 'a@b.com')).toBe('mailto:a@b.com?subject=Consulta%20F-91W&body=Hola')
  })
})
