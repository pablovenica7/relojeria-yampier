import { useCallback, useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import {
  adminListReservations, adminSetReservationStatus, adminUpdateReservation,
} from '../../services/adminReservationsService'
import { adminListWatches } from '../../services/adminWatchesService'
import EmptyState from '../../components/ui/EmptyState'
import Button from '../../components/ui/Button'
import Pagination from '../../components/ui/Pagination'
import { SkeletonLines } from '../../components/ui/Skeleton'
import { useAsync } from '../../hooks/useAsync'
import { formatCurrency, parseARS } from '../../utils/formatCurrency'
import {
  ACTION_LABELS, DEPOSIT_METHODS, RESERVATION_LABELS, RESERVATION_STATUS as S, dateInputValue,
  depositMethodLabel, expiryFromDate, holdsStock, isExpirable, nextStatuses, reservationLabel,
} from '../../utils/reservationStatus'
import { DOCUMENT_TYPES, TAX_CONDITIONS, TAX_ID_TYPES, labelFrom } from '../../utils/customerOptions'
import swal from '../../utils/swal'

const PAGE_SIZE = 20
const fmtDate = iso => (iso ? new Date(iso).toLocaleString('es-AR', { dateStyle: 'short', timeStyle: 'short' }) : '—')

const CONFIRM_TEXT = {
  [S.CONFIRMED]: 'Se descuenta 1 unidad del stock del reloj.',
  [S.COMPLETED]: 'La venta queda registrada como cerrada: la unidad no vuelve al stock.',
  [S.CANCELLED]: 'Si la reserva tenía una unidad retenida, vuelve al stock. Si hubo seña, su devolución se gestiona en el local.',
}

/** Datos completos del cliente (documento, fiscal, domicilio). Solo en el Admin. */
function CustomerDetails({ c }) {
  const a = c.billingAddress
  return (
    <details className="admin-customer">
      <summary>Datos del cliente (cuenta web)</summary>
      <dl className="admin-reservation-grid">
        <div><dt>Nombre legal</dt><dd>{c.legalName}</dd></div>
        <div><dt>Documento</dt><dd>{labelFrom(DOCUMENT_TYPES, c.documentType)} {c.documentNumber}</dd></div>
        <div><dt>Condición fiscal</dt><dd>{labelFrom(TAX_CONDITIONS, c.taxCondition)}</dd></div>
        <div><dt>CUIT / CUIL / CDI</dt><dd>{c.taxId ? `${labelFrom(TAX_ID_TYPES, c.taxIdType)} ${c.taxId}` : '—'}</dd></div>
        <div><dt>Teléfono</dt><dd>{c.phone}</dd></div>
        <div><dt>Email</dt><dd>{c.email}</dd></div>
        <div className="span-2">
          <dt>Domicilio de facturación</dt>
          <dd>
            {a
              ? `${a.street} ${a.number}${a.floor ? `, piso ${a.floor}` : ''}${a.apartment ? ` dto. ${a.apartment}` : ''} — ${a.postalCode} ${a.city}, ${a.province}, ${a.country}`
              : 'No informado'}
          </dd>
        </div>
      </dl>
    </details>
  )
}

function DepositForm({ r, busy, onSubmit, onCancel }) {
  const [amount, setAmount] = useState(r.depositAmount ? String(r.depositAmount) : '')
  const [method, setMethod] = useState('')
  const [error, setError] = useState('')
  const submit = (e) => {
    e.preventDefault()
    const n = parseARS(amount)
    if (Number.isNaN(n) || n <= 0) return setError('Ingresá un monto entero mayor a 0')
    if (r.priceAtReservation > 0 && n > r.priceAtReservation) return setError('La seña no puede superar el precio reservado')
    if (!method) return setError('Elegí cómo se recibió la seña')
    setError('')
    onSubmit(n, method)
  }
  return (
    <form className="admin-deposit-form" onSubmit={submit} noValidate>
      <div className="field">
        <label htmlFor={`dep-amount-${r.id}`} className="field-label">Monto recibido (ARS)</label>
        <input id={`dep-amount-${r.id}`} className="field-input" inputMode="numeric" value={amount} onChange={e => setAmount(e.target.value)} />
      </div>
      <div className="field">
        <label htmlFor={`dep-method-${r.id}`} className="field-label">Método</label>
        <select id={`dep-method-${r.id}`} className="field-select" value={method} onChange={e => setMethod(e.target.value)}>
          <option value="">Elegí…</option>
          {DEPOSIT_METHODS.map(m => <option key={m.value} value={m.value}>{m.label}</option>)}
        </select>
      </div>
      <div className="d-flex gap-2 align-items-end">
        <Button type="submit" size="sm" variant="primary" loading={busy}>Registrar seña</Button>
        <Button type="button" size="sm" variant="ghost" onClick={onCancel}>Cancelar</Button>
      </div>
      <span className="field-error" role={error ? 'alert' : undefined}>{error}</span>
      <p className="field-hint mb-0">Solo es un registro administrativo: el cobro se hace en el local.</p>
    </form>
  )
}

function ReservationCard({ r, onUpdated }) {
  const [showDeposit, setShowDeposit] = useState(false)
  const [notes, setNotes] = useState(r.notes || '')
  const [expiry, setExpiry] = useState(dateInputValue(r.expiresAt))
  const [busy, setBusy] = useState(false)
  useEffect(() => { setNotes(r.notes || ''); setExpiry(dateInputValue(r.expiresAt)) }, [r])

  const run = async (fn) => {
    setBusy(true)
    try {
      onUpdated(await fn())
    } catch (e) {
      swal.fire({ icon: 'error', title: 'No se pudo actualizar la reserva', text: e.message })
    } finally {
      setBusy(false)
    }
  }

  const transition = async (to) => {
    if (to === S.DEPOSIT_PAID) {
      setShowDeposit(true)
      return
    }
    const { isConfirmed } = await swal.fire({
      icon: to === S.CANCELLED ? 'warning' : 'question',
      title: `${ACTION_LABELS[to]}?`,
      text: CONFIRM_TEXT[to],
      showCancelButton: true,
      confirmButtonText: 'Sí, continuar',
      cancelButtonText: 'Volver',
    })
    if (isConfirmed) run(() => adminSetReservationStatus(r.id, to))
  }

  const notesDirty = notes.trim() !== (r.notes || '')
  const expiryDirty = expiry !== dateInputValue(r.expiresAt)
  const saveDetails = () => {
    const data = {}
    if (notesDirty) data.notes = notes
    if (expiryDirty) {
      if (expiry) data.expiresAt = expiryFromDate(expiry)
      else data.clearExpiry = true
    }
    run(() => adminUpdateReservation(r.id, data))
  }

  return (
    <article className={`admin-inquiry ${r.status === S.PENDING ? 'is-unread' : ''}`}>
      <div className="d-flex justify-content-between flex-wrap gap-2">
        <div>
          <span className={`admin-badge me-2 ${r.status === S.PENDING ? 'is-new' : ''}`}>{reservationLabel(r.status)}</span>
          {r.source === 'web' && <span className="admin-badge me-2">Web</span>}
          <strong>{r.watchNameSnapshot}</strong>{' '}
          <span className="meta">{r.watchModelSnapshot}</span>
        </div>
        <span className="meta">
          Creada {fmtDate(r.createdAt)}
          {r.confirmedAt && <> · Confirmada {fmtDate(r.confirmedAt)}</>}
          {r.completedAt && <> · Completada {fmtDate(r.completedAt)}</>}
          {r.cancelledAt && <> · Cancelada {fmtDate(r.cancelledAt)}</>}
          {r.expiredAt && <> · Vencida {fmtDate(r.expiredAt)}</>}
        </span>
      </div>

      <dl className="admin-reservation-grid">
        <div><dt>Cliente</dt><dd>{r.customerName}</dd></div>
        <div><dt>Teléfono</dt><dd>{r.phone ? <a href={`tel:${r.phone}`}>{r.phone}</a> : '—'}</dd></div>
        <div><dt>Email</dt><dd>{r.email ? <a href={`mailto:${r.email}`}>{r.email}</a> : '—'}</dd></div>
        <div><dt>Precio reservado</dt><dd>{formatCurrency(r.priceAtReservation) || 'A definir'}</dd></div>
        <div>
          <dt>Seña</dt>
          <dd>
            {formatCurrency(r.depositAmount) || '—'}
            {r.depositMethod && <span className="d-block text-faint-token">{depositMethodLabel(r.depositMethod)} · {fmtDate(r.depositPaidAt)}</span>}
          </dd>
        </div>
        <div><dt>Vence</dt><dd>{isExpirable(r.status) ? fmtDate(r.expiresAt) : '—'}</dd></div>
        <div><dt>Stock</dt><dd>{holdsStock(r.status) ? 'Retiene 1 unidad' : r.status === S.COMPLETED ? 'Vendida' : 'No retiene'}</dd></div>
      </dl>
      {r.customerNotes && <p className="meta mt-3 mb-0">Comentario del cliente: <span className="text-strong-token">{r.customerNotes}</span></p>}
      {r.customer && <CustomerDetails c={r.customer} />}

      <div className="admin-inquiry-controls">
        <div className="field flex-grow-1">
          <label htmlFor={`rn-${r.id}`} className="field-label">Notas</label>
          <textarea
            id={`rn-${r.id}`} className="field-textarea" rows={2} maxLength={2000}
            value={notes} onChange={e => setNotes(e.target.value)}
          />
        </div>
        {isExpirable(r.status) && (
          <div className="field">
            <label htmlFor={`re-${r.id}`} className="field-label">Vencimiento</label>
            <input
              id={`re-${r.id}`} type="date" className="field-input"
              value={expiry} onChange={e => setExpiry(e.target.value)}
            />
          </div>
        )}
      </div>

      <div className="d-flex flex-wrap gap-3 mt-3 align-items-center">
        {(notesDirty || expiryDirty) && (
          <Button size="sm" variant="secondary" loading={busy} onClick={saveDetails}>Guardar cambios</Button>
        )}
        {showDeposit && (
          <DepositForm
            r={r} busy={busy} onCancel={() => setShowDeposit(false)}
            onSubmit={(amount, method) => run(() => adminSetReservationStatus(r.id, S.DEPOSIT_PAID, amount, method)).then(() => setShowDeposit(false))}
          />
        )}
        {!showDeposit && nextStatuses(r.status).map(to => (
          <Button
            key={to} size="sm" variant={to === S.CANCELLED ? 'ghost' : 'secondary'}
            disabled={busy} onClick={() => transition(to)}
          >
            {ACTION_LABELS[to]}
          </Button>
        ))}
      </div>
    </article>
  )
}

export default function Reservations() {
  const [params] = useSearchParams()
  // Filtro inicial desde la URL (links del Dashboard, ej: ?status=pending).
  const [status, setStatus] = useState(params.get('status') || '')
  const [watchId, setWatchId] = useState('')
  const [from, setFrom] = useState('')
  const [to, setTo] = useState('')
  const [page, setPage] = useState(1)
  const [watches, setWatches] = useState([])

  useEffect(() => {
    // Para el filtro "Reloj" (incluye archivados: pueden tener reservas).
    adminListWatches({ status: 'all', sort: 'name_asc', limit: 100 })
      .then(d => setWatches(d.items))
      .catch(() => {})
  }, [])

  const fetcher = useCallback(
    () => adminListReservations({ status, watchId, from, to, page, limit: PAGE_SIZE }),
    [status, watchId, from, to, page]
  )
  const { state, data, reload, setData } = useAsync(fetcher, [status, watchId, from, to, page])
  const items = data?.items || []
  const changeFilter = setter => e => { setter(e.target.value); setPage(1) }
  const replace = updated =>
    setData(prev => ({ ...prev, items: prev.items.map(x => (x.id === updated.id ? updated : x)) }))

  return (
    <div>
      <div className="d-flex justify-content-between align-items-center flex-wrap gap-3 mb-4">
        <div>
          <span className="eyebrow">Panel Admin</span>
          <h2 className="mb-0">Reservas</h2>
        </div>
        <Link to="/admin/reservas/nueva" className="btn btn-primary">Nueva reserva</Link>
      </div>
      <p className="text-muted-token mb-5">
        Una reserva confirmada retiene una unidad del stock. Si se cancela o vence
        antes de completarse, la unidad vuelve al stock automáticamente.
      </p>

      <div className="admin-toolbar">
        <div className="field">
          <label htmlFor="r-status" className="field-label">Estado</label>
          <select id="r-status" className="field-select" value={status} onChange={changeFilter(setStatus)}>
            <option value="">Todos</option>
            {Object.entries(RESERVATION_LABELS).map(([v, l]) => <option key={v} value={v}>{l}</option>)}
          </select>
        </div>
        <div className="field">
          <label htmlFor="r-watch" className="field-label">Reloj</label>
          <select id="r-watch" className="field-select" value={watchId} onChange={changeFilter(setWatchId)}>
            <option value="">Todos</option>
            {watches.map(w => <option key={w.id} value={w.id}>{w.name}</option>)}
          </select>
        </div>
        <div className="field">
          <label htmlFor="r-from" className="field-label">Desde</label>
          <input id="r-from" type="date" className="field-input" value={from} onChange={changeFilter(setFrom)} />
        </div>
        <div className="field">
          <label htmlFor="r-to" className="field-label">Hasta</label>
          <input id="r-to" type="date" className="field-input" value={to} onChange={changeFilter(setTo)} />
        </div>
      </div>

      {state === 'loading' && !data && <SkeletonLines count={4} />}

      {state === 'error' && (
        <EmptyState
          icon="!" title="No pudimos cargar las reservas"
          action={<Button variant="secondary" size="sm" onClick={reload}>Reintentar</Button>}
        />
      )}

      {data && items.length === 0 && state !== 'error' && (
        <EmptyState icon="—" title="No hay reservas con estos filtros" />
      )}

      {data && items.length > 0 && state !== 'error' && (
        <>
          <p className="admin-count">{data.total} {data.total === 1 ? 'reserva' : 'reservas'}</p>
          <div className="d-grid gap-3">
            {items.map(r => <ReservationCard key={r.id} r={r} onUpdated={replace} />)}
          </div>
          <Pagination page={data.page} pages={data.pages} onChange={setPage} />
        </>
      )}
    </div>
  )
}
