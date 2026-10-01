import { useCallback, useEffect, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { authRequest } from '../../services/api'
import { toQuery } from '../../services/watchesService'
import { adminListWatches } from '../../services/adminWatchesService'
import EmptyState from '../../components/ui/EmptyState'
import Button from '../../components/ui/Button'
import Pagination from '../../components/ui/Pagination'
import { SkeletonLines } from '../../components/ui/Skeleton'
import { useAsync } from '../../hooks/useAsync'
import { MOVEMENT_TYPES, movementLabel } from '../../utils/stockMovements'

const fmt = iso => new Date(iso).toLocaleString('es-AR', { dateStyle: 'short', timeStyle: 'short' })

/** Historial de movimientos de stock (libro de inventario). Solo lectura. */
export default function StockMovements() {
  const [params] = useSearchParams()
  const [watchId, setWatchId] = useState(params.get('watchId') || '')
  const [type, setType] = useState('')
  const [from, setFrom] = useState('')
  const [to, setTo] = useState('')
  const [page, setPage] = useState(1)
  const [watches, setWatches] = useState([])

  useEffect(() => {
    adminListWatches({ status: 'all', sort: 'name_asc', limit: 100 }).then(d => setWatches(d.items)).catch(() => {})
  }, [])

  const fetcher = useCallback(
    () => authRequest(`/admin/stock-movements${toQuery({ watchId, type, from, to, page, limit: 50 })}`),
    [watchId, type, from, to, page]
  )
  const { state, data, reload } = useAsync(fetcher, [watchId, type, from, to, page])
  const items = data?.items || []
  const changeFilter = setter => e => { setter(e.target.value); setPage(1) }

  return (
    <div>
      <span className="eyebrow">Panel Admin</span>
      <h2 className="mb-3">Movimientos de stock</h2>
      <p className="text-muted-token mb-5">
        Cada cambio de stock queda registrado con el stock anterior y posterior.
        Las reservas confirmadas descuentan 1 unidad; si se cancelan o vencen, la devuelven.
        Una venta completada no vuelve a descontar (la unidad ya estaba retenida).
      </p>

      <div className="admin-toolbar">
        <div className="field">
          <label htmlFor="m-watch" className="field-label">Reloj</label>
          <select id="m-watch" className="field-select" value={watchId} onChange={changeFilter(setWatchId)}>
            <option value="">Todos</option>
            {watches.map(w => <option key={w.id} value={w.id}>{w.name}</option>)}
          </select>
        </div>
        <div className="field">
          <label htmlFor="m-type" className="field-label">Tipo</label>
          <select id="m-type" className="field-select" value={type} onChange={changeFilter(setType)}>
            <option value="">Todos</option>
            {MOVEMENT_TYPES.map(t => <option key={t.value} value={t.value}>{t.label}</option>)}
          </select>
        </div>
        <div className="field">
          <label htmlFor="m-from" className="field-label">Desde</label>
          <input id="m-from" type="date" className="field-input" value={from} onChange={changeFilter(setFrom)} />
        </div>
        <div className="field">
          <label htmlFor="m-to" className="field-label">Hasta</label>
          <input id="m-to" type="date" className="field-input" value={to} onChange={changeFilter(setTo)} />
        </div>
      </div>

      {state === 'loading' && !data && <SkeletonLines count={4} />}
      {state === 'error' && (
        <EmptyState icon="!" title="No pudimos cargar los movimientos"
          action={<Button variant="secondary" size="sm" onClick={reload}>Reintentar</Button>} />
      )}
      {data && items.length === 0 && state !== 'error' && <EmptyState icon="—" title="No hay movimientos con estos filtros" />}

      {data && items.length > 0 && (
        <>
          <div className="admin-table-wrap">
            <table className="admin-table">
              <thead>
                <tr><th>Fecha</th><th>Reloj</th><th>Tipo</th><th>Cantidad</th><th>Stock</th><th>Reserva</th><th>Motivo</th><th>Por</th></tr>
              </thead>
              <tbody>
                {items.map(m => (
                  <tr key={m.id}>
                    <td className="text-nowrap">{fmt(m.createdAt)}</td>
                    <td>{m.watchName}</td>
                    <td>{movementLabel(m.type)}</td>
                    <td className="tabular">{m.quantityDelta > 0 ? `+${m.quantityDelta}` : m.quantityDelta}</td>
                    <td className="tabular text-nowrap">{m.stockBefore} → {m.stockAfter}</td>
                    <td className="text-faint-token">{m.reservationId ? `…${m.reservationId.slice(-6)}` : '—'}</td>
                    <td>{m.reason || '—'}</td>
                    <td className="text-faint-token">{m.createdBy}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <Pagination page={data.page} pages={data.pages} onChange={setPage} />
        </>
      )}
    </div>
  )
}
