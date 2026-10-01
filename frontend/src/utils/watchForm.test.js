import { describe, expect, it } from 'vitest'
import { emptyWatchForm, toWatchPayload, validateWatchForm, watchToForm } from './watchForm'

const valid = { ...emptyWatchForm, name: 'Casio F-91W-1', model: 'f-91w-1', price: '30.000', stockQuantity: '2' }

describe('formulario de reloj (Admin)', () => {
  it('acepta un formulario completo', () => {
    expect(validateWatchForm(valid)).toEqual({})
  })

  it('exige modelo y stock entero >= 0', () => {
    expect(validateWatchForm({ ...valid, model: '' }).model).toBeTruthy()
    expect(validateWatchForm({ ...valid, stockQuantity: '-1' }).stockQuantity).toBeTruthy()
    expect(validateWatchForm({ ...valid, stockQuantity: '1.5' }).stockQuantity).toBeTruthy()
    expect(validateWatchForm({ ...valid, stockQuantity: '' }).stockQuantity).toBeTruthy()
  })

  it('rechaza precios con centavos', () => {
    expect(validateWatchForm({ ...valid, price: '1500,50' }).price).toBeTruthy()
  })

  it('envía montos y cantidades como enteros, modelo normalizado y stock base', () => {
    const payload = toWatchPayload(valid, 3)
    expect(payload.priceARS).toBe(30000)
    expect(payload.stockQuantity).toBe(2)
    expect(payload.stockQuantityBase).toBe(3)
    expect(payload.model).toBe('F-91W-1')
    expect(payload).not.toHaveProperty('price')
  })

  it('al crear no envía stock base', () => {
    expect(toWatchPayload(valid)).not.toHaveProperty('stockQuantityBase')
  })

  it('convierte un reloj de la API al formulario', () => {
    const form = watchToForm({ name: 'X', model: 'GA-2100', priceARS: 0, stockQuantity: 1, specs: ['a', 'b'] })
    expect(form.price).toBe('')
    expect(form.stockQuantity).toBe('1')
    expect(form.specs).toBe('a\nb')
  })
})
