import { useState } from 'react'
import { Navigate, useNavigate } from 'react-router-dom'
import { login, isAuthenticated } from '../../services/authService'
import FormField from '../../components/ui/FormField'
import Button from '../../components/ui/Button'

export default function Login() {
  const navigate = useNavigate()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  // Si ya hay sesión, redirigimos declarativamente con <Navigate />, en vez
  // de llamar a navigate() durante el render (eso es un side-effect fuera de
  // lugar: React puede volver a ejecutar el render en modo estricto/concurrente
  // y navigate() no es una operación segura para hacer mientras se renderiza).
  if (isAuthenticated()) {
    return <Navigate to="/admin" replace />
  }

  const submit = async (e) => {
    e.preventDefault()
    setError('')
    setLoading(true)
    try {
      await login(email, password)
      navigate('/admin')
    } catch (err) {
      setError(err.message)
    } finally {
      setLoading(false)
    }
  }

  return (
    <section className="section auth-screen">
      <div className="container auth-container">
        <span className="eyebrow">Acceso restringido</span>
        <h2 className="mb-5">Panel Admin</h2>
        <form onSubmit={submit} noValidate className="d-grid gap-4">
          <FormField
            label="Email" type="email" required autoComplete="email"
            value={email} onChange={e => setEmail(e.target.value)}
          />
          <FormField
            label="Contraseña" type="password" required autoComplete="current-password"
            value={password} onChange={e => setPassword(e.target.value)}
            error={error}
          />
          <Button type="submit" variant="primary" loading={loading}>
            {loading ? 'Ingresando' : 'Ingresar'}
          </Button>
        </form>
      </div>
    </section>
  )
}
