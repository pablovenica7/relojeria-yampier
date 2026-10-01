import { useEffect, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { sendInquiry } from '../../services/inquiriesService'
import { getWatch } from '../../services/watchesService'
import RevealOnScroll from '../../components/RevealOnScroll/RevealOnScroll'
import FormField from '../../components/ui/FormField'
import Button from '../../components/ui/Button'
import swal from '../../utils/swal'
import { business, contact, mailtoUrl, whatsappUrl } from '../../utils/siteConfig'

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

function validate(form) {
  const errors = {}
  if (!form.name.trim()) errors.name = 'Ingresá tu nombre'
  if (!form.email.trim()) errors.email = 'Ingresá tu email'
  else if (!EMAIL_RE.test(form.email)) errors.email = 'El email no es válido'
  if (!form.message.trim()) errors.message = 'Contanos brevemente tu consulta'
  else if (form.message.trim().length < 10) errors.message = 'Escribí un poco más de detalle (mínimo 10 caracteres)'
  return errors
}

export default function Contacto() {
  const [params] = useSearchParams()
  const ref = params.get('ref') // compatibilidad con links viejos (?ref=Nombre)
  const watchId = params.get('watch')

  const [form, setForm] = useState({
    name: '', email: '', phone: '',
    message: ref ? `Hola, quiero consultar por el reloj ${ref}.` : '',
  })
  const [errors, setErrors] = useState({})
  const [touched, setTouched] = useState({})
  const [sending, setSending] = useState(false)
  // Reloj que originó la consulta (?watch=<id>): se envía su id para que la
  // consulta quede vinculada aunque después se cambie el nombre del reloj.
  const [watch, setWatch] = useState(null)

  useEffect(() => {
    if (!watchId) return
    let alive = true
    getWatch(watchId)
      .then(w => {
        if (!alive) return
        setWatch(w)
        setForm(f => (f.message ? f : { ...f, message: `Hola, quisiera consultar disponibilidad del ${w.name}.` }))
      })
      .catch(() => {}) // reloj inexistente o archivado: queda como consulta general
    return () => { alive = false }
  }, [watchId])

  const set = (k) => (e) => {
    const value = e.target.value
    setForm(f => ({ ...f, [k]: value }))
    if (touched[k]) setErrors(validate({ ...form, [k]: value }))
  }
  const blur = (k) => () => {
    setTouched(t => ({ ...t, [k]: true }))
    setErrors(validate(form))
  }

  const submit = async (e) => {
    e.preventDefault()
    const nextErrors = validate(form)
    setErrors(nextErrors)
    setTouched({ name: true, email: true, message: true })
    if (Object.keys(nextErrors).length > 0) return

    setSending(true)
    try {
      await sendInquiry(watch ? { ...form, watchId: watch.id } : form)
      setForm({ name: '', email: '', phone: '', message: '' })
      setWatch(null)
      setTouched({})
      await swal.fire({
        icon: 'success',
        title: 'Consulta enviada',
        text: 'Te respondemos a la brevedad.',
        confirmButtonText: 'Perfecto',
      })
    } catch (err) {
      await swal.fire({
        icon: 'error',
        title: 'No se pudo enviar',
        text: err.message,
        confirmButtonText: 'Entendido',
      })
    } finally {
      setSending(false)
    }
  }

  return (
    <section className="section">
      <div className="container">
        <div className="section-header">
          <span className="eyebrow">Hablemos</span>
          <h2>Contacto</h2>
        </div>
        <div className="row g-5">
          <div className="col-lg-5">
            <RevealOnScroll>
              <p className="text-strong-token mb-2">{business.name}</p>
              <address className="mb-4">
                <p className="mb-0">{business.address.street}</p>
                <p className="mb-0">{business.address.area}</p>
                <p className="mb-0">{business.address.city}, {business.address.country}</p>
              </address>
              <p>
                Teléfono:{" "}
                <a className="link-underline" href={`tel:${business.phone.tel}`}>{business.phone.display}</a>
              </p>
              {contact.whatsappNumber && (
                <p>
                  <a className="link-underline" href={whatsappUrl('Hola, quisiera hacer una consulta.')} target="_blank" rel="noopener noreferrer">
                    Escribinos por WhatsApp
                  </a>
                </p>
              )}
              {contact.email && (
                <p>
                  Email: <a className="link-underline" href={mailtoUrl('Consulta desde la web')}>{contact.email}</a>
                </p>
              )}
              {business.hours.length > 0
                ? business.hours.map(h => <p className="mb-1" key={h.days}>{h.days}: {h.hours}</p>)
                : <p className="text-faint-token">{business.hoursNotice}</p>}
              <p className="text-muted-token mt-4">
                Las reservas se coordinan directamente con el local y el pago final
                se realiza de forma presencial.
              </p>
            </RevealOnScroll>
          </div>
          <div className="col-lg-7">
            <RevealOnScroll delay={0.08}>
              <form onSubmit={submit} noValidate className="d-grid gap-4">
                {watch && (
                  <p className="contact-watch-ref mb-0" role="status">
                    Consulta sobre: <span className="text-strong-token">{watch.name}</span>
                    {watch.model && <span className="text-faint-token"> · {watch.model}</span>}
                  </p>
                )}
                <FormField
                  label="Nombre" required
                  value={form.name} onChange={set('name')} onBlur={blur('name')}
                  error={touched.name ? errors.name : ''}
                  autoComplete="name"
                />
                <FormField
                  label="Email" type="email" required
                  value={form.email} onChange={set('email')} onBlur={blur('email')}
                  error={touched.email ? errors.email : ''}
                  autoComplete="email"
                />
                <FormField
                  label="Teléfono" hint="Opcional"
                  value={form.phone} onChange={set('phone')}
                  autoComplete="tel"
                />
                <FormField
                  label="Mensaje" as="textarea" required rows={4}
                  value={form.message} onChange={set('message')} onBlur={blur('message')}
                  error={touched.message ? errors.message : ''}
                />
                <div>
                  <Button type="submit" variant="primary" loading={sending}>
                    {sending ? 'Enviando' : 'Enviar consulta'}
                  </Button>
                </div>
              </form>
            </RevealOnScroll>
          </div>
        </div>
      </div>
    </section>
  )
}
