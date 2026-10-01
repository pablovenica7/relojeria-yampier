import { CUSTOMER_TOKEN_KEY, customerRequest, request } from './api'

// Sesión de CLIENTES. Independiente del Admin (authService.js): otro
// endpoint de login, otro token y otra clave de almacenamiento.

function saveSession(data) {
  localStorage.setItem(CUSTOMER_TOKEN_KEY, data.token)
  return data.customer
}

export const registerCustomer = (data) =>
  request('/auth/register', { method: 'POST', body: JSON.stringify(data) }).then(saveSession)

export const loginCustomer = (email, password) =>
  request('/auth/login', { method: 'POST', body: JSON.stringify({ email, password }) }).then(saveSession)

export function logoutCustomer() {
  localStorage.removeItem(CUSTOMER_TOKEN_KEY)
}

export function hasCustomerSession() {
  return Boolean(localStorage.getItem(CUSTOMER_TOKEN_KEY))
}

export const getMe = () => customerRequest('/auth/me')

export const updateMe = (data) => customerRequest('/auth/me', { method: 'PUT', body: JSON.stringify(data) })

// Cambiar la contraseña cierra las demás sesiones (el backend sube la
// versión del token) y devuelve un token nuevo para seguir en esta.
export const changePassword = (currentPassword, newPassword) =>
  customerRequest('/auth/me/password', { method: 'POST', body: JSON.stringify({ currentPassword, newPassword }) })
    .then(data => { if (data.token) localStorage.setItem(CUSTOMER_TOKEN_KEY, data.token); return data })

// Recuperación de contraseña: la respuesta es siempre genérica.
export const forgotPassword = (email) =>
  request('/auth/password/forgot', { method: 'POST', body: JSON.stringify({ email }) })

export const resetPassword = (token, newPassword) =>
  request('/auth/password/reset', { method: 'POST', body: JSON.stringify({ token, newPassword }) })

// Reservas del cliente (el backend siempre filtra por el dueño del token).
export const requestReservation = (watchId, termsAccepted, notes) =>
  customerRequest('/reservations', { method: 'POST', body: JSON.stringify({ watchId, termsAccepted, notes }) })

export const getMyReservations = () => customerRequest('/reservations')

export const cancelMyReservation = (id) => customerRequest(`/reservations/${id}/cancel`, { method: 'POST' })
