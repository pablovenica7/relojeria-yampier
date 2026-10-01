import { NavLink, Outlet, useNavigate } from 'react-router-dom'
import { logout } from '../../services/authService'
import Button from '../ui/Button'

export default function AdminLayout() {
  const navigate = useNavigate()

  const salir = () => {
    logout()
    navigate('/admin/login')
  }

  return (
    <div className="admin-shell">
      <aside className="admin-sidebar">
        <div className="admin-brand">YAMPIER <span>Admin</span></div>
        <nav className="admin-nav" aria-label="Panel de administración">
          <NavLink to="/admin" end className={({ isActive }) => `admin-link ${isActive ? 'active' : ''}`}>
            Inicio
          </NavLink>
          <NavLink to="/admin/relojes" className={({ isActive }) => `admin-link ${isActive ? 'active' : ''}`}>
            Relojes
          </NavLink>
          <NavLink to="/admin/reservas" className={({ isActive }) => `admin-link ${isActive ? 'active' : ''}`}>
            Reservas
          </NavLink>
          <NavLink to="/admin/stock" className={({ isActive }) => `admin-link ${isActive ? 'active' : ''}`}>
            Stock
          </NavLink>
          <NavLink to="/admin/clientes" className={({ isActive }) => `admin-link ${isActive ? 'active' : ''}`}>
            Clientes
          </NavLink>
          <NavLink to="/admin/consultas" className={({ isActive }) => `admin-link ${isActive ? 'active' : ''}`}>
            Consultas
          </NavLink>
        </nav>
        <Button variant="secondary" size="sm" className="mt-auto" onClick={salir}>
          Cerrar sesión
        </Button>
      </aside>
      <main className="admin-content">
        <Outlet />
      </main>
    </div>
  )
}
