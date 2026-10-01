import { useState } from 'react'
import { Link } from 'react-router-dom'
import { forgotPassword } from '../../services/customerService'
import FormField from '../../components/ui/FormField'
import Button from '../../components/ui/Button'

// Paso 1: pedir el enlace. La respuesta es siempre la misma, exista o no la
// cuenta (no se revela qué emails están registrados).
export default function RecuperarContrasena() {
  const [email, setEmail] = useState('')
  const [sent, setSent] = useState('')
  const [error, setError] = useState('')
  const [sending, setSending] = useState(false)

  const submit = async (e) => {
    e.preventDefault()
    setError('')
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.trim())) {
      setError('Ingresá un email válido')
      return
    }
    setSending(true)
    try {
      const res = await forgotPassword(email.trim())
      setSent(res.message)
    } catch (err) {
      setError(err.message)
    } finally {
      setSending(false)
    }
  }

  return (
    <section className="section auth-screen">
      <div className="container auth-container">
        <span className="eyebrow">Clientes</span>
        <h1 className="h2 mb-3">Recuperar contraseña</h1>
        {sent ? (
          <div className="success-panel" role="status">
            <p>{sent}</p>
            <p className="mb-0">El enlace vence en 30 minutos. Si no lo ves, revisá la carpeta de spam.</p>
          </div>
        ) : (
          <>
            <p className="mb-5">Te enviamos un enlace por email para que elijas una contraseña nueva.</p>
            <form onSubmit={submit} noValidate className="d-grid gap-4">
              <FormField label="Email" type="email" required autoComplete="email" value={email} onChange={e => setEmail(e.target.value)} error={error} />
              <Button type="submit" variant="primary" loading={sending}>{sending ? 'Enviando' : 'Enviar enlace'}</Button>
            </form>
          </>
        )}
        <p className="mt-5 mb-0"><Link to="/ingresar" className="link-underline">Volver a Ingresar</Link></p>
      </div>
    </section>
  )
}
