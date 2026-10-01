// Tipos de movimiento de stock: deben coincidir con backend/models.go.
export const MOVEMENT_TYPES = [
  { value: 'stock_entry', label: 'Ingreso de mercadería' },
  { value: 'manual_adjustment', label: 'Ajuste manual' },
  { value: 'correction', label: 'Corrección de inventario' },
  { value: 'reservation_hold', label: 'Reserva confirmada' },
  { value: 'reservation_release', label: 'Reserva liberada' },
  { value: 'sale', label: 'Venta completada' },
]

// Los que el Admin puede elegir al cambiar stock a mano.
export const MANUAL_MOVEMENT_TYPES = MOVEMENT_TYPES.filter(t =>
  ['stock_entry', 'manual_adjustment', 'correction'].includes(t.value))

export const movementLabel = (type) => MOVEMENT_TYPES.find(t => t.value === type)?.label || type
