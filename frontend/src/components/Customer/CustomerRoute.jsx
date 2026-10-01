import { Navigate, useLocation } from 'react-router-dom'
import { useCustomerAuth } from '../../context/CustomerAuthContext'
import { loginPath } from '../../utils/returnTo'

/**
 * Protege páginas de clientes. Sin sesión, manda a /ingresar conservando la
 * página actual en ?returnTo= (validado con safeReturnTo al volver).
 * Es solo UX: la autorización real la hace el backend en cada request.
 */
export default function CustomerRoute({ children }) {
  const { status } = useCustomerAuth()
  const location = useLocation()

  if (status === 'loading') {
    return (
      <div className="route-fallback" role="status" aria-live="polite">
        <span className="spinner" aria-hidden="true" />
        <span className="visually-hidden">Cargando…</span>
      </div>
    )
  }
  if (status !== 'authenticated') {
    return <Navigate to={loginPath(location.pathname + location.search)} replace />
  }
  return children
}
