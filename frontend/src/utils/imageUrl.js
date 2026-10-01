const API_ORIGIN = (import.meta.env.VITE_API_URL || 'http://localhost:8080/api').replace(/\/api$/, '')

// Las imágenes subidas desde el panel Admin se guardan como rutas relativas
// (ej: "/uploads/archivo.jpg") y las sirve el backend: a esas se les antepone
// el origen de la API. Las imágenes estáticas del propio sitio (ej:
// "/images/relojes/casio/f-91w-1.webp", en frontend/public) se dejan tal cual.
export function imageUrl(path) {
  if (!path) return ''
  if (path.startsWith('http')) return path
  if (path.startsWith('/uploads/')) return `${API_ORIGIN}${path}`
  return path
}
