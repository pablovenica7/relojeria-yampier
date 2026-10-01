import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { resetPassword } from '../../services/customerService'
import FormField from '../../components/ui/FormField'
import Button from '../../components/ui/Button'

// Paso 2: elegir la contraseña nueva con el enlace recibido por email.
// El token se lee una vez y se quita de la barra de direcciones (no queda en
// el historial ni se filtra por Referer al navegar a otro sitio).
function takeTokenFromUrl() {
  const params = new URLSearchParams(window.location.search)
  const token = params.get('token') || ''
  if (token) window.history.replaceState(null, '', window.location.pathname)
  return token
}

export default function RestablecerContrasena() {
  const [token] = useState(takeTokenFromUrl)
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [errors, setErrors] = useState({})
  const [serverError, setServerError] = useState('')
  const [saving, setSaving] = useState(false)
  const [done, setDone] = useState(false)

  useEffect(() => {
    if (!token) setServerError('El enlace no es válido. Pedí uno nuevo.')
  }, [token])

  const submit = async (e) => {
    e.preventDefault()
    const next = {}
    if (password.length < 8 || !/\p{L}/u.test(password) || !/\d/.test(password)) next.password = 'Mínimo 8 caracteres, con letras y números'
    else if (password !== confirm) next.confirm = 'Las contraseñas no coinciden'
    setErrors(next)
    if (Object.keys(next).length > 0) return
    setSaving(true)
    setServerError('')
    try {
      await resetPassword(token, password)
      setDone(true)
    } catch (err) {
      setServerError(err.message)
    } finally {
      setSaving(false)
    }
  }

  return (
    <section className="section auth-screen">
      <div className="container auth-container">
        <span className="eyebrow">Clientes</span>
        <h1 className="h2 mb-4">Nueva contraseña</h1>
        {done ? (
          <div className="success-panel" role="status">
            <p>Listo, tu contraseña se actualizó. Por seguridad cerramos las sesiones abiertas en otros dispositivos.</p>
            <Link to="/ingresar" className="btn btn-primary mt-3">Ingresar</Link>
          </div>
        ) : (
          <form onSubmit={submit} noValidate className="d-grid gap-4">
            <FormField label="Contraseña nueva" type="password" required autoComplete="new-password"
              hint="Mínimo 8 caracteres, con letras y números" value={password} onChange={e => setPassword(e.target.value)} error={errors.password} />
            <FormField label="Repetir contraseña" type="password" required autoComplete="new-password"
              value={confirm} onChange={e => setConfirm(e.target.value)} error={errors.confirm} />
            {serverError && (
              <p className="form-alert" role="alert">
                {serverError} <Link to="/recuperar-contrasena" className="link-underline">Pedir un enlace nuevo</Link>
              </p>
            )}
            <Button type="submit" variant="primary" loading={saving} disabled={!token}>{saving ? 'Guardando' : 'Guardar contraseña'}</Button>
          </form>
        )}
      </div>
    </section>
  )
}
