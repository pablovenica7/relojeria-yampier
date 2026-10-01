/**
 * Datos del negocio: única fuente de verdad para Navbar, Footer, Contacto,
 * detalle de reloj y "Cómo reservar". Si cambia la dirección, un teléfono o los
 * horarios, se edita solo acá.
 *
 * WhatsApp, email y redes sociales NO se completan a mano en este archivo:
 * llegan por variables de entorno (frontend/.env o el .env de la raíz para
 * Docker) y, si están vacías, los botones/enlaces correspondientes no se
 * muestran. Nunca se inventa un número o perfil.
 */
const env = import.meta.env

export const business = {
  name: 'Relojería Yampier',
  address: {
    street: 'Bernardino Rivadavia 59',
    area: 'San Martín Centro',
    city: 'Córdoba Capital',
    postalCode: 'X5000',
    country: 'Argentina',
  },
  // Teléfono fijo publicado del local (no se asume que tenga WhatsApp).
  phone: { display: '0351 422-9087', tel: '+543514229087' },
  // Horarios: las fuentes públicas no coinciden (aprox. lunes a viernes
  // 08:30 a 20:00, sábado horario reducido, domingo cerrado). Hasta
  // confirmarlos con el local se deja la lista vacía y se muestra el aviso.
  // Formato al confirmarlos: [{ days: 'Lunes a viernes', hours: '08:30 a 20:00' }, ...]
  hours: [],
  hoursNotice: 'Horarios sujetos a confirmación. Llamanos antes de acercarte.',
}

// Por ahora el catálogo público muestra solamente esta marca.
export const CATALOG_BRAND = 'Casio'

/** Deja solo dígitos (formato que exige wa.me: código de país + número, sin "+"). */
export function normalizeWhatsAppNumber(value) {
  return String(value || '').replace(/\D/g, '')
}

export const contact = {
  whatsappNumber: normalizeWhatsAppNumber(env.VITE_WHATSAPP_NUMBER),
  email: (env.VITE_CONTACT_EMAIL || '').trim(),
  instagramUrl: (env.VITE_INSTAGRAM_URL || '').trim(),
  facebookUrl: (env.VITE_FACEBOOK_URL || '').trim(),
}

/** Link de WhatsApp con mensaje precargado, o '' si no hay número configurado. */
export function whatsappUrl(message, number = contact.whatsappNumber) {
  if (!number) return ''
  const text = message ? `?text=${encodeURIComponent(message)}` : ''
  return `https://wa.me/${number}${text}`
}

/** Link mailto con asunto y cuerpo, o '' si no hay email configurado. */
export function mailtoUrl(subject, body, email = contact.email) {
  if (!email) return ''
  const params = new URLSearchParams()
  if (subject) params.set('subject', subject)
  if (body) params.set('body', body)
  const qs = params.toString().replace(/\+/g, '%20')
  return `mailto:${email}${qs ? `?${qs}` : ''}`
}

/** Mensaje estándar para consultar por un reloj concreto. */
export function watchInquiryMessage(watch) {
  return `Hola, quisiera consultar disponibilidad del ${watch.name}.`
}
