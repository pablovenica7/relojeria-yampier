import { request, TOKEN_KEY } from './api'

export async function login(email, password) {
  const data = await request('/admin/login', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  })
  localStorage.setItem(TOKEN_KEY, data.token)
  return data
}

export function logout() {
  localStorage.removeItem(TOKEN_KEY)
}

export function getToken() {
  return localStorage.getItem(TOKEN_KEY)
}

export function isAuthenticated() {
  return !!getToken()
}
