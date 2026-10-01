import { useCallback } from 'react'
import { Link, useParams } from 'react-router-dom'
import { getWatch } from '../../services/watchesService'
import WatchClock from '../../components/WatchClock/WatchClock'
import RevealOnScroll from '../../components/RevealOnScroll/RevealOnScroll'
import ReservationSteps from '../../components/ReservationSteps/ReservationSteps'
import EmptyState from '../../components/ui/EmptyState'
import StockBadge from '../../components/ui/StockBadge'
import { WhatsAppIcon } from '../../components/ui/Icons'
import { SkeletonLines } from '../../components/ui/Skeleton'
import { imageUrl } from '../../utils/imageUrl'
import { formatCurrency } from '../../utils/formatCurrency'
import { isAvailable } from '../../utils/stock'
import { business, mailtoUrl, watchInquiryMessage, whatsappUrl } from '../../utils/siteConfig'
import { useAsync } from '../../hooks/useAsync'

function ContactActions({ watch }) {
  const available = isAvailable(watch.stockQuantity)
  const message = available
    ? watchInquiryMessage(watch)
    : `Hola, quisiera saber si van a volver a tener el ${watch.name}.`
  const wa = whatsappUrl(message)
  const mail = mailtoUrl(`Consulta por ${watch.name}`, message)
  // La consulta queda vinculada al reloj por id (no solo por el nombre).
  const contactLink = `/contacto?watch=${watch.id}`

  return (
    <div className="detail-actions">
      {/* CTA principal: solicitar reserva. Si no hay sesión, /reservar/:id
          lleva a Ingresar y vuelve a este reloj después del login. */}
      {available && <Link to={`/reservar/${watch.id}`} className="btn btn-primary">Reservar</Link>}
      {wa && (
        <a href={wa} target="_blank" rel="noopener noreferrer" className="btn btn-secondary">
          <WhatsAppIcon />
          {available ? 'Consultar por WhatsApp' : 'Consultar reposición por WhatsApp'}
        </a>
      )}
      <Link to={contactLink} className={`btn ${available || wa ? 'btn-secondary' : 'btn-primary'}`}>
        {available ? 'Consultar disponibilidad' : 'Consultar reposición'}
      </Link>
      {mail && <a href={mail} className="btn btn-secondary">Consultar por email</a>}
    </div>
  )
}

export default function RelojDetalle() {
  const { id } = useParams()
  const fetcher = useCallback(() => getWatch(id), [id])
  const { state, data: w } = useAsync(fetcher, [id])

  if (state === 'error') {
    return (
      <section className="section">
        <div className="container">
          <EmptyState
            icon="!"
            title="No encontramos este reloj"
            description="Puede que ya no esté disponible."
            action={<Link to="/relojes" className="btn btn-secondary btn-sm">Ver catálogo</Link>}
          />
        </div>
      </section>
    )
  }

  if (state === 'loading' || !w) {
    return (
      <section className="section">
        <div className="container">
          <div className="row g-5 align-items-center">
            <div className="col-md-6">
              <div className="skeleton skeleton-card detail-skeleton" />
            </div>
            <div className="col-md-6"><SkeletonLines count={5} /></div>
          </div>
        </div>
      </section>
    )
  }

  const available = isAvailable(w.stockQuantity)
  const price = formatCurrency(w.priceARS)

  return (
    <section className="section">
      <div className="container">
        <Link to="/relojes" className="link-underline">Volver al catálogo</Link>
        <div className="row g-5 mt-2 align-items-start">
          <div className="col-md-6">
            <RevealOnScroll y={0}>
              <div className={`detail-ph ${w.image ? 'has-image' : ''}`}>
                {w.image
                  ? <img src={imageUrl(w.image)} alt={`Reloj ${w.name}`} width="800" height="800" />
                  : <WatchClock hour={10} minute={8 + (w.id?.length || 1) * 3} label={w.name} />}
              </div>
            </RevealOnScroll>
          </div>
          <div className="col-md-6">
            <RevealOnScroll delay={0.08}>
              {w.brand && <span className="eyebrow mb-2">{w.brand}</span>}
              <h1 className="h2 mb-2">{w.name}</h1>
              {w.model && <p className="detail-model">Modelo {w.model}</p>}

              <div className="detail-price-row">
                <p className="h4 mb-0 text-strong-token">{price || 'Consultar precio'}</p>
                <StockBadge quantity={w.stockQuantity} />
              </div>
              {!available && (
                <p className="detail-notice" role="status">
                  Este modelo no está disponible por el momento. Podés consultarnos
                  si vamos a volver a tenerlo.
                </p>
              )}

              {w.description && <p className="mb-5">{w.description}</p>}

              {w.specs?.length > 0 && (
                <>
                  <h2 className="detail-subtitle">Especificaciones</h2>
                  <ul className="list-unstyled mb-5">
                    {w.specs.map(s => (
                      <li className="service-row" key={s}>
                        <span className="index" aria-hidden="true">·</span>
                        <p className="mb-0">{s}</p>
                      </li>
                    ))}
                  </ul>
                </>
              )}

              <ContactActions watch={w} />
              <p className="detail-phone">
                También podés llamarnos al <a href={`tel:${business.phone.tel}`}>{business.phone.display}</a>.
              </p>
            </RevealOnScroll>
          </div>
        </div>

        <div className="detail-reservation">
          <div className="row g-5">
            <div className="col-lg-4">
              <span className="eyebrow">Cómo reservar</span>
              <h2 className="h3">Reserva coordinada con el local</h2>
              <p>
                No vendemos online. Al tocar “Reservar” enviás una solicitud que
                la relojería revisa y confirma; puede requerir una seña acordada
                con el local. El pago final se realiza en nuestro local. Nunca te
                vamos a pedir datos de tarjeta por este sitio.
              </p>
            </div>
            <div className="col-lg-8"><ReservationSteps compact /></div>
          </div>
        </div>
      </div>
    </section>
  )
}
