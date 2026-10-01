import { authRequest } from './api'
import { toQuery } from './watchesService'

export const adminListInquiries = (params = {}) => authRequest(`/admin/inquiries${toQuery(params)}`)

/** Cambia estado (new | contacted | closed) y/o notas internas. */
export const adminUpdateInquiry = (id, data) =>
  authRequest(`/admin/inquiries/${id}`, { method: 'PATCH', body: JSON.stringify(data) })

export const adminDeleteInquiry = (id) =>
  authRequest(`/admin/inquiries/${id}`, { method: 'DELETE' })
