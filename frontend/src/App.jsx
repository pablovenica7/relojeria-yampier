import { Suspense, lazy } from 'react'
import { Routes, Route, Navigate } from 'react-router-dom'
import SiteLayout from './components/layout/SiteLayout'
import Home from './pages/Home/Home'
import Relojes from './pages/Relojes/Relojes'
import RelojDetalle from './pages/RelojDetalle/RelojDetalle'
import Servicios from './pages/Servicios/Servicios'
import Contacto from './pages/Contacto/Contacto'
import ComoReservar from './pages/ComoReservar/ComoReservar'
import NotFound from './pages/NotFound/NotFound'
import ProtectedRoute from './components/Admin/ProtectedRoute'
import CustomerRoute from './components/Customer/CustomerRoute'
import { CustomerAuthProvider } from './context/CustomerAuthContext'
import Ingresar from './pages/Cuenta/Ingresar'
import CrearCuenta from './pages/Cuenta/CrearCuenta'
import RecuperarContrasena from './pages/Cuenta/RecuperarContrasena'
import RestablecerContrasena from './pages/Cuenta/RestablecerContrasena'
import Privacidad from './pages/Legal/Privacidad'
import TerminosReserva from './pages/Legal/TerminosReserva'

// Páginas de cliente con sesión: chunk aparte (no las usa la mayoría de las visitas).
const MiCuenta = lazy(() => import('./pages/Cuenta/MiCuenta'))
const MisReservas = lazy(() => import('./pages/Cuenta/MisReservas'))
const Reservar = lazy(() => import('./pages/Reservar/Reservar'))

// El panel Admin se carga en un chunk aparte: la gran mayoría de las visitas
// son al sitio público, así que no tiene sentido que ese código (formularios,
// tablas, lógica de autenticación) viaje en el bundle inicial de Home.
const AdminLayout = lazy(() => import('./components/Admin/AdminLayout'))
const AdminLogin = lazy(() => import('./pages/Admin/Login'))
const AdminWatchesList = lazy(() => import('./pages/Admin/WatchesList'))
const AdminWatchForm = lazy(() => import('./pages/Admin/WatchForm'))
const AdminInquiries = lazy(() => import('./pages/Admin/Inquiries'))
const AdminDashboard = lazy(() => import('./pages/Admin/Dashboard'))
const AdminStockMovements = lazy(() => import('./pages/Admin/StockMovements'))
const AdminCustomers = lazy(() => import('./pages/Admin/Customers'))
const AdminReservations = lazy(() => import('./pages/Admin/Reservations'))
const AdminReservationForm = lazy(() => import('./pages/Admin/ReservationForm'))

function RouteFallback() {
  return (
    <div className="route-fallback" role="status" aria-live="polite">
      <span className="spinner" aria-hidden="true" />
      <span className="visually-hidden">Cargando…</span>
    </div>
  )
}

export default function App() {
  return (
    <Routes>
      {/* Sitio público: SiteLayout es un layout real de React Router (usa
          <Outlet/>), así que se mantiene montado al navegar entre estas
          rutas en vez de desmontarse y remontarse en cada click. */}
      <Route element={<CustomerAuthProvider><SiteLayout /></CustomerAuthProvider>}>
        <Route path="/" element={<Home />} />
        <Route path="/relojes" element={<Relojes />} />
        <Route path="/relojes/:genero" element={<Relojes />} />
        <Route path="/reloj/:id" element={<RelojDetalle />} />
        <Route path="/servicios" element={<Servicios />} />
        <Route path="/contacto" element={<Contacto />} />
        <Route path="/como-reservar" element={<ComoReservar />} />
        {/* Ruta anterior: se redirige para no romper links ya compartidos. */}
        <Route path="/mi-reserva" element={<Navigate to="/como-reservar" replace />} />
        <Route path="/privacidad" element={<Privacidad />} />
        <Route path="/terminos-de-reserva" element={<TerminosReserva />} />
        <Route path="/ingresar" element={<Ingresar />} />
        <Route path="/crear-cuenta" element={<CrearCuenta />} />
        <Route path="/recuperar-contrasena" element={<RecuperarContrasena />} />
        <Route path="/restablecer-contrasena" element={<RestablecerContrasena />} />
        <Route path="/mi-cuenta" element={<CustomerRoute><Suspense fallback={<RouteFallback />}><MiCuenta /></Suspense></CustomerRoute>} />
        <Route path="/mi-cuenta/reservas" element={<CustomerRoute><Suspense fallback={<RouteFallback />}><MisReservas /></Suspense></CustomerRoute>} />
        <Route path="/reservar/:id" element={<CustomerRoute><Suspense fallback={<RouteFallback />}><Reservar /></Suspense></CustomerRoute>} />
        <Route path="*" element={<NotFound />} />
      </Route>

      {/* Panel Admin: código cargado bajo demanda (React.lazy + Suspense). */}
      <Route
        path="/admin/login"
        element={<Suspense fallback={<RouteFallback />}><AdminLogin /></Suspense>}
      />
      <Route
        path="/admin"
        element={
          <ProtectedRoute>
            <Suspense fallback={<RouteFallback />}><AdminLayout /></Suspense>
          </ProtectedRoute>
        }
      >
        <Route index element={<Suspense fallback={<RouteFallback />}><AdminDashboard /></Suspense>} />
        <Route path="stock" element={<Suspense fallback={<RouteFallback />}><AdminStockMovements /></Suspense>} />
        <Route path="clientes" element={<Suspense fallback={<RouteFallback />}><AdminCustomers /></Suspense>} />
        <Route path="relojes" element={<Suspense fallback={<RouteFallback />}><AdminWatchesList /></Suspense>} />
        <Route path="relojes/nuevo" element={<Suspense fallback={<RouteFallback />}><AdminWatchForm /></Suspense>} />
        <Route path="relojes/:id/editar" element={<Suspense fallback={<RouteFallback />}><AdminWatchForm /></Suspense>} />
        <Route path="consultas" element={<Suspense fallback={<RouteFallback />}><AdminInquiries /></Suspense>} />
        <Route path="reservas" element={<Suspense fallback={<RouteFallback />}><AdminReservations /></Suspense>} />
        <Route path="reservas/nueva" element={<Suspense fallback={<RouteFallback />}><AdminReservationForm /></Suspense>} />
      </Route>
    </Routes>
  )
}
