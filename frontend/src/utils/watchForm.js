import { parseARS } from './formatCurrency'

// Lógica del formulario de reloj del Admin, separada del componente para
// poder testearla. El backend vuelve a validar todo: esto es solo UX.

export const emptyWatchForm = {
  brand: 'Casio', name: '', model: '', gender: 'hombre', description: '',
  image: '', price: '', stockQuantity: '0', specs: '',
  stockMoveType: 'manual_adjustment', stockMoveReason: '',
}

const MODEL_RE = /^[A-Za-z0-9][A-Za-z0-9 ./_-]*$/

export function watchToForm(w) {
  return {
    brand: w.brand || '', name: w.name || '', model: w.model || '', gender: w.gender || 'hombre',
    description: w.description || '', image: w.image || '',
    price: w.priceARS ? String(w.priceARS) : '',
    stockQuantity: String(w.stockQuantity ?? 0),
    specs: (w.specs || []).join('\n'),
    stockMoveType: 'manual_adjustment', stockMoveReason: '',
  }
}

function parseQuantity(value) {
  const s = String(value ?? '').trim()
  if (!/^\d+$/.test(s)) return NaN
  return Number(s)
}

export function validateWatchForm(form) {
  const errors = {}
  if (!form.name.trim()) errors.name = 'El nombre es obligatorio'
  if (!form.brand.trim()) errors.brand = 'La marca es obligatoria'
  if (!form.model.trim()) errors.model = 'El modelo es obligatorio (identifica al reloj y no puede repetirse)'
  else if (!MODEL_RE.test(form.model.trim())) errors.model = 'Usá solo letras, números, espacios y - . / _'
  if (Number.isNaN(parseARS(form.price))) errors.price = 'Ingresá un precio entero en pesos, sin centavos (ej: 150000)'
  const qty = parseQuantity(form.stockQuantity)
  if (Number.isNaN(qty)) errors.stockQuantity = 'Ingresá una cantidad entera mayor o igual a 0'
  return errors
}

/**
 * Arma el cuerpo para la API. `baseStock` es la cantidad que tenía el reloj
 * al abrir el formulario: el backend solo pisa el stock si no cambió
 * mientras tanto (por ejemplo, por una reserva confirmada en paralelo).
 */
export function toWatchPayload(form, baseStock) {
  const payload = {
    brand: form.brand.trim(),
    name: form.name.trim(),
    model: form.model.trim().toUpperCase(),
    gender: form.gender,
    description: form.description.trim(),
    image: form.image,
    priceARS: parseARS(form.price),
    stockQuantity: parseQuantity(form.stockQuantity),
    specs: form.specs.split('\n').map(s => s.trim()).filter(Boolean),
  }
  if (baseStock !== undefined && baseStock !== null) payload.stockQuantityBase = baseStock
  // Tipo y motivo del movimiento de stock (si el stock cambió, quedan en el historial).
  payload.stockMoveType = form.stockMoveType || 'manual_adjustment'
  if (form.stockMoveReason?.trim()) payload.stockMoveReason = form.stockMoveReason.trim()
  return payload
}
