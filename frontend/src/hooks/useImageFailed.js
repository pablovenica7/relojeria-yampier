import { useState } from 'react'

/**
 * Indica si la imagen `src` no pudo cargarse (archivo borrado, URL vieja,
 * servidor dormido). Se reinicia sola cuando cambia `src`.
 * Uso: const [failed, onError] = useImageFailed(src); <img onError={onError} />
 */
export function useImageFailed(src) {
  const [failedSrc, setFailedSrc] = useState(null)
  return [Boolean(src) && failedSrc === src, () => setFailedSrc(src)]
}
