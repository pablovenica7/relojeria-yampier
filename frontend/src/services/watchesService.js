import { request } from './api'

/** Arma "?a=1&b=2" omitiendo valores vacíos. */
export function toQuery(params = {}) {
  const qs = new URLSearchParams()
  Object.entries(params).forEach(([k, v]) => {
    if (v !== undefined && v !== null && v !== '') qs.set(k, v)
  })
  const s = qs.toString()
  return s ? `?${s}` : ''
}

/**
 * Catálogo público paginado. Devuelve { items, page, limit, total, pages }.
 * Parámetros: search, brand, gender, availability, sort, page, limit (máx. 100).
 */
export const getWatches = (params = {}) => request(`/watches${toQuery(params)}`)
export const getWatch = (id) => request(`/watches/${id}`)
