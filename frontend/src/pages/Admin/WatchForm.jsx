import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { adminGetWatch, createWatch, updateWatch, uploadWatchImage } from '../../services/adminWatchesService'
import { imageUrl } from '../../utils/imageUrl'
import FormField from '../../components/ui/FormField'
import Button from '../../components/ui/Button'
import StockBadge from '../../components/ui/StockBadge'
import swal from '../../utils/swal'
import { emptyWatchForm, toWatchPayload, validateWatchForm, watchToForm } from '../../utils/watchForm'
import { MANUAL_MOVEMENT_TYPES } from '../../utils/stockMovements'

export default function WatchForm() {
  const { id } = useParams()
  const editing = Boolean(id)
  const navigate = useNavigate()

  const [form, setForm] = useState(emptyWatchForm)
  // Stock al abrir el formulario: se envía para no pisar cambios hechos en
  // paralelo (reservas confirmadas, ajustes desde el listado).
  const [baseStock, setBaseStock] = useState(null)
  const [archived, setArchived] = useState(false)
  const [errors, setErrors] = useState({})
  const [loadState, setLoadState] = useState(editing ? 'loading' : 'ready')
  const [uploading, setUploading] = useState(false)
  const [saving, setSaving] = useState(false)
  const set = k => e => setForm(f => ({ ...f, [k]: e.target.value }))

  useEffect(() => {
    if (!editing) return
    adminGetWatch(id).then(w => {
      setForm(watchToForm(w))
      setBaseStock(w.stockQuantity ?? 0)
      setArchived(Boolean(w.archivedAt))
      setLoadState('ready')
    }).catch(() => {
      setLoadState('error')
      swal.fire({ icon: 'error', title: 'No se pudo cargar el reloj' })
    })
  }, [id, editing])

  const onFile = async (e) => {
    const file = e.target.files[0]
    if (!file) return
    setUploading(true)
    try {
      const { url } = await uploadWatchImage(file)
      setForm(f => ({ ...f, image: url }))
    } catch (err) {
      swal.fire({ icon: 'error', title: 'No se pudo subir la imagen', text: err.message })
    } finally {
      setUploading(false)
    }
  }

  const submit = async (e) => {
    e.preventDefault()
    const nextErrors = validateWatchForm(form)
    setErrors(nextErrors)
    if (Object.keys(nextErrors).length > 0) return

    setSaving(true)
    try {
      if (editing) await updateWatch(id, toWatchPayload(form, baseStock))
      else await createWatch(toWatchPayload(form))
      await swal.fire({ icon: 'success', title: editing ? 'Reloj actualizado' : 'Reloj creado', timer: 1400, showConfirmButton: false })
      navigate('/admin/relojes')
    } catch (err) {
      swal.fire({ icon: 'error', title: 'No se pudo guardar', text: err.message })
    } finally {
      setSaving(false)
    }
  }

  if (loadState === 'loading') {
    return <div className="admin-form"><p className="text-muted-token">Cargando reloj…</p></div>
  }
  if (loadState === 'error') {
    return <div className="admin-form"><p className="text-muted-token">No se pudo cargar este reloj.</p></div>
  }

  return (
    <div className="admin-form">
      <span className="eyebrow">Panel Admin</span>
      <h2 className="mb-2">{editing ? 'Editar reloj' : 'Nuevo reloj'}</h2>
      {archived && <p className="admin-badge mb-4">Archivado: no se muestra en el sitio</p>}
      <form onSubmit={submit} noValidate className="d-grid gap-4 mt-4">
        <FormField label="Marca" required value={form.brand} onChange={set('brand')} error={errors.brand} />
        <FormField
          label="Nombre" required hint="Tal como se muestra en el catálogo, ej: Casio G-Shock GA-2100-1A1"
          value={form.name} onChange={set('name')} error={errors.name}
        />
        <FormField
          label="Modelo" required hint="Código comercial único, ej: GA-2100-1A1"
          value={form.model} onChange={set('model')} error={errors.model}
        />

        <div className="field">
          <label htmlFor="gender" className="field-label">Género</label>
          <select id="gender" className="field-select" value={form.gender} onChange={set('gender')}>
            <option value="hombre">Hombre</option>
            <option value="mujer">Mujer</option>
          </select>
        </div>

        <FormField
          label="Unidades en stock" required type="number" min="0" step="1" inputMode="numeric"
          hint="Las reservas confirmadas descuentan unidades automáticamente."
          value={form.stockQuantity} onChange={set('stockQuantity')} error={errors.stockQuantity}
        />
        <div className="d-flex align-items-center gap-3 admin-stock-preview">
          <span className="field-hint">Se muestra como:</span>
          <StockBadge quantity={Number(form.stockQuantity) || 0} />
        </div>
        {editing && baseStock !== null && String(baseStock) !== form.stockQuantity.trim() && (
          <div className="form-grid">
            <div className="field">
              <label htmlFor="moveType" className="field-label">Tipo de movimiento</label>
              <select id="moveType" className="field-select" value={form.stockMoveType} onChange={set('stockMoveType')}>
                {MANUAL_MOVEMENT_TYPES.map(t => <option key={t.value} value={t.value}>{t.label}</option>)}
              </select>
            </div>
            <FormField label="Motivo (queda en el historial)" maxLength={300} value={form.stockMoveReason} onChange={set('stockMoveReason')} />
          </div>
        )}

        <FormField
          label="Precio (ARS)" inputMode="numeric"
          hint="Entero en pesos, sin centavos. Si queda vacío se muestra “Consultar precio”."
          value={form.price} onChange={set('price')} error={errors.price}
        />
        <FormField label="Descripción" as="textarea" rows={3} value={form.description} onChange={set('description')} />
        <FormField
          label="Especificaciones" as="textarea" rows={4} hint="Una por línea"
          value={form.specs} onChange={set('specs')}
        />

        <div className="field">
          <label htmlFor="image" className="field-label">Imagen</label>
          <input
            id="image" className="field-input" type="file"
            accept="image/png,image/jpeg,image/webp" onChange={onFile}
          />
          {uploading && <span className="field-hint"><span className="spinner" aria-hidden="true" /> Subiendo imagen…</span>}
          {form.image && <img src={imageUrl(form.image)} alt="Vista previa del reloj" className="admin-preview mt-3" />}
        </div>

        <div className="d-flex gap-3">
          <Button type="submit" variant="primary" loading={saving} disabled={uploading}>
            {saving ? 'Guardando' : 'Guardar'}
          </Button>
          <Button type="button" variant="secondary" onClick={() => navigate('/admin/relojes')}>
            Cancelar
          </Button>
        </div>
      </form>
    </div>
  )
}
