import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { motion } from 'motion/react'
import WatchClock from '../../components/WatchClock/WatchClock'
import WatchCard from '../../components/WatchCard/WatchCard'
import SplitText from '../../components/SplitText/SplitText'
import RevealOnScroll from '../../components/RevealOnScroll/RevealOnScroll'
import Marquee from '../../components/Marquee/Marquee'
import { SkeletonCardGrid } from '../../components/ui/Skeleton'
import { getWatches } from '../../services/watchesService'
import { services } from '../../utils/servicesData'
import { CATALOG_BRAND } from '../../utils/siteConfig'

const trust = [
  'Taller propio', 'Marcas seleccionadas', 'Repuestos originales',
  'Presupuesto sin cargo', 'Córdoba, Argentina',
]

export default function Home() {
  const [featured, setFeatured] = useState([])
  const [state, setState] = useState('loading')

  useEffect(() => {
    // Destacados: los 4 relojes disponibles más recientes.
    getWatches({ brand: CATALOG_BRAND, availability: 'available', limit: 4 })
      .then(d => { setFeatured(d.items); setState('ok') })
      .catch(() => setState('error'))
  }, [])

  return (
    <>
      <section className="hero">
        <div className="container">
          <div className="row align-items-center g-5">
            <div className="col-lg-6 order-2 order-lg-1">
              <span className="eyebrow hero-eyebrow">Relojería · Córdoba</span>
              <SplitText text="El tiempo, en buenas manos." className="mb-4" />
              <motion.p
                className="hero-lead mb-5"
                initial={{ opacity: 0, y: 12 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.6, delay: 0.5 }}
              >
                Venta, reparación y mantenimiento de relojes en el centro de Córdoba.
                Consultá disponibilidad online y reservá tu reloj con nuestro local.
              </motion.p>
              <motion.div
                className="hero-actions"
                initial={{ opacity: 0, y: 12 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.6, delay: 0.65 }}
              >
                <Link to="/relojes" className="btn btn-primary">Ver relojes</Link>
                <Link to="/como-reservar" className="btn btn-secondary">Cómo reservar</Link>
              </motion.div>
            </div>
            <div className="col-lg-6 order-1 order-lg-2">
              <motion.div
                className="hero-watch"
                initial={{ opacity: 0, scale: 0.94 }}
                animate={{ opacity: 1, scale: 1 }}
                transition={{ duration: 1, delay: 0.15, ease: [0.16, 1, 0.3, 1] }}
              >
                <WatchClock live label="Reloj Yampier con la hora actual" />
              </motion.div>
            </div>
          </div>
        </div>
      </section>

      <Marquee items={trust} />

      <section className="section">
        <div className="container">
          <RevealOnScroll className="d-flex justify-content-between align-items-end flex-wrap gap-3 section-header mb-0">
            <div className="mb-0">
              <span className="eyebrow">Catálogo</span>
              <h2 className="mb-0">Selección {CATALOG_BRAND}</h2>
            </div>
            <Link to="/relojes" className="link-underline">Ver todos</Link>
          </RevealOnScroll>

          <div className="mt-8">
            {state === 'loading' && <SkeletonCardGrid count={4} />}
            {state === 'ok' && (
              <div className="card-grid">
                {featured.slice(0, 4).map((w, i) => (
                  <RevealOnScroll delay={i * 0.06} key={w.id}><WatchCard watch={w} /></RevealOnScroll>
                ))}
              </div>
            )}
          </div>
        </div>
      </section>

      <section className="section section-alt">
        <div className="container">
          <div className="row g-5 align-items-start">
            <div className="col-lg-5">
              <RevealOnScroll>
                <span className="eyebrow">Nuestro trabajo</span>
                <h2>Taller propio</h2>
                <p className="mb-5">
                  Cada reloj se revisa en el taller antes de entregarlo. También reparamos
                  y mantenemos el tuyo, sea de cuarzo o mecánico.
                </p>
                <Link to="/servicios" className="btn btn-secondary">Ver todos los servicios</Link>
              </RevealOnScroll>
            </div>
            <div className="col-lg-7">
              {services.slice(0, 4).map(([t, d], i) => (
                <RevealOnScroll delay={i * 0.05} key={t}>
                  <div className="service-row">
                    <span className="index">{String(i + 1).padStart(2, '0')}</span>
                    <div>
                      <h3>{t}</h3>
                      <p>{d}</p>
                    </div>
                  </div>
                </RevealOnScroll>
              ))}
            </div>
          </div>
        </div>
      </section>

      <section className="section text-center">
        <div className="container">
          <RevealOnScroll>
            <span className="eyebrow">Empecemos</span>
            <h2 className="mb-6">
              ¿Buscás un reloj o querés reparar el tuyo?
            </h2>
            <Link to="/contacto" className="btn btn-primary">Contactanos</Link>
          </RevealOnScroll>
        </div>
      </section>
    </>
  )
}
