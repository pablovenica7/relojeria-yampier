/**
 * Valida el destino al que volver después de iniciar sesión (?returnTo=).
 * Solo acepta rutas internas del sitio público: evita "open redirects"
 * (que un link armado por un tercero mande al usuario a otro dominio después
 * del login) y nunca lleva al panel Admin.
 */
export function safeReturnTo(value, fallback = '/mi-cuenta') {
  if (typeof value !== 'string' || value.length === 0 || value.length > 300) return fallback
  // Debe ser una ruta relativa al sitio: "/algo", nunca "//dominio" ni "/\dominio".
  if (!value.startsWith('/') || value.startsWith('//') || value.startsWith('/\\')) return fallback
  // Sin esquemas ni caracteres de control escondidos.
  const hasControlChars = [...value].some(ch => ch.charCodeAt(0) < 32)
  if (hasControlChars || value.includes('\\') || /^\/*[a-z][a-z0-9+.-]*:/i.test(value)) return fallback
  if (value === '/admin' || value.startsWith('/admin/')) return fallback
  if (value.startsWith('/ingresar') || value.startsWith('/crear-cuenta')) return fallback
  return value
}

/** Arma el link de login conservando la página actual. */
export function loginPath(currentPath) {
  return `/ingresar?returnTo=${encodeURIComponent(safeReturnTo(currentPath, '/'))}`
}
