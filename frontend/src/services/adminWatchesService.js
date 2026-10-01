import { authRequest, authUpload } from './api'
import { toQuery } from './watchesService'

// El Admin usa sus propios endpoints de lectura porque necesita ver relojes
// archivados y los contadores de stock; el sitio público solo ve activos.

/** { items, page, limit, total, pages, stats: { total, outOfStock, lastUnit } } */
export const adminListWatches = (params = {}) => authRequest(`/admin/watches${toQuery(params)}`)

export const adminGetWatch = (id) => authRequest(`/admin/watches/${id}`)

export const createWatch = (data) =>
  authRequest('/admin/watches', { method: 'POST', body: JSON.stringify(data) })

export const updateWatch = (id, data) =>
  authRequest(`/admin/watches/${id}`, { method: 'PUT', body: JSON.stringify(data) })

/** Ajuste atómico de stock (+1 / -1). El backend nunca deja el stock negativo. */
export const adjustWatchStock = (id, delta) =>
  authRequest(`/admin/watches/${id}/stock`, { method: 'PATCH', body: JSON.stringify({ delta }) })

// Si el reloj tiene reservas activas, el backend responde 409
// ACTIVE_RESERVATIONS; con confirm=true se archiva igualmente.
export const archiveWatch = (id, confirm = false) =>
  authRequest(`/admin/watches/${id}/archive${confirm ? '?confirm=true' : ''}`, { method: 'PATCH' })

export const restoreWatch = (id) => authRequest(`/admin/watches/${id}/restore`, { method: 'PATCH' })

/** Borrado definitivo: solo relojes archivados y sin reservas. */
export const deleteWatch = (id) =>
  authRequest(`/admin/watches/${id}`, { method: 'DELETE' })

export const uploadWatchImage = (file) => authUpload('/admin/uploads', file)
