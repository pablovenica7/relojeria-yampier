import { authRequest } from './api'
import { toQuery } from './watchesService'

/** Filtros: status, watchId, from, to (AAAA-MM-DD), page, limit. */
export const adminListReservations = (params = {}) => authRequest(`/admin/reservations${toQuery(params)}`)

export const adminCreateReservation = (data) =>
  authRequest('/admin/reservations', { method: 'POST', body: JSON.stringify(data) })

/** Cambio de estado: el backend aplica el efecto sobre el stock. */
export const adminSetReservationStatus = (id, status, depositAmount, depositMethod) =>
  authRequest(`/admin/reservations/${id}/status`, {
    method: 'PATCH',
    body: JSON.stringify(depositAmount === undefined ? { status } : { status, depositAmount, depositMethod }),
  })

/** Notas, seña acordada y vencimiento. */
export const adminUpdateReservation = (id, data) =>
  authRequest(`/admin/reservations/${id}`, { method: 'PATCH', body: JSON.stringify(data) })
