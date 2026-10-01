import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react'
import { CUSTOMER_SESSION_EXPIRED } from '../services/api'
import { getMe, hasCustomerSession, loginCustomer, logoutCustomer, registerCustomer } from '../services/customerService'

/**
 * Sesión del cliente para toda la app pública (Navbar, Reservar, Mi cuenta).
 * React Context alcanza para esto: no hace falta una librería de estado.
 * status: 'loading' | 'authenticated' | 'anonymous'.
 */
const CustomerAuthContext = createContext(null)

export function CustomerAuthProvider({ children }) {
  const [customer, setCustomer] = useState(null)
  const [status, setStatus] = useState(hasCustomerSession() ? 'loading' : 'anonymous')

  // Al cargar el sitio con un token guardado, se valida contra el backend.
  useEffect(() => {
    if (!hasCustomerSession()) return
    getMe()
      .then(c => { setCustomer(c); setStatus('authenticated') })
      .catch(() => { logoutCustomer(); setCustomer(null); setStatus('anonymous') })
  }, [])

  // Si una llamada responde 401 (token vencido), se cierra la sesión local.
  useEffect(() => {
    const onExpired = () => { setCustomer(null); setStatus('anonymous') }
    window.addEventListener(CUSTOMER_SESSION_EXPIRED, onExpired)
    return () => window.removeEventListener(CUSTOMER_SESSION_EXPIRED, onExpired)
  }, [])

  const login = useCallback(async (email, password) => {
    const c = await loginCustomer(email, password)
    setCustomer(c)
    setStatus('authenticated')
    return c
  }, [])

  const register = useCallback(async (data) => {
    const c = await registerCustomer(data)
    setCustomer(c)
    setStatus('authenticated')
    return c
  }, [])

  const logout = useCallback(() => {
    logoutCustomer()
    setCustomer(null)
    setStatus('anonymous')
  }, [])

  const value = useMemo(
    () => ({ customer, status, isAuthenticated: status === 'authenticated', login, register, logout, setCustomer }),
    [customer, status, login, register, logout]
  )
  return <CustomerAuthContext.Provider value={value}>{children}</CustomerAuthContext.Provider>
}

export function useCustomerAuth() {
  const ctx = useContext(CustomerAuthContext)
  if (!ctx) throw new Error('useCustomerAuth debe usarse dentro de <CustomerAuthProvider>')
  return ctx
}
