import { describe, expect, it } from 'vitest'
import { formatCurrency, parseARS } from './formatCurrency'

describe('formatCurrency', () => {
  it('formatea un entero como moneda en pesos argentinos', () => {
    expect(formatCurrency(150000)).toMatch(/150\.000/)
  })

  it('devuelve string vacío para 0, null o undefined (precio a consultar)', () => {
    expect(formatCurrency(0)).toBe('')
    expect(formatCurrency(null)).toBe('')
    expect(formatCurrency(undefined)).toBe('')
  })

  it('no muestra decimales', () => {
    expect(formatCurrency(1999)).not.toMatch(/,\d{2}$/)
  })
})

describe('parseARS', () => {
  it('acepta montos con separador de miles y símbolo', () => {
    expect(parseARS('150.000')).toBe(150000)
    expect(parseARS('$ 150.000')).toBe(150000)
    expect(parseARS(170000)).toBe(170000)
  })

  it('vacío es 0 (precio a consultar)', () => {
    expect(parseARS('')).toBe(0)
  })

  it('rechaza centavos, negativos y texto', () => {
    expect(parseARS('1500,50')).toBeNaN()
    expect(parseARS('-100')).toBeNaN()
    expect(parseARS('abc')).toBeNaN()
  })
})
