import { Link } from 'react-router-dom'
import RevealOnScroll from '../../components/RevealOnScroll/RevealOnScroll'
import ReservationSteps from '../../components/ReservationSteps/ReservationSteps'
import { WhatsAppIcon } from '../../components/ui/Icons'
import { business, contact, mailtoUrl, whatsappUrl } from '../../utils/siteConfig'

/**
 * Página informativa a la que lleva el icono de usuario del Navbar. Por ahora
 * no hay cuentas de cliente: las reservas se gestionan directamente con el
 * local, así que acá se explica el proceso y se ofrecen los canales de contacto.
 */
export default function ComoReservar() {
  const wa = whatsappUrl('Hola, quisiera consultar por una reserva.')
  const mail = mailtoUrl('Consulta por una reserva')

  return (
    <section className="section">
      <div className="container">
        <RevealOnScroll className="section-header">
          <span className="eyebrow">Reservas</span>
          <h1 className="h2">Cómo reservar</h1>
          <p>
            Elegí un reloj, tocá “Reservar” y enviá tu solicitud desde tu cuenta.
            La relojería la revisa y te contacta para confirmarla. Podés seguir el
            estado en <Link to="/mi-cuenta/reservas" className="link-underline">Mis reservas</Link>.
          </p>
        </RevealOnScroll>

        <div className="row g-5">
          <div className="col-lg-7">
            <RevealOnScroll>
              <h2 className="detail-subtitle">Cómo funciona</h2>
              <ReservationSteps />
              <p className="text-faint-token mt-4 mb-0">
                No realizamos ventas ni cobros online y nunca solicitamos datos de
                tarjeta por este sitio.
              </p>
            </RevealOnScroll>
          </div>
          <div className="col-lg-5">
            <RevealOnScroll delay={0.08}>
              <div className="info-panel">
                <h2 className="detail-subtitle">Contacto</h2>
                <p className="text-strong-token mb-1">{business.name}</p>
                <p className="mb-4">
                  {business.address.street}, {business.address.area}<br />
                  {business.address.city}
                </p>
                <p className="mb-4">
                  Teléfono: <a className="link-underline" href={`tel:${business.phone.tel}`}>{business.phone.display}</a>
                  {contact.email && (
                    <>
                      <br />Email: <a className="link-underline" href={mail}>{contact.email}</a>
                    </>
                  )}
                </p>
                <div className="detail-actions">
                  {wa && (
                    <a href={wa} target="_blank" rel="noopener noreferrer" className="btn btn-primary">
                      <WhatsAppIcon /> Escribinos por WhatsApp
                    </a>
                  )}
                  <Link to="/contacto" className={`btn ${wa ? 'btn-secondary' : 'btn-primary'}`}>
                    Enviar una consulta
                  </Link>
                  <Link to="/relojes" className="btn btn-secondary">Ver relojes</Link>
                </div>
              </div>
            </RevealOnScroll>
          </div>
        </div>
      </div>
    </section>
  )
}
