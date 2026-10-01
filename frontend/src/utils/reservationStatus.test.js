import { describe, expect, it } from 'vitest'
import {
  RESERVATION_STATUS as S, dateInputValue, expiryFromDate, holdsStock, isExpirable, isFinalStatus, nextStatuses, reservationLabel,
} from './reservationStatus'

describe('estados de reserva', () => {
  it('permite el flujo normal pendiente → confirmada → seña → completada', () => {
    expect(nextStatuses(S.PENDING)).toContain(S.CONFIRMED)
    expect(nextStatuses(S.CONFIRMED)).toContain(S.DEPOSIT_PAID)
    expect(nextStatuses(S.DEPOSIT_PAID)).toContain(S.COMPLETED)
  })

  it('no ofrece transiciones absurdas', () => {
    expect(nextStatuses(S.PENDING)).not.toContain(S.COMPLETED)
    expect(nextStatuses(S.COMPLETED)).toEqual([])
    expect(isFinalStatus(S.CANCELLED)).toBe(true)
    expect(isFinalStatus(S.EXPIRED)).toBe(true)
  })

  it('solo confirmada y seña paga retienen stock', () => {
    expect(holdsStock(S.CONFIRMED)).toBe(true)
    expect(holdsStock(S.DEPOSIT_PAID)).toBe(true)
    expect(holdsStock(S.PENDING)).toBe(false)
    expect(holdsStock(S.COMPLETED)).toBe(false)
  })

  it('traduce los estados', () => {
    expect(reservationLabel(S.DEPOSIT_PAID)).toBe('Seña paga')
  })

  it('solo pendientes y confirmadas vencen', () => {
    expect(isExpirable(S.PENDING)).toBe(true)
    expect(isExpirable(S.DEPOSIT_PAID)).toBe(false)
  })

  it('convierte la fecha de vencimiento ida y vuelta (fin del día local)', () => {
    const iso = expiryFromDate('2030-05-10')
    expect(new Date(iso).getHours()).toBe(23)
    expect(dateInputValue(iso)).toBe('2030-05-10')
    expect(expiryFromDate('')).toBeNull()
  })
})
