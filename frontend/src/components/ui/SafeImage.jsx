import { useImageFailed } from '../../hooks/useImageFailed'

/** <img> que muestra `fallback` si no hay imagen o si no pudo cargarse. */
export default function SafeImage({ src, fallback = null, alt, ...props }) {
  const [failed, onError] = useImageFailed(src)
  if (!src || failed) return fallback
  return <img src={src} alt={alt} onError={onError} {...props} />
}
