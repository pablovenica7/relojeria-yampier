// Estados de reserva: deben coincidir con backend/reservations.go.
export const RESERVATION_STATUS = {
  PENDING: 'pending',
  CONFIRMED: 'confirmed',
  DEPOSIT_PAID: 'deposit_paid',
  COMPLETED: 'completed',
  CANCELLED: 'cancelled',
  EXPIRED: 'expired',
}

const S = RESERVATION_STATUS

export const RESERVATION_LABELS = {
  [S.PENDING]: 'Pendiente',
  [S.CONFIRMED]: 'Confirmada',
  [S.DEPOSIT_PAID]: 'Seña paga',
  [S.COMPLETED]: 'Completada',
  [S.CANCELLED]: 'Cancelada',
  [S.EXPIRED]: 'Vencida',
}

// Lo que ve el cliente en "Mis reservas" (más explicativo que el del Admin).
export const CUSTOMER_LABELS = {
  [S.PENDING]: 'Pendiente de confirmación',
  [S.CONFIRMED]: 'Confirmada',
  [S.DEPOSIT_PAID]: 'Seña recibida',
  [S.COMPLETED]: 'Compra completada',
  [S.CANCELLED]: 'Cancelada',
  [S.EXPIRED]: 'Vencida',
}

export const CUSTOMER_STATUS_HELP = {
  [S.PENDING]: 'Recibimos tu solicitud. La relojería revisará la disponibilidad y te contactará para confirmarla. Una solicitud pendiente todavía no bloquea la unidad.',
  [S.CONFIRMED]: 'Reserva confirmada: la unidad está apartada para vos. Si corresponde, se coordinará la seña con la relojería.',
  [S.DEPOSIT_PAID]: 'Registramos tu seña. Te esperamos en el local para completar el pago.',
  [S.COMPLETED]: '¡Gracias por tu compra!',
  [S.CANCELLED]: 'Esta reserva fue cancelada.',
  [S.EXPIRED]: 'Esta reserva venció sin completarse.',
}

export function customerReservationLabel(status) {
  return CUSTOMER_LABELS[status] || status
}

// Métodos de seña: solo registro administrativo (no hay cobro online).
export const DEPOSIT_METHODS = [
  { value: 'cash', label: 'Efectivo' },
  { value: 'bank_transfer', label: 'Transferencia bancaria' },
  { value: 'in_store_card', label: 'Tarjeta en el local' },
  { value: 'other', label: 'Otro' },
]

export const depositMethodLabel = (m) => DEPOSIT_METHODS.find(o => o.value === m)?.label || ''

// Transiciones que el Admin puede disparar a mano (el backend las valida
// igual; esto solo decide qué botones mostrar). "Vencida" no se ofrece: la
// aplica el sistema al pasar la fecha de vencimiento.
const TRANSITIONS = {
  [S.PENDING]: [S.CONFIRMED, S.CANCELLED],
  [S.CONFIRMED]: [S.DEPOSIT_PAID, S.COMPLETED, S.CANCELLED],
  [S.DEPOSIT_PAID]: [S.COMPLETED, S.CANCELLED],
}

// Texto de cada acción en el Admin.
export const ACTION_LABELS = {
  [S.CONFIRMED]: 'Confirmar (retiene 1 unidad)',
  [S.DEPOSIT_PAID]: 'Registrar seña',
  [S.COMPLETED]: 'Completar venta',
  [S.CANCELLED]: 'Cancelar',
}

export function reservationLabel(status) {
  return RESERVATION_LABELS[status] || status
}

export function nextStatuses(status) {
  return TRANSITIONS[status] || []
}

export function isFinalStatus(status) {
  return nextStatuses(status).length === 0
}

/** Solo pendientes y confirmadas vencen solas (con seña paga decide el local). */
export function isExpirable(status) {
  return status === S.PENDING || status === S.CONFIRMED
}

/**
 * Convierte la fecha de un <input type="date"> (AAAA-MM-DD) en el fin de ese
 * día en hora local, como ISO para la API. Devuelve null si está vacía.
 */
export function expiryFromDate(dateStr) {
  if (!dateStr) return null
  const d = new Date(`${dateStr}T23:59:00`)
  return Number.isNaN(d.getTime()) ? null : d.toISOString()
}

/** Inversa de expiryFromDate: ISO -> AAAA-MM-DD (hora local) para el input. */
export function dateInputValue(iso) {
  if (!iso) return ''
  const d = new Date(iso)
  const pad = n => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

/** Estados que tienen una unidad del reloj retenida. */
export function holdsStock(status) {
  return status === S.CONFIRMED || status === S.DEPOSIT_PAID
}
