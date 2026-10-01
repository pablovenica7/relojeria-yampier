import { useState } from 'react'
import { Link, Navigate, useNavigate, useSearchParams } from 'react-router-dom'
import { useCustomerAuth } from '../../context/CustomerAuthContext'
import { safeReturnTo } from '../../utils/returnTo'
import FormField from '../../components/ui/FormField'
import Button from '../../components/ui/Button'

// Login de CLIENTES. El panel Admin tiene su propio login en /admin/login.
export default function Ingresar() {
  const [params] = useSearchParams()
  const returnTo = safeReturnTo(params.get('returnTo'))
  const navigate = useNavigate()
  const { login, isAuthenticated } = useCustomerAuth()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  if (isAuthenticated && !loading) return <Navigate to={returnTo} replace />

  const submit = async (e) => {
    e.preventDefault()
    setError('')
    setLoading(true)
    try {
      await login(email, password)
      navigate(returnTo, { replace: true })
    } catch (err) {
      setError(err.message)
      setLoading(false)
    }
  }

  const registerLink = `/crear-cuenta${params.get('returnTo') ? `?returnTo=${encodeURIComponent(returnTo)}` : ''}`
  return (
    <section className="section auth-screen">
      <div className="container auth-container">
        <span className="eyebrow">Clientes</span>
        <h1 className="h2 mb-3">Ingresar</h1>
        {returnTo.startsWith('/reservar/') && (
          <p className="mb-4">Iniciá sesión o creá una cuenta para solicitar la reserva. No vas a perder el reloj que estabas viendo.</p>
        )}
        <form onSubmit={submit} noValidate className="d-grid gap-4">
          <FormField label="Email" type="email" required autoComplete="email" value={email} onChange={e => setEmail(e.target.value)} />
          <FormField
            label="Contraseña" type="password" required autoComplete="current-password"
            value={password} onChange={e => setPassword(e.target.value)} error={error}
          />
          <Button type="submit" variant="primary" loading={loading}>{loading ? 'Ingresando' : 'Ingresar'}</Button>
        </form>
        <p className="mt-5 mb-0">
          ¿No tenés cuenta? <Link to={registerLink} className="link-underline">Crear cuenta</Link>
        </p>
        <p className="mt-3">
          <Link to="/recuperar-contrasena" className="link-underline">¿Olvidaste tu contraseña?</Link>
        </p>
      </div>
    </section>
  )
}
