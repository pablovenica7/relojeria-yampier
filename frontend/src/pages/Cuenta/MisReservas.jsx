import { Link } from 'react-router-dom'
import { cancelMyReservation, getMyReservations } from '../../services/customerService'
import EmptyState from '../../components/ui/EmptyState'
import Button from '../../components/ui/Button'
import { SkeletonLines } from '../../components/ui/Skeleton'
import { useAsync } from '../../hooks/useAsync'
import { formatCurrency } from '../../utils/formatCurrency'
import { imageUrl } from '../../utils/imageUrl'
import { CUSTOMER_STATUS_HELP, RESERVATION_STATUS as S, customerReservationLabel, isFinalStatus } from '../../utils/reservationStatus'
import { business } from '../../utils/siteConfig'
import swal from '../../utils/swal'

const fmtDate = iso => (iso ? new Date(iso).toLocaleDateString('es-AR', { dateStyle: 'medium' }) : '—')

export default function MisReservas() {
  const { state, data: items, reload, setData } = useAsync(getMyReservations, [])

  const cancel = async (r) => {
    const { isConfirmed } = await swal.fire({
      icon: 'question',
      title: '¿Cancelar la solicitud?',
      text: `Se cancelará tu solicitud de reserva del ${r.watchName}.`,
      showCancelButton: true,
      confirmButtonText: 'Sí, cancelar',
      cancelButtonText: 'Volver',
    })
    if (!isConfirmed) return
    try {
      const updated = await cancelMyReservation(r.id)
      setData(prev => prev.map(x => (x.id === updated.id ? updated : x)))
    } catch (e) {
      swal.fire({ icon: 'error', title: 'No se pudo cancelar', text: e.message })
    }
  }

  return (
    <section className="section">
      <div className="container account-container">
        <span className="eyebrow">Mi cuenta</span>
        <h1 className="h2 mb-3">Mis reservas</h1>
        <p className="mb-6">El pago final se realiza en nuestro local. No cobramos nada online.</p>

        {state === 'loading' && <SkeletonLines count={4} />}
        {state === 'error' && (
          <EmptyState icon="!" title="No pudimos cargar tus reservas"
            action={<Button variant="secondary" size="sm" onClick={reload}>Reintentar</Button>} />
        )}
        {state === 'success' && items.length === 0 && (
          <EmptyState icon="—" title="Todavía no solicitaste reservas"
            action={<Link to="/relojes" className="btn btn-primary btn-sm">Ver relojes</Link>} />
        )}

        {state === 'success' && items.length > 0 && (
          <ul className="my-reservations">
            {items.map(r => (
              <li key={r.id} className="my-reservation">
                <div className="my-reservation-img">
                  {r.watchImage
                    ? <img src={imageUrl(r.watchImage)} alt={`Reloj ${r.watchName}`} loading="lazy" width="120" height="120" />
                    : <span aria-hidden="true">—</span>}
                </div>
                <div className="my-reservation-body">
                  <div className="d-flex justify-content-between flex-wrap gap-2">
                    <div>
                      <h2 className="h5 mb-1">{r.watchName}</h2>
                      <p className="text-faint-token mb-0">Modelo {r.watchModel}</p>
                    </div>
                    <span className={`admin-badge ${r.status === S.PENDING ? '' : r.status === S.CONFIRMED || r.status === S.DEPOSIT_PAID ? 'is-new' : ''}`}>
                      {customerReservationLabel(r.status)}
                    </span>
                  </div>
                  <dl className="admin-reservation-grid">
                    <div><dt>Precio reservado</dt><dd>{formatCurrency(r.priceAtReservation) || 'A confirmar'}</dd></div>
                    <div><dt>Solicitada</dt><dd>{fmtDate(r.createdAt)}</dd></div>
                    <div><dt>Seña</dt><dd>{formatCurrency(r.depositAmount) || '—'}</dd></div>
                    <div><dt>Vence</dt><dd>{fmtDate(r.expiresAt)}</dd></div>
                  </dl>
                  <p className="my-reservation-help">{CUSTOMER_STATUS_HELP[r.status]}</p>
                  {r.canCancel && (
                    <Button variant="secondary" size="sm" onClick={() => cancel(r)}>Cancelar solicitud</Button>
                  )}
                  {!r.canCancel && !isFinalStatus(r.status) && (
                    <p className="text-faint-token mb-0">
                      Contactanos para cancelar esta reserva: {business.phone.display} o desde <Link to="/contacto" className="link-underline">Contacto</Link>.
                    </p>
                  )}
                </div>
              </li>
            ))}
          </ul>
        )}
      </div>
    </section>
  )
}
