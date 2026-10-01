import { useCallback, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { getWatch } from '../../services/watchesService'
import { requestReservation } from '../../services/customerService'
import { useCustomerAuth } from '../../context/CustomerAuthContext'
import { useAsync } from '../../hooks/useAsync'
import { CheckField } from '../../components/Customer/CustomerFields'
import EmptyState from '../../components/ui/EmptyState'
import FormField from '../../components/ui/FormField'
import Button from '../../components/ui/Button'
import StockBadge from '../../components/ui/StockBadge'
import { SkeletonLines } from '../../components/ui/Skeleton'
import { formatCurrency } from '../../utils/formatCurrency'
import { imageUrl } from '../../utils/imageUrl'
import { isAvailable } from '../../utils/stock'

/**
 * Solicitud de reserva de un cliente autenticado (protegida por CustomerRoute).
 * La solicitud queda PENDIENTE: no bloquea la unidad hasta que la relojería
 * la confirme. No hay pago online.
 */
export default function Reservar() {
  const { id } = useParams()
  const { customer } = useCustomerAuth()
  const fetcher = useCallback(() => getWatch(id), [id])
  const { state, data: w } = useAsync(fetcher, [id])

  const [notes, setNotes] = useState('')
  const [terms, setTerms] = useState(false) // nunca premarcada
  const [termsError, setTermsError] = useState('')
  const [serverError, setServerError] = useState('')
  const [sending, setSending] = useState(false)
  const [done, setDone] = useState(null)

  const submit = async (e) => {
    e.preventDefault()
    setServerError('')
    if (!terms) {
      setTermsError('Para continuar, leé y aceptá los términos de reserva')
      return
    }
    setTermsError('')
    setSending(true)
    try {
      setDone(await requestReservation(w.id, true, notes.trim()))
      window.scrollTo({ top: 0 })
    } catch (err) {
      setServerError(err.message)
    } finally {
      setSending(false)
    }
  }

  if (state === 'loading') {
    return <section className="section"><div className="container account-container"><SkeletonLines count={5} /></div></section>
  }
  if (state === 'error' || !w) {
    return (
      <section className="section">
        <div className="container">
          <EmptyState icon="!" title="No encontramos este reloj"
            action={<Link to="/relojes" className="btn btn-secondary btn-sm">Ver catálogo</Link>} />
        </div>
      </section>
    )
  }

  if (done) {
    return (
      <section className="section">
        <div className="container account-container">
          <div className="success-panel" role="status">
            <span className="eyebrow">Reserva</span>
            <h1 className="h2 mb-3">Solicitud enviada</h1>
            <p>Tu solicitud de reserva del <strong className="text-strong-token">{done.watchName}</strong> fue recibida.</p>
            <p>Relojería Yampier revisará la disponibilidad y te contactará para confirmarla.</p>
            <p className="text-strong-token">Una solicitud pendiente todavía no bloquea la unidad.</p>
            <p>El pago final se realiza en nuestro local.</p>
            <div className="detail-actions mt-5">
              <Link to="/mi-cuenta/reservas" className="btn btn-primary">Ver mis reservas</Link>
              <Link to="/relojes" className="btn btn-secondary">Seguir viendo relojes</Link>
            </div>
          </div>
        </div>
      </section>
    )
  }

  const available = isAvailable(w.stockQuantity)
  return (
    <section className="section">
      <div className="container account-container">
        <Link to={`/reloj/${w.id}`} className="link-underline">Volver al reloj</Link>
        <h1 className="h2 mt-4 mb-5">Solicitar reserva</h1>

        <div className="reserve-summary">
          <div className="my-reservation-img">
            {w.image && <img src={imageUrl(w.image)} alt={`Reloj ${w.name}`} width="120" height="120" />}
          </div>
          <div>
            <p className="eyebrow mb-1">{w.brand}</p>
            <h2 className="h4 mb-1">{w.name}</h2>
            <p className="text-faint-token mb-2">Modelo {w.model}</p>
            <div className="d-flex align-items-center gap-3 flex-wrap">
              <span className="text-strong-token">{formatCurrency(w.priceARS) || 'Precio a confirmar'}</span>
              <StockBadge quantity={w.stockQuantity} />
            </div>
          </div>
        </div>

        {!available ? (
          <EmptyState icon="—" title="Este reloj no tiene stock en este momento"
            description="Podés consultarnos si vamos a volver a tenerlo."
            action={<Link to={`/contacto?watch=${w.id}`} className="btn btn-secondary btn-sm">Consultar reposición</Link>} />
        ) : (
          <form onSubmit={submit} noValidate className="d-grid gap-5 mt-5">
            <div className="info-panel">
              <h2 className="detail-subtitle">Cómo sigue</h2>
              <ol className="reserve-steps">
                <li>Tu solicitud queda <strong>pendiente de confirmación</strong> (todavía no bloquea la unidad).</li>
                <li>La relojería revisa la disponibilidad y te contacta.</li>
                <li>Si corresponde, se coordina una seña con la relojería.</li>
                <li>Retirás el reloj y completás el pago en el local.</li>
              </ol>
              <p className="mb-0 text-strong-token">El pago final se realiza en nuestro local.</p>
            </div>

            <div>
              <h2 className="detail-subtitle">Te vamos a contactar a</h2>
              <p className="mb-1">{customer.firstName} {customer.lastName}</p>
              <p className="mb-1">{customer.phone} · {customer.email}</p>
              <Link to="/mi-cuenta" className="link-underline">Corregir mis datos</Link>
            </div>

            <FormField
              label="Comentario (opcional)" as="textarea" rows={3} maxLength={500}
              hint="Por ejemplo, cuándo podés pasar por el local."
              value={notes} onChange={e => setNotes(e.target.value)}
            />

            <CheckField id="terms" checked={terms} onChange={e => setTerms(e.target.checked)} error={termsError}>
              He leído y acepto los <Link to="/terminos-de-reserva" target="_blank" rel="noopener">términos de reserva</Link>.
            </CheckField>

            {serverError && <p className="form-alert" role="alert">{serverError}</p>}
            <div>
              <Button type="submit" variant="primary" loading={sending}>{sending ? 'Enviando' : 'Solicitar reserva'}</Button>
            </div>
          </form>
        )}
      </div>
    </section>
  )
}
