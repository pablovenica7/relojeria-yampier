import { useEffect, useMemo, useState } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { adminListWatches } from '../../services/adminWatchesService'
import { adminCreateReservation } from '../../services/adminReservationsService'
import FormField from '../../components/ui/FormField'
import Button from '../../components/ui/Button'
import StockBadge from '../../components/ui/StockBadge'
import { formatCurrency, parseARS } from '../../utils/formatCurrency'
import { RESERVATION_STATUS as S, dateInputValue, expiryFromDate } from '../../utils/reservationStatus'
import swal from '../../utils/swal'

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/
const DEFAULT_HOLD_DAYS = 3

function defaultExpiry() {
  const d = new Date()
  d.setDate(d.getDate() + DEFAULT_HOLD_DAYS)
  return dateInputValue(d.toISOString())
}

export default function ReservationForm() {
  const navigate = useNavigate()
  // Si se llega desde "Crear reserva" en Consultas, se precargan los datos.
  const inquiry = useLocation().state?.inquiry

  const [watches, setWatches] = useState([])
  const [form, setForm] = useState({
    watchId: inquiry?.watchId || '',
    customerName: inquiry?.name || '',
    phone: inquiry?.phone || '',
    email: inquiry?.email || '',
    status: S.PENDING,
    price: '',
    deposit: '',
    expiry: defaultExpiry(),
    notes: '',
  })
  const [errors, setErrors] = useState({})
  const [saving, setSaving] = useState(false)
  const set = k => e => setForm(f => ({ ...f, [k]: e.target.value }))

  useEffect(() => {
    adminListWatches({ status: 'active', sort: 'name_asc', limit: 100 })
      .then(d => setWatches(d.items))
      .catch(e => swal.fire({ icon: 'error', title: 'No se pudo cargar el catálogo', text: e.message }))
  }, [])

  const watch = useMemo(() => watches.find(w => w.id === form.watchId), [watches, form.watchId])

  const validate = () => {
    const e = {}
    if (!form.watchId) e.watchId = 'Elegí un reloj'
    if (!form.customerName.trim()) e.customerName = 'Ingresá el nombre del cliente'
    if (!form.phone.trim() && !form.email.trim()) e.phone = 'Indicá al menos un teléfono o un email'
    if (form.email.trim() && !EMAIL_RE.test(form.email.trim())) e.email = 'El email no es válido'
    const price = parseARS(form.price)
    const deposit = parseARS(form.deposit)
    if (Number.isNaN(price)) e.price = 'Monto entero en pesos, sin centavos'
    if (Number.isNaN(deposit)) e.deposit = 'Monto entero en pesos, sin centavos'
    const finalPrice = price || watch?.priceARS || 0
    if (!e.deposit && finalPrice > 0 && deposit > finalPrice) e.deposit = 'La seña no puede superar el precio'
    if (form.status === S.CONFIRMED && watch && watch.stockQuantity < 1) e.status = 'No hay stock para confirmar: registrala como pendiente'
    if (form.expiry && new Date(expiryFromDate(form.expiry)) <= new Date()) e.expiry = 'Elegí una fecha futura'
    return e
  }

  const submit = async (ev) => {
    ev.preventDefault()
    const e = validate()
    setErrors(e)
    if (Object.keys(e).length > 0) return
    setSaving(true)
    try {
      await adminCreateReservation({
        watchId: form.watchId,
        inquiryId: inquiry?.id || '',
        customerName: form.customerName.trim(),
        phone: form.phone.trim(),
        email: form.email.trim(),
        status: form.status,
        priceAtReservation: parseARS(form.price),
        depositAmount: parseARS(form.deposit),
        expiresAt: expiryFromDate(form.expiry),
        notes: form.notes.trim(),
      })
      await swal.fire({ icon: 'success', title: 'Reserva creada', timer: 1400, showConfirmButton: false })
      navigate('/admin/reservas')
    } catch (err) {
      swal.fire({ icon: 'error', title: 'No se pudo crear la reserva', text: err.message })
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="admin-form">
      <span className="eyebrow">Panel Admin</span>
      <h2 className="mb-5">Nueva reserva</h2>
      <form onSubmit={submit} noValidate className="d-grid gap-4">
        <div className="field">
          <label htmlFor="res-watch" className="field-label">Reloj *</label>
          <select
            id="res-watch" className="field-select" value={form.watchId} onChange={set('watchId')}
            aria-invalid={errors.watchId ? 'true' : 'false'} aria-describedby="res-watch-error"
          >
            <option value="">Elegí un reloj…</option>
            {watches.map(w => (
              <option key={w.id} value={w.id}>
                {w.name} · {w.stockQuantity} {w.stockQuantity === 1 ? 'unidad' : 'unidades'}
              </option>
            ))}
          </select>
          <span id="res-watch-error" className="field-error" role={errors.watchId ? 'alert' : undefined}>{errors.watchId || ''}</span>
          {watch && (
            <div className="d-flex align-items-center gap-3">
              <StockBadge quantity={watch.stockQuantity} />
              <span className="field-hint">Precio de lista: {formatCurrency(watch.priceARS) || 'sin cargar'}</span>
            </div>
          )}
        </div>

        <FormField label="Cliente" required value={form.customerName} onChange={set('customerName')} error={errors.customerName} />
        <FormField label="Teléfono" value={form.phone} onChange={set('phone')} error={errors.phone} autoComplete="off" />
        <FormField label="Email" type="email" value={form.email} onChange={set('email')} error={errors.email} autoComplete="off" />

        <div className="field">
          <label htmlFor="res-status" className="field-label">Estado inicial</label>
          <select
            id="res-status" className="field-select" value={form.status} onChange={set('status')}
            aria-describedby="res-status-hint res-status-error"
          >
            <option value={S.PENDING}>Pendiente (no retiene stock)</option>
            <option value={S.CONFIRMED}>Confirmada (retiene 1 unidad)</option>
          </select>
          <span id="res-status-hint" className="field-hint">La seña se registra después, desde el listado de reservas.</span>
          <span id="res-status-error" className="field-error" role={errors.status ? 'alert' : undefined}>{errors.status || ''}</span>
        </div>

        <FormField
          label="Precio acordado (ARS)" inputMode="numeric"
          hint="Opcional. Vacío = precio de lista actual. Queda guardado aunque después cambie el precio del reloj."
          value={form.price} onChange={set('price')} error={errors.price}
        />
        <FormField
          label="Seña acordada (ARS)" inputMode="numeric" hint="Opcional. Monto a cobrar como seña."
          value={form.deposit} onChange={set('deposit')} error={errors.deposit}
        />
        <FormField
          label="Vence" type="date" hint={`Por defecto ${DEFAULT_HOLD_DAYS} días. Vacío = sin vencimiento.`}
          value={form.expiry} onChange={set('expiry')} error={errors.expiry}
        />
        <FormField label="Notas" as="textarea" rows={3} maxLength={2000} value={form.notes} onChange={set('notes')} />

        <div className="d-flex gap-3">
          <Button type="submit" variant="primary" loading={saving}>{saving ? 'Guardando' : 'Crear reserva'}</Button>
          <Button type="button" variant="secondary" onClick={() => navigate('/admin/reservas')}>Cancelar</Button>
        </div>
      </form>
    </div>
  )
}
