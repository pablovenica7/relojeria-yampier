import { describe, expect, it } from 'vitest'
import { safeReturnTo, loginPath } from './returnTo'
import {
  emptyAddress, formToCustomerPayload, isValidCuit, requiresTaxId, validateCustomerForm,
} from './customerOptions'
import { RESERVATION_STATUS as S, customerReservationLabel, depositMethodLabel } from './reservationStatus'

describe('safeReturnTo (evita open redirects)', () => {
  it('acepta rutas internas del sitio', () => {
    expect(safeReturnTo('/reservar/abc123')).toBe('/reservar/abc123')
    expect(safeReturnTo('/relojes?page=2')).toBe('/relojes?page=2')
  })

  it.each([
    'https://evil.com', '//evil.com', '/\\evil.com', 'javascript:alert(1)',
    '/admin/reservas', '/admin', '', null, 'relojes', '/ingresar?returnTo=/x',
  ])('rechaza %s', (value) => {
    expect(safeReturnTo(value, '/fallback')).toBe('/fallback')
  })

  it('arma el link de login conservando la página', () => {
    expect(loginPath('/reservar/abc')).toBe('/ingresar?returnTo=%2Freservar%2Fabc')
  })
})

const validForm = {
  firstName: 'Ana', lastName: 'Pérez', email: 'ana@example.com', phone: '351 555 1234',
  password: 'secreta123', passwordConfirm: 'secreta123', privacyAccepted: true, marketingConsent: false,
  documentType: 'dni', documentNumber: '30.123.456', taxCondition: 'consumer_final', taxIdType: '', taxId: '',
  billingAddress: { ...emptyAddress },
}

describe('formulario de cuenta', () => {
  it('acepta un registro válido sin domicilio', () => {
    expect(validateCustomerForm(validForm, { register: true })).toEqual({})
  })

  it('exige aceptar la privacidad pero NO el consentimiento comercial', () => {
    expect(validateCustomerForm({ ...validForm, privacyAccepted: false }, { register: true }).privacyAccepted).toBeTruthy()
    expect(validateCustomerForm({ ...validForm, marketingConsent: false }, { register: true })).toEqual({})
  })

  it('pide CUIT/CUIL/CDI para monotributo y valida el verificador', () => {
    expect(requiresTaxId('monotributo')).toBe(true)
    expect(requiresTaxId('consumer_final')).toBe(false)
    expect(validateCustomerForm({ ...validForm, taxCondition: 'monotributo' }).taxId).toBeTruthy()
    expect(validateCustomerForm({ ...validForm, taxCondition: 'monotributo', taxIdType: 'cuit', taxId: '20-12345678-6' })).toEqual({})
    expect(isValidCuit('20-12345678-7')).toBe(false)
  })

  it('valida el domicilio solo si se empezó a completar', () => {
    const partial = { ...validForm, billingAddress: { ...emptyAddress, street: 'Rivadavia' } }
    const e = validateCustomerForm(partial)
    expect(e.number).toBeTruthy()
    expect(e.postalCode).toBeTruthy()
  })

  it('envía null como domicilio si está vacío', () => {
    expect(formToCustomerPayload(validForm).billingAddress).toBeNull()
  })
})

describe('estados de reserva para el cliente', () => {
  it('usa nombres amigables', () => {
    expect(customerReservationLabel(S.PENDING)).toBe('Pendiente de confirmación')
    expect(customerReservationLabel(S.DEPOSIT_PAID)).toBe('Seña recibida')
    expect(customerReservationLabel(S.COMPLETED)).toBe('Compra completada')
    expect(depositMethodLabel('bank_transfer')).toBe('Transferencia bancaria')
  })
})
