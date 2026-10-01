import { useCallback, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import {
  adjustWatchStock, adminListWatches, archiveWatch, deleteWatch, restoreWatch,
} from '../../services/adminWatchesService'
import { imageUrl } from '../../utils/imageUrl'
import { formatCurrency } from '../../utils/formatCurrency'
import { AVAILABILITY_FILTERS } from '../../utils/stock'
import EmptyState from '../../components/ui/EmptyState'
import Button from '../../components/ui/Button'
import StockBadge from '../../components/ui/StockBadge'
import Pagination from '../../components/ui/Pagination'
import { SkeletonLines } from '../../components/ui/Skeleton'
import { useAsync } from '../../hooks/useAsync'
import { useDebouncedValue } from '../../hooks/useDebouncedValue'
import swal from '../../utils/swal'

const SORTS = [
  { value: 'newest', label: 'Más recientes' },
  { value: 'name_asc', label: 'Nombre (A-Z)' },
  { value: 'name_desc', label: 'Nombre (Z-A)' },
  { value: 'price_asc', label: 'Precio (menor a mayor)' },
  { value: 'price_desc', label: 'Precio (mayor a menor)' },
]
const PAGE_SIZE = 25

export default function WatchesList() {
  const [params] = useSearchParams()
  const [search, setSearch] = useState('')
  // Filtro inicial desde la URL (links del Dashboard, ej: ?availability=out_of_stock).
  const [availability, setAvailability] = useState(params.get('availability') || '')
  const [status, setStatus] = useState('active')
  const [sort, setSort] = useState('newest')
  const [page, setPage] = useState(1)
  const debouncedSearch = useDebouncedValue(search.trim())

  // Cada filtro vuelve a la página 1.
  const changeFilter = setter => e => { setter(e.target.value); setPage(1) }

  const fetcher = useCallback(
    () => adminListWatches({ search: debouncedSearch, availability, status, sort, page, limit: PAGE_SIZE }),
    [debouncedSearch, availability, status, sort, page]
  )
  const { state, data, reload, setData } = useAsync(fetcher, [debouncedSearch, availability, status, sort, page])
  const items = data?.items || []
  const stats = data?.stats

  const replaceItem = (updated) =>
    setData(prev => ({ ...prev, items: prev.items.map(x => (x.id === updated.id ? updated : x)) }))

  const changeStock = async (w, delta) => {
    try {
      replaceItem(await adjustWatchStock(w.id, delta))
      // Los contadores (sin stock / última unidad) dependen del cambio.
      reload()
    } catch (e) {
      swal.fire({ icon: 'error', title: 'No se pudo actualizar el stock', text: e.message })
    }
  }

  const toggleArchive = async (w) => {
    const archiving = !w.archivedAt
    if (archiving) {
      const result = await swal.fire({
        icon: 'question',
        title: `¿Archivar "${w.name}"?`,
        text: 'Deja de mostrarse en el sitio, pero se conservan sus consultas y reservas. Podés restaurarlo cuando quieras.',
        showCancelButton: true,
        confirmButtonText: 'Archivar',
        cancelButtonText: 'Cancelar',
      })
      if (!result.isConfirmed) return
    }
    try {
      await (archiving ? archiveWatch(w.id) : restoreWatch(w.id))
      reload()
    } catch (e) {
      // El backend exige confirmación explícita si hay reservas activas.
      if (archiving && e.code === 'ACTIVE_RESERVATIONS') {
        const n = e.data?.activeReservations
        const again = await swal.fire({
          icon: 'warning',
          title: `Este reloj tiene ${n} ${n === 1 ? 'reserva activa' : 'reservas activas'}`,
          text: 'Las reservas confirmadas o con seña se mantienen y siguen apareciendo en Reservas. ¿Querés archivarlo igualmente?',
          showCancelButton: true,
          confirmButtonText: 'Archivar igualmente',
          cancelButtonText: 'Cancelar',
        })
        if (!again.isConfirmed) return
        try {
          await archiveWatch(w.id, true)
          reload()
        } catch (e2) {
          swal.fire({ icon: 'error', title: 'No se pudo archivar', text: e2.message })
        }
        return
      }
      swal.fire({ icon: 'error', title: 'No se pudo actualizar', text: e.message })
    }
  }

  const removeForever = async (w) => {
    const result = await swal.fire({
      icon: 'warning',
      title: `¿Eliminar definitivamente "${w.name}"?`,
      text: 'Solo es posible si no tiene reservas. Esta acción no se puede deshacer.',
      showCancelButton: true,
      confirmButtonText: 'Eliminar definitivamente',
      cancelButtonText: 'Cancelar',
    })
    if (!result.isConfirmed) return
    try {
      await deleteWatch(w.id)
      reload()
    } catch (e) {
      swal.fire({ icon: 'error', title: 'No se pudo eliminar', text: e.message })
    }
  }

  return (
    <div>
      <div className="d-flex justify-content-between align-items-center flex-wrap gap-3 mb-5">
        <div>
          <span className="eyebrow">Panel Admin</span>
          <h2 className="mb-0">Relojes</h2>
        </div>
        <Link to="/admin/relojes/nuevo" className="btn btn-primary">Nuevo reloj</Link>
      </div>

      {stats && (
        <dl className="admin-stats" aria-label="Resumen del catálogo">
          <div><dt>{status === 'archived' ? 'Archivados' : status === 'all' ? 'Total' : 'Activos'}</dt><dd>{stats.total}</dd></div>
          <div><dt>Sin stock</dt><dd>{stats.outOfStock}</dd></div>
          <div><dt>Última unidad</dt><dd>{stats.lastUnit}</dd></div>
        </dl>
      )}

      <div className="admin-toolbar" role="search">
        <div className="field">
          <label htmlFor="w-search" className="field-label">Buscar</label>
          <input
            id="w-search" className="field-input" type="search" placeholder="Nombre, marca o modelo (ej: ga2100)"
            value={search} onChange={changeFilter(setSearch)} maxLength={80}
          />
        </div>
        <div className="field">
          <label htmlFor="w-stock" className="field-label">Stock</label>
          <select id="w-stock" className="field-select" value={availability} onChange={changeFilter(setAvailability)}>
            {AVAILABILITY_FILTERS.map(o => <option key={o.value} value={o.value}>{o.label}</option>)}
          </select>
        </div>
        <div className="field">
          <label htmlFor="w-status" className="field-label">Estado</label>
          <select id="w-status" className="field-select" value={status} onChange={changeFilter(setStatus)}>
            <option value="active">Activos</option>
            <option value="archived">Archivados</option>
            <option value="all">Todos</option>
          </select>
        </div>
        <div className="field">
          <label htmlFor="w-sort" className="field-label">Orden</label>
          <select id="w-sort" className="field-select" value={sort} onChange={changeFilter(setSort)}>
            {SORTS.map(o => <option key={o.value} value={o.value}>{o.label}</option>)}
          </select>
        </div>
      </div>

      {state === 'loading' && !data && <SkeletonLines count={4} />}

      {state === 'error' && (
        <EmptyState
          icon="!" title="No pudimos cargar el catálogo"
          action={<Button variant="secondary" size="sm" onClick={reload}>Reintentar</Button>}
        />
      )}

      {data && items.length === 0 && state !== 'error' && (
        <EmptyState
          icon="—" title="No hay relojes con estos filtros"
          action={<Link to="/admin/relojes/nuevo" className="btn btn-primary btn-sm">Nuevo reloj</Link>}
        />
      )}

      {data && items.length > 0 && state !== 'error' && (
        <>
          <p className="admin-count" aria-live="polite">
            {data.total} {data.total === 1 ? 'reloj' : 'relojes'}
            {state === 'loading' && <span className="spinner ms-2" aria-label="Actualizando" />}
          </p>
          <div className="admin-table-wrap">
            <table className="admin-table">
              <thead>
                <tr><th>Imagen</th><th>Reloj</th><th>Género</th><th>Precio</th><th>Stock</th><th /></tr>
              </thead>
              <tbody>
                {items.map(w => (
                  <tr key={w.id} className={w.archivedAt ? 'is-archived' : ''}>
                    <td>
                      {w.image
                        ? <img src={imageUrl(w.image)} alt={w.name} className="admin-thumb" loading="lazy" />
                        : <div className="admin-thumb admin-thumb-empty" aria-hidden="true">Sin foto</div>}
                    </td>
                    <td>
                      {w.name}
                      <span className="d-block text-faint-token">{w.model || 'Sin modelo'}{w.archivedAt ? ' · Archivado' : ''}</span>
                    </td>
                    <td className="text-capitalize">{w.gender}</td>
                    <td>{formatCurrency(w.priceARS) || '—'}</td>
                    <td>
                      <div className="admin-stock-cell">
                        <button
                          type="button" className="admin-stepper" onClick={() => changeStock(w, -1)}
                          disabled={w.stockQuantity <= 0} aria-label={`Restar una unidad a ${w.name}`}
                        >−</button>
                        <span className="admin-stock-qty" aria-label={`${w.stockQuantity} unidades`}>{w.stockQuantity}</span>
                        <button
                          type="button" className="admin-stepper" onClick={() => changeStock(w, 1)}
                          aria-label={`Sumar una unidad a ${w.name}`}
                        >+</button>
                        <StockBadge quantity={w.stockQuantity} />
                      </div>
                    </td>
                    <td className="text-end admin-actions">
                      <Link to={`/admin/relojes/${w.id}/editar`} className="link-underline">Editar</Link>
                      <Link to={`/admin/stock?watchId=${w.id}`} className="link-underline">Historial</Link>
                      <button className="btn-danger" onClick={() => toggleArchive(w)}>
                        {w.archivedAt ? 'Restaurar' : 'Archivar'}
                      </button>
                      {w.archivedAt && (
                        <button className="btn-danger" onClick={() => removeForever(w)}>Eliminar</button>
                      )}
                    </td>
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
