import { useEffect, useState } from 'react'
import { Outlet } from 'react-router-dom'
import Navbar from '../Navbar/Navbar'
import Footer from '../Footer/Footer'
import Loader from '../Loader/Loader'
import BackToTop from '../BackToTop/BackToTop'
import useScrollTop from '../../hooks/useScrollTop'

/**
 * Layout persistente del sitio público. Al usarse como elemento de una
 * <Route> "envolvente" (ver App.jsx) con <Outlet /> para las rutas hijas,
 * React Router mantiene este componente montado al navegar entre páginas
 * públicas: el Loader inicial NO se remonta ni vuelve a mostrarse en cada
 * cambio de ruta, solo una vez cuando se carga el sitio.
 *
 * Antes, cada <Route> envolvía manualmente <SiteLayout><Home/></SiteLayout>,
 * <SiteLayout><Relojes/></SiteLayout>, etc. Como cada <Route> es un elemento
 * distinto, React desmontaba y volvía a montar SiteLayout completo en cada
 * navegación, reiniciando su temporizador del loader cada vez.
 */
export default function SiteLayout() {
  useScrollTop()
  const [showLoader, setShowLoader] = useState(true)

  useEffect(() => {
    const t = setTimeout(() => setShowLoader(false), 700)
    return () => clearTimeout(t)
  }, [])

  return (
    <>
      <a href="#main-content" className="skip-link">Saltar al contenido</a>
      <Loader visible={showLoader} />
      <Navbar />
      <main id="main-content">
        <Outlet />
      </main>
      <Footer />
      <BackToTop />
    </>
  )
}
