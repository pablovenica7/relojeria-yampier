const formatter = new Intl.NumberFormat('es-AR', {
  style: 'currency',
  currency: 'ARS',
  maximumFractionDigits: 0,
})

// Única fuente de verdad para mostrar precios. Los montos llegan del backend
// como enteros en pesos (priceARS, depositAmount): nunca se usa punto
// flotante para dinero.
export function formatCurrency(value) {
  // 0 se trata como "sin precio cargado" (precio a consultar), no como $0.
  if (value === null || value === undefined || value === 0 || value === '') return ''
  return formatter.format(value)
}

/**
 * Convierte lo que el Admin escribe ("150.000", "150000", "$ 150.000") en un
 * entero de pesos. Devuelve NaN si no es un monto entero válido (centavos,
 * signos o letras), para que el formulario muestre el error.
 */
export function parseARS(input) {
  const s = String(input ?? '').replace(/[$\s]/g, '').replace(/\./g, '')
  if (s === '') return 0
  if (!/^\d+$/.test(s)) return NaN
  const n = Number(s)
  return Number.isSafeInteger(n) ? n : NaN
}
