import { useCallback, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import { adminDeleteInquiry, adminListInquiries, adminUpdateInquiry } from '../../services/adminInquiriesService'
import EmptyState from '../../components/ui/EmptyState'
import Button from '../../components/ui/Button'
import { SkeletonLines } from '../../components/ui/Skeleton'
import { useAsync } from '../../hooks/useAsync'
import swal from '../../utils/swal'

const INQUIRY_STATUSES = [
  { value: 'new', label: 'Nueva' },
  { value: 'contacted', label: 'Contactado' },
  { value: 'closed', label: 'Cerrada' },
]
const labelOf = s => INQUIRY_STATUSES.find(o => o.value === s)?.label || s

function InquiryCard({ q, onChange, onRemove }) {
  const [notes, setNotes] = useState(q.adminNotes || '')
  const [saving, setSaving] = useState(false)
  const dirty = notes.trim() !== (q.adminNotes || '')

  const update = async (data) => {
    setSaving(true)
    try {
      onChange(await adminUpdateInquiry(q.id, data))
    } catch (e) {
      swal.fire({ icon: 'error', title: 'No se pudo actualizar', text: e.message })
    } finally {
      setSaving(false)
    }
  }

  return (
    <article className={`admin-inquiry ${q.status === 'new' ? 'is-unread' : ''}`}>
      <div className="d-flex justify-content-between flex-wrap gap-2">
        <div>
          <span className={`admin-badge me-2 ${q.status === 'new' ? 'is-new' : ''}`}>{labelOf(q.status)}</span>
          <strong>{q.name}</strong>{' '}
          <span className="meta">{q.email}{q.phone ? ` · ${q.phone}` : ''}</span>
        </div>
        <span className="meta">
          {new Date(q.createdAt).toLocaleString('es-AR')}
          {q.contactedAt && <> · Contactado {new Date(q.contactedAt).toLocaleDateString('es-AR')}</>}
          {q.closedAt && <> · Cerrada {new Date(q.closedAt).toLocaleDateString('es-AR')}</>}
        </span>
      </div>
      {q.watchNameSnapshot && (
        <p className="meta mt-2 mb-0">
          Reloj consultado: <span className="text-strong-token">{q.watchNameSnapshot}</span>
          {q.watchModelSnapshot && <> · {q.watchModelSnapshot}</>}
        </p>
      )}
      <p className="mb-3 mt-3">{q.message}</p>

      <div className="admin-inquiry-controls">
        <div className="field">
          <label htmlFor={`st-${q.id}`} className="field-label">Estado</label>
          <select
            id={`st-${q.id}`} className="field-select" value={q.status} disabled={saving}
            onChange={e => update({ status: e.target.value })}
          >
            {INQUIRY_STATUSES.map(o => <option key={o.value} value={o.value}>{o.label}</option>)}
          </select>
        </div>
        <div className="field flex-grow-1">
          <label htmlFor={`nt-${q.id}`} className="field-label">Notas internas</label>
          <textarea
            id={`nt-${q.id}`} className="field-textarea" rows={2} maxLength={2000}
            value={notes} onChange={e => setNotes(e.target.value)}
            placeholder="Solo visibles en el panel"
          />
        </div>
      </div>
      <div className="d-flex flex-wrap gap-4 mt-3 align-items-center">
        {dirty && (
          <Button size="sm" variant="secondary" loading={saving} onClick={() => update({ adminNotes: notes })}>
            Guardar notas
          </Button>
        )}
        {q.watchId && (
          <Link to="/admin/reservas/nueva" state={{ inquiry: q }} className="link-underline">
            Crear reserva
          </Link>
        )}
        <button className="btn-danger" onClick={() => onRemove(q)}>Eliminar</button>
      </div>
    </article>
  )
}

export default function Inquiries() {
  const [params] = useSearchParams()
  const [status, setStatus] = useState(params.get('status') || '')
  const fetcher = useCallback(() => adminListInquiries({ status }), [status])
  const { state, data: items, reload, setData } = useAsync(fetcher, [status])

  const replace = updated => setData(prev => prev.map(x => (x.id === updated.id ? updated : x)))

  const remove = async (q) => {
    const result = await swal.fire({
      icon: 'warning',
      title: `¿Eliminar la consulta de ${q.name}?`,
      text: 'Usalo para spam o errores. Para consultas atendidas, mejor marcarlas como cerradas.',
      showCancelButton: true,
      confirmButtonText: 'Eliminar',
      cancelButtonText: 'Cancelar',
    })
    if (!result.isConfirmed) return
    try {
      await adminDeleteInquiry(q.id)
      setData(prev => prev.filter(x => x.id !== q.id))
    } catch (e) {
      swal.fire({ icon: 'error', title: 'No se pudo eliminar', text: e.message })
    }
  }

  return (
    <div>
      <span className="eyebrow">Panel Admin</span>
      <h2 className="mb-4">Consultas</h2>
      <p className="text-muted-token mb-5">
        Una consulta solo expresa interés: no descuenta stock. Para apartar un
        reloj, creá una reserva.
      </p>

      <div className="admin-toolbar">
        <div className="field">
          <label htmlFor="q-status" className="field-label">Estado</label>
          <select id="q-status" className="field-select" value={status} onChange={e => setStatus(e.target.value)}>
            <option value="">Todas</option>
            {INQUIRY_STATUSES.map(o => <option key={o.value} value={o.value}>{o.label}</option>)}
          </select>
        </div>
      </div>

      {state === 'loading' && !items && <SkeletonLines count={4} />}

      {state === 'error' && (
        <EmptyState
          icon="!" title="No pudimos cargar las consultas"
          action={<Button variant="secondary" size="sm" onClick={reload}>Reintentar</Button>}
        />
      )}

      {state === 'success' && items.length === 0 && (
        <EmptyState icon="—" title="No hay consultas con este filtro" />
      )}

      {state === 'success' && items.length > 0 && (
        <div className="d-grid gap-3">
          {items.map(q => <InquiryCard key={q.id} q={q} onChange={replace} onRemove={remove} />)}
        </div>
      )}
    </div>
  )
}
