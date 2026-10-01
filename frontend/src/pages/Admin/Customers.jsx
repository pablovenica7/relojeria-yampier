import { useCallback, useState } from 'react'
import { authRequest } from '../../services/api'
import { toQuery } from '../../services/watchesService'
import EmptyState from '../../components/ui/EmptyState'
import Button from '../../components/ui/Button'
import Pagination from '../../components/ui/Pagination'
import { SkeletonLines } from '../../components/ui/Skeleton'
import { useAsync } from '../../hooks/useAsync'
import { useDebouncedValue } from '../../hooks/useDebouncedValue'
import { DOCUMENT_TYPES, TAX_CONDITIONS, labelFrom } from '../../utils/customerOptions'
import swal from '../../utils/swal'

/**
 * Cuentas de clientes. No se borran (tienen reservas históricas): se
 * desactivan, lo que impide nuevos logins y cierra sus sesiones.
 */
export default function Customers() {
  const [search, setSearch] = useState('')
  const [status, setStatus] = useState('all')
  const [page, setPage] = useState(1)
  const q = useDebouncedValue(search.trim())

  const fetcher = useCallback(
    () => authRequest(`/admin/customers${toQuery({ search: q, status, page, limit: 25 })}`),
    [q, status, page]
  )
  const { state, data, reload, setData } = useAsync(fetcher, [q, status, page])
  const items = data?.items || []

  const toggle = async (c) => {
    const deactivating = c.active
    const { isConfirmed } = await swal.fire({
      icon: deactivating ? 'warning' : 'question',
      title: deactivating ? `¿Desactivar la cuenta de ${c.firstName} ${c.lastName}?` : `¿Reactivar la cuenta de ${c.firstName} ${c.lastName}?`,
      text: deactivating
        ? 'No podrá iniciar sesión y se cerrarán sus sesiones abiertas. Sus reservas se conservan.'
        : 'Podrá volver a iniciar sesión con su contraseña.',
      showCancelButton: true,
      confirmButtonText: deactivating ? 'Desactivar' : 'Reactivar',
      cancelButtonText: 'Cancelar',
    })
    if (!isConfirmed) return
    try {
      const updated = await authRequest(`/admin/customers/${c.id}/${deactivating ? 'deactivate' : 'reactivate'}`, { method: 'PATCH' })
      setData(prev => ({ ...prev, items: prev.items.map(x => (x.id === updated.id ? updated : x)) }))
    } catch (e) {
      swal.fire({ icon: 'error', title: 'No se pudo actualizar la cuenta', text: e.message })
    }
  }

  return (
    <div>
      <span className="eyebrow">Panel Admin</span>
      <h2 className="mb-5">Clientes</h2>

      <div className="admin-toolbar">
        <div className="field">
          <label htmlFor="c-search" className="field-label">Buscar</label>
          <input id="c-search" type="search" className="field-input" placeholder="Nombre, email o documento" maxLength={80}
            value={search} onChange={e => { setSearch(e.target.value); setPage(1) }} />
        </div>
        <div className="field">
          <label htmlFor="c-status" className="field-label">Estado</label>
          <select id="c-status" className="field-select" value={status} onChange={e => { setStatus(e.target.value); setPage(1) }}>
            <option value="all">Todas</option>
            <option value="active">Activas</option>
            <option value="inactive">Desactivadas</option>
          </select>
        </div>
      </div>

      {state === 'loading' && !data && <SkeletonLines count={4} />}
      {state === 'error' && (
        <EmptyState icon="!" title="No pudimos cargar los clientes"
          action={<Button variant="secondary" size="sm" onClick={reload}>Reintentar</Button>} />
      )}
      {data && items.length === 0 && state !== 'error' && <EmptyState icon="—" title="No hay clientes con estos filtros" />}

      {data && items.length > 0 && (
        <>
          <p className="admin-count">{data.total} {data.total === 1 ? 'cuenta' : 'cuentas'}</p>
          <div className="admin-table-wrap">
            <table className="admin-table">
              <thead><tr><th>Cliente</th><th>Contacto</th><th>Documento</th><th>Condición fiscal</th><th>Estado</th><th /></tr></thead>
              <tbody>
                {items.map(c => (
                  <tr key={c.id} className={c.active ? '' : 'is-archived'}>
                    <td>{c.legalName}<span className="d-block text-faint-token">Alta {new Date(c.createdAt).toLocaleDateString('es-AR')}</span></td>
                    <td>{c.email}<span className="d-block text-faint-token">{c.phone}</span></td>
                    <td className="text-nowrap">{labelFrom(DOCUMENT_TYPES, c.documentType)} {c.documentNumber}</td>
                    <td>{labelFrom(TAX_CONDITIONS, c.taxCondition)}</td>
                    <td>{c.active ? 'Activa' : 'Desactivada'}</td>
                    <td className="text-end">
                      <button className="btn-danger" onClick={() => toggle(c)}>{c.active ? 'Desactivar' : 'Reactivar'}</button>
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
