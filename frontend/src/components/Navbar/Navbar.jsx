import { useEffect, useRef, useState } from 'react'
import { Link, NavLink, useNavigate } from 'react-router-dom'
import { UserIcon } from '../ui/Icons'
import { useCustomerAuth } from '../../context/CustomerAuthContext'

export default function Navbar() {
  const [menuOpen, setMenuOpen] = useState(false)
  const [dropdownOpen, setDropdownOpen] = useState(false)
  const [userOpen, setUserOpen] = useState(false)
  const { customer, isAuthenticated, logout } = useCustomerAuth()
  const navigate = useNavigate()
  const [scrolled, setScrolled] = useState(false)
  const dropdownRef = useRef(null)
  const navRef = useRef(null)

  const closeAll = () => { setMenuOpen(false); setDropdownOpen(false); setUserOpen(false) }

  const signOut = () => {
    closeAll()
    logout()
    navigate('/')
  }

  // Fondo del navbar al hacer scroll.
  useEffect(() => {
    const onScroll = () => setScrolled(window.scrollY > 16)
    onScroll()
    window.addEventListener('scroll', onScroll, { passive: true })
    return () => window.removeEventListener('scroll', onScroll)
  }, [])

  // Cerrar con Escape y al hacer click fuera del navbar (accesibilidad de teclado).
  useEffect(() => {
    const onKeyDown = (e) => { if (e.key === 'Escape') closeAll() }
    const onClickOutside = (e) => {
      if (navRef.current && !navRef.current.contains(e.target)) closeAll()
    }
    document.addEventListener('keydown', onKeyDown)
    document.addEventListener('mousedown', onClickOutside)
    return () => {
      document.removeEventListener('keydown', onKeyDown)
      document.removeEventListener('mousedown', onClickOutside)
    }
  }, [])

  return (
    <nav ref={navRef} className={`site-nav ${scrolled ? 'scrolled' : ''}`} aria-label="Principal">
      <div className="container d-flex align-items-center justify-content-between">
        <Link to="/" className="brand" onClick={closeAll}>YAMPIER</Link>

        <button
          className="nav-toggle"
          aria-label={menuOpen ? 'Cerrar menú' : 'Abrir menú'}
          aria-expanded={menuOpen}
          onClick={() => setMenuOpen(o => !o)}
        >
          <span aria-hidden="true">{menuOpen ? '✕' : '☰'}</span>
        </button>

        <div className={`nav-links ${menuOpen ? 'open' : ''}`} id="primary-navigation">
          <div className="nav-dropdown" ref={dropdownRef}>
            <button
              className="nav-link-item nav-dropdown-toggle"
              aria-haspopup="true"
              aria-expanded={dropdownOpen}
              onClick={() => { setUserOpen(false); setDropdownOpen(o => !o) }}
            >
              Relojes <span aria-hidden="true">{dropdownOpen ? '︿' : '﹀'}</span>
            </button>
            <div className={`nav-dropdown-panel ${dropdownOpen ? 'open' : ''}`} role="menu">
              <NavLink role="menuitem" className="nav-dropdown-item" to="/relojes" end onClick={closeAll}>
                Todos los relojes
              </NavLink>
              <NavLink role="menuitem" className="nav-dropdown-item" to="/relojes/hombre" onClick={closeAll}>
                Relojes de Hombre
              </NavLink>
              <NavLink role="menuitem" className="nav-dropdown-item" to="/relojes/mujer" onClick={closeAll}>
                Relojes de Mujer
              </NavLink>
            </div>
          </div>

          <NavLink
            className={({ isActive }) => `nav-link-item ${isActive ? 'active' : ''}`}
            to="/servicios"
            onClick={closeAll}
          >
            Servicios
          </NavLink>
          <NavLink
            className={({ isActive }) => `nav-link-item ${isActive ? 'active' : ''}`}
            to="/contacto"
            onClick={closeAll}
          >
            Contacto
          </NavLink>
          {/* Menú de cuenta de CLIENTE. No tiene relación con el panel Admin:
              nunca muestra opciones de administración. */}
          <div className="nav-dropdown nav-user-menu">
            <button
              type="button"
              className={`nav-user ${userOpen ? 'active' : ''}`}
              aria-label={isAuthenticated ? `Cuenta de ${customer.firstName}` : 'Ingresar o crear cuenta'}
              title={isAuthenticated ? 'Mi cuenta' : 'Ingresar'}
              aria-haspopup="true"
              aria-expanded={userOpen}
              onClick={() => { setDropdownOpen(false); setUserOpen(o => !o) }}
            >
              <UserIcon />
              <span className="nav-user-label" aria-hidden="true">
                {isAuthenticated ? customer.firstName : 'Mi cuenta'}
              </span>
            </button>
            <div className={`nav-dropdown-panel nav-user-panel ${userOpen ? 'open' : ''}`} role="menu">
              {isAuthenticated ? (
                <>
                  <NavLink role="menuitem" className="nav-dropdown-item" to="/mi-cuenta" end onClick={closeAll}>Mi cuenta</NavLink>
                  <NavLink role="menuitem" className="nav-dropdown-item" to="/mi-cuenta/reservas" onClick={closeAll}>Mis reservas</NavLink>
                  <button type="button" role="menuitem" className="nav-dropdown-item nav-dropdown-button" onClick={signOut}>
                    Cerrar sesión
                  </button>
                </>
              ) : (
                <>
                  <NavLink role="menuitem" className="nav-dropdown-item" to="/ingresar" onClick={closeAll}>Ingresar</NavLink>
                  <NavLink role="menuitem" className="nav-dropdown-item" to="/crear-cuenta" onClick={closeAll}>Crear cuenta</NavLink>
                  <NavLink role="menuitem" className="nav-dropdown-item" to="/como-reservar" onClick={closeAll}>Cómo reservar</NavLink>
                </>
              )}
            </div>
          </div>
        </div>
      </div>
    </nav>
  )
}
