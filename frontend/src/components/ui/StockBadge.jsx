import { availabilityFor, availabilityLabel } from '../../utils/stock'

/**
 * Estado de stock de un reloj, derivado de su cantidad. Se diferencia por
 * estilo de borde/relleno y por texto, nunca por color (identidad
 * estrictamente blanco/negro/grises).
 */
export default function StockBadge({ quantity, className = '' }) {
  const value = availabilityFor(quantity)
  return <span className={`stock-badge is-${value} ${className}`}>{availabilityLabel(value)}</span>
}
