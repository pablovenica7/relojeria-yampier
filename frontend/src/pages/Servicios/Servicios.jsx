import { Link } from 'react-router-dom'
import { services } from '../../utils/servicesData'
import RevealOnScroll from '../../components/RevealOnScroll/RevealOnScroll'

export default function Servicios() {
  return (
    <>
      <section className="section pb-0">
        <div className="container">
          <RevealOnScroll className="section-header">
            <span className="eyebrow">Lo que hacemos</span>
            <h2>Servicios</h2>
            <p>
              Trabajamos con relojes de cuarzo y mecánicos. Traelo al local y
              te decimos qué necesita, sin compromiso.
            </p>
          </RevealOnScroll>
        </div>
      </section>
      <section className="section pt-0">
        <div className="container">
          <div className="row">
            <div className="col-lg-9">
              {services.map(([t, d], i) => (
                <RevealOnScroll delay={Math.min(i * 0.04, 0.24)} key={t}>
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
          <div className="mt-6">
            <Link to="/contacto" className="btn btn-primary">Pedir presupuesto</Link>
          </div>
        </div>
      </section>
    </>
  )
}
