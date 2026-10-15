// Achica las fotos ANTES de subirlas desde el Admin. Una foto de celular
// (4000 px, 3-5 MB) se descargaría entera en cada visita al catálogo y
// ocuparía lugar en la base (las imágenes se guardan en MongoDB). El backend
// igual valida el contenido real; si algo falla acá, se sube el original.

export const MAX_IMAGE_SIDE = 1600
// Por debajo de esto no vale la pena recomprimir una foto que ya entra en el tamaño.
const SMALL_ENOUGH_BYTES = 600 * 1024
const QUALITY = 0.85

/** Medidas finales manteniendo la proporción; nunca agranda. */
export function targetSize(width, height, max = MAX_IMAGE_SIDE) {
  const scale = Math.min(1, max / Math.max(width, height))
  return { width: Math.round(width * scale), height: Math.round(height * scale) }
}

function canvasToBlob(canvas, type) {
  return new Promise(resolve => canvas.toBlob(resolve, type, QUALITY))
}

const EXT = { 'image/jpeg': '.jpg', 'image/png': '.png', 'image/webp': '.webp' }

/** Devuelve un File más liviano, o el original si no hace falta o no se puede. */
export async function resizeImageFile(file) {
  if (!EXT[file?.type] || typeof createImageBitmap !== 'function') return file
  let bitmap
  try {
    bitmap = await createImageBitmap(file)
    const size = targetSize(bitmap.width, bitmap.height)
    if (size.width === bitmap.width && file.size <= SMALL_ENOUGH_BYTES) return file

    const canvas = document.createElement('canvas')
    canvas.width = size.width
    canvas.height = size.height
    canvas.getContext('2d').drawImage(bitmap, 0, 0, size.width, size.height)

    // JPEG queda en JPEG. PNG/WEBP pueden tener transparencia: se intenta WEBP
    // y, si el navegador no lo genera, se mantiene el formato original (PNG)
    // o se pasa a JPEG.
    let blob
    if (file.type === 'image/jpeg') {
      blob = await canvasToBlob(canvas, 'image/jpeg')
    } else {
      blob = await canvasToBlob(canvas, 'image/webp')
      if (blob?.type !== 'image/webp') blob = await canvasToBlob(canvas, file.type === 'image/png' ? 'image/png' : 'image/jpeg')
    }
    if (!blob || blob.size >= file.size) return file
    const name = file.name.replace(/\.[^.]+$/, '') + EXT[blob.type]
    return new File([blob], name, { type: blob.type })
  } catch {
    return file
  } finally {
    bitmap?.close?.()
  }
}
