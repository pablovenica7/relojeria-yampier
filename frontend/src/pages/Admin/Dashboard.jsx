import { Link } from 'react-router-dom'
import { authRequest } from '../../services/api'
import EmptyState from '../../components/ui/EmptyState'
import Button from '../../components/ui/Button'
import { SkeletonLines } from '../../components/ui/Skeleton'
import { useAsync } from '../../hooks/useAsync'
import { reservationLabel } from '../../utils/reservationStatus'

const fetchDashboard = () => authRequest('/admin/dashboard')
const fmt = iso => new Date(iso).toLocaleString('es-AR', { dateStyle: 'short', timeStyle: 'short' })

// Cada número lleva a la vista ya filtrada para actuar en un click.
const CARDS = [
  { key: 'pendingReservations', label: 'Reservas pendientes', to: '/admin/reservas?status=pending', hint: 'Confirmar o cancelar' },
  { key: 'confirmedReservations', label: 'Reservas confirmadas', to: '/admin/reservas?status=confirmed', hint: 'Retienen 1 unidad' },
  { key: 'depositPaidReservations', label: 'Con seña recibida', to: '/admin/reservas?status=deposit_paid', hint: 'Esperando retiro' },
  { key: 'newInquiries', label: 'Consultas nuevas', to: '/admin/consultas?status=new', hint: 'Sin responder' },
  { key: 'outOfStock', label: 'Relojes sin stock', to: '/admin/relojes?availability=out_of_stock', hint: 'Activos con 0 unidades' },
  { key: 'lastUnit', label: 'Última unidad', to: '/admin/relojes?availability=last_unit', hint: 'Activos con 1 unidad' },
]

export default function Dashboard() {
  const { state, data, reload } = useAsync(fetchDashboard, [])

  return (
    <div>
      <div className="d-flex justify-content-between align-items-center flex-wrap gap-3 mb-5">
        <div>
          <span className="eyebrow">Panel Admin</span>
          <h2 className="mb-0">Inicio</h2>
        </div>
        <div className="d-flex gap-3 flex-wrap">
          <Link to="/admin/reservas/nueva" className="btn btn-secondary btn-sm">Nueva reserva</Link>
          <Link to="/admin/relojes/nuevo" className="btn btn-secondary btn-sm">Nuevo reloj</Link>
          <Button variant="ghost" size="sm" onClick={reload}>Actualizar</Button>
        </div>
      </div>

      {state === 'loading' && !data && <SkeletonLines count={4} />}
      {state === 'error' && (
        <EmptyState icon="!" title="No pudimos cargar el resumen"
          action={<Button variant="secondary" size="sm" onClick={reload}>Reintentar</Button>} />
      )}

      {data && (
        <>
          <div className="dashboard-grid">
            {CARDS.map(c => (
              <Link key={c.key} to={c.to} className="dashboard-card">
                <span className="dashboard-label">{c.label}</span>
                <span className="dashboard-value">{data[c.key]}</span>
                <span className="dashboard-hint">{c.hint}</span>
              </Link>
            ))}
          </div>

          <h3 className="detail-subtitle mt-5 pt-3">Reservas que vencen en las próximas 48 h</h3>
          {data.expiringSoon.length === 0 ? (
            <p className="text-faint-token">No hay reservas por vencer.</p>
          ) : (
            <div className="admin-table-wrap">
              <table className="admin-table">
                <thead><tr><th>Reloj</th><th>Cliente</th><th>Estado</th><th>Vence</th></tr></thead>
                <tbody>
                  {data.expiringSoon.map(r => (
                    <tr key={r.id}>
                      <td>{r.watchName}</td>
                      <td>{r.customerName}</td>
                      <td>{reservationLabel(r.status)}</td>
                      <td>{fmt(r.expiresAt)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
          <p className="text-faint-token mt-3 mb-0">
            Las reservas vencidas liberan su unidad automáticamente. Para extender una, editá su vencimiento en <Link to="/admin/reservas" className="link-underline">Reservas</Link>.
          </p>
        </>
      )}
    </div>
  )
}
