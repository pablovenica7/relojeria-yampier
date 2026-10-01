const BASE = import.meta.env.VITE_API_URL || 'http://localhost:8080/api'
const TOKEN_KEY = 'yampier_admin_token'
const CUSTOMER_TOKEN_KEY = 'yampier_customer_token'
const CUSTOMER_SESSION_EXPIRED = 'yampier:customer-session-expired'
const DEFAULT_TIMEOUT = 15000

/**
 * apiFetch: única fuente de verdad para hablar con la API. Centraliza:
 * - construcción de la URL y headers (incluido el token, si corresponde)
 * - timeout con AbortController (evita requests colgados para siempre)
 * - parseo de JSON tolerante a respuestas vacías o no-JSON
 * - manejo uniforme de errores HTTP, 401 y errores de red
 *
 * request/authRequest/authUpload de más abajo son wrappers finos sobre esta
 * función; no duplican esta lógica.
 */
async function apiFetch(path, { auth = false, isUpload = false, timeout = DEFAULT_TIMEOUT, ...options } = {}) {
  const controller = new AbortController()
  const timer = setTimeout(() => controller.abort(), timeout)

  const headers = { ...(options.headers || {}) }
  if (!isUpload) headers['Content-Type'] = 'application/json'
  // auth: true = sesión Admin; 'customer' = sesión de cliente. Son tokens
  // distintos, guardados con claves distintas: nunca se mezclan.
  const tokenKey = auth === 'customer' ? CUSTOMER_TOKEN_KEY : TOKEN_KEY
  if (auth) {
    const token = localStorage.getItem(tokenKey)
    if (token) headers.Authorization = `Bearer ${token}`
  }

  let res
  try {
    res = await fetch(`${BASE}${path}`, { ...options, headers, signal: controller.signal })
  } catch (err) {
    if (err.name === 'AbortError') {
      throw new Error('La solicitud tardó demasiado. Verificá tu conexión e intentá de nuevo.')
    }
    throw new Error('No se pudo conectar con el servidor. Verificá tu conexión.')
  } finally {
    clearTimeout(timer)
  }

  // Sesión vencida o inválida: limpiamos el token y mandamos al login.
  // Se hace acá, en un único lugar, para no repetir esta lógica en cada
  // servicio que llama a una ruta protegida.
  if (auth === true && res.status === 401) {
    localStorage.removeItem(TOKEN_KEY)
    if (typeof window !== 'undefined' && window.location.pathname !== '/admin/login') {
      window.location.href = '/admin/login'
    }
    throw new Error('Sesión expirada. Iniciá sesión de nuevo.')
  }
  // Cliente: se limpia la sesión y se avisa a la app (CustomerAuthContext),
  // que decide a dónde llevarlo sin perder la página en la que estaba.
  if (auth === 'customer' && res.status === 401) {
    localStorage.removeItem(CUSTOMER_TOKEN_KEY)
    if (typeof window !== 'undefined') window.dispatchEvent(new Event(CUSTOMER_SESSION_EXPIRED))
  }

  // Algunas respuestas (204, o errores de proxy) pueden no traer JSON.
  const text = await res.text()
  const data = text ? safeParseJSON(text) : {}

  if (!res.ok) {
    // El backend responde {"error": "mensaje", "code": "CODIGO"}. Se expone
    // code/status para reaccionar a casos puntuales sin comparar textos.
    const err = new Error(data?.error || `Error inesperado del servidor (${res.status})`)
    err.status = res.status
    err.code = data?.code
    err.data = data
    throw err
  }
  return data
}

function safeParseJSON(text) {
  try {
    return JSON.parse(text)
  } catch {
    return {}
  }
}

/** Llamada pública, sin autenticación. */
export const request = (path, options = {}) => apiFetch(path, options)

/** Llamada autenticada: agrega el token del panel Admin y maneja 401. */
export const authRequest = (path, options = {}) => apiFetch(path, { ...options, auth: true })

/** Subida de archivos autenticada (FormData, sin Content-Type manual). */
export function authUpload(path, file) {
  const form = new FormData()
  form.append('image', file)
  return apiFetch(path, { method: 'POST', body: form, auth: true, isUpload: true })
}

/** Llamada autenticada como cliente (token de cliente, nunca el del Admin). */
export const customerRequest = (path, options = {}) => apiFetch(path, { ...options, auth: 'customer' })

// Se exportan las keys para que los servicios de sesión no repitan el string.
export { TOKEN_KEY, CUSTOMER_TOKEN_KEY, CUSTOMER_SESSION_EXPIRED }
