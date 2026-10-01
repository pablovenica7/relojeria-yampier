// Disponibilidad derivada de stockQuantity. El backend ya la calcula
// (campo `availability`), pero se deriva también acá para que la UI nunca
// muestre algo distinto a la cantidad real. Debe coincidir con
// availabilityFor() en backend/catalog.go.
export const AVAILABILITY = {
  IN_STOCK: 'in_stock',
  LAST_UNIT: 'last_unit',
  OUT_OF_STOCK: 'out_of_stock',
}

const LABELS = {
  [AVAILABILITY.IN_STOCK]: 'En stock',
  [AVAILABILITY.LAST_UNIT]: 'Última unidad',
  [AVAILABILITY.OUT_OF_STOCK]: 'Sin stock',
}

// Opciones de filtro del Admin (`available` = cualquier cantidad >= 1).
export const AVAILABILITY_FILTERS = [
  { value: '', label: 'Todo el stock' },
  { value: 'available', label: 'Disponibles' },
  { value: AVAILABILITY.IN_STOCK, label: LABELS[AVAILABILITY.IN_STOCK] },
  { value: AVAILABILITY.LAST_UNIT, label: LABELS[AVAILABILITY.LAST_UNIT] },
  { value: AVAILABILITY.OUT_OF_STOCK, label: LABELS[AVAILABILITY.OUT_OF_STOCK] },
]

export function availabilityFor(quantity) {
  const n = Number(quantity)
  if (!Number.isFinite(n) || n <= 0) return AVAILABILITY.OUT_OF_STOCK
  if (n === 1) return AVAILABILITY.LAST_UNIT
  return AVAILABILITY.IN_STOCK
}

export function availabilityLabel(availability) {
  return LABELS[availability] || LABELS[AVAILABILITY.OUT_OF_STOCK]
}

/** true si el reloj se puede consultar para reservar (al menos 1 unidad). */
export function isAvailable(quantity) {
  return availabilityFor(quantity) !== AVAILABILITY.OUT_OF_STOCK
}
