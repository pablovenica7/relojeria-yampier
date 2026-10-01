import { Link } from 'react-router-dom'
import { FacebookIcon, InstagramIcon, WhatsAppIcon } from '../ui/Icons'
import { business, contact, whatsappUrl } from '../../utils/siteConfig'

export default function Footer() {
  const wa = whatsappUrl('Hola, quisiera hacer una consulta.')
  // Solo se muestran redes/canales confirmados (configurados por variable de entorno).
  const socials = [
    contact.instagramUrl && { href: contact.instagramUrl, label: 'Instagram de Relojería Yampier', Icon: InstagramIcon },
    contact.facebookUrl && { href: contact.facebookUrl, label: 'Facebook de Relojería Yampier', Icon: FacebookIcon },
    wa && { href: wa, label: 'WhatsApp de Relojería Yampier', Icon: WhatsAppIcon },
  ].filter(Boolean)

  return (
    <footer className="site-footer">
      <div className="container">
        <div className="footer-grid">
          <nav className="footer-col" aria-label="Enlaces del sitio">
            <h2 className="footer-title">Navegación</h2>
            <ul className="footer-list">
              <li><Link to="/">Inicio</Link></li>
              <li><Link to="/relojes">Relojes</Link></li>
              <li><Link to="/servicios">Servicios</Link></li>
              <li><Link to="/contacto">Contacto</Link></li>
            </ul>
          </nav>

          <div className="footer-col">
            <h2 className="footer-title">Contacto</h2>
            <address className="footer-list mb-0">
              <span className="text-strong-token">{business.name}</span>
              <span>{business.address.street}</span>
              <span>{business.address.area}</span>
              <span>{business.address.city}</span>
              <span className="mt-2">
                Teléfono: <a href={`tel:${business.phone.tel}`}>{business.phone.display}</a>
              </span>
              {contact.email && <a href={`mailto:${contact.email}`}>{contact.email}</a>}
            </address>
          </div>

          <div className="footer-col">
            <h2 className="footer-title">Atención</h2>
            {business.hours.length > 0 ? (
              <ul className="footer-list">
                {business.hours.map(h => (
                  <li key={h.days}><span className="text-strong-token">{h.days}</span><br />{h.hours}</li>
                ))}
              </ul>
            ) : (
              <p className="footer-text">{business.hoursNotice}</p>
            )}
          </div>

          <div className="footer-col">
            <h2 className="footer-title">Reservas</h2>
            <p className="footer-text">
              Consultá disponibilidad y reservá tu reloj. La operación se coordina
              directamente con nuestro local.
            </p>
            {wa ? (
              <a href={wa} target="_blank" rel="noopener noreferrer" className="btn btn-secondary btn-sm">
                <WhatsAppIcon /> Consultar por WhatsApp
              </a>
            ) : (
              <Link to="/contacto" className="btn btn-secondary btn-sm">Hacer una consulta</Link>
            )}
          </div>
        </div>

        <hr className="divider footer-divider" />

        <div className="footer-bottom">
          <p className="footer-meta mb-0">
            © {new Date().getFullYear()} {business.name}. Todos los derechos reservados.
          </p>
          <nav className="footer-legal" aria-label="Información legal">
            <Link to="/privacidad">Política de Privacidad</Link>
            <Link to="/terminos-de-reserva">Términos de Reserva</Link>
          </nav>
          {socials.length > 0 && (
            <ul className="footer-socials">
              {socials.map(({ href, label, Icon }) => (
                <li key={label}>
                  <a href={href} target="_blank" rel="noopener noreferrer" aria-label={label}>
                    <Icon />
                  </a>
                </li>
              ))}
            </ul>
          )}
        </div>
      </div>
    </footer>
  )
}
