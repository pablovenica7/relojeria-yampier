import { Link } from 'react-router-dom'

export default function NotFound() {
  return (
    <section className="section text-center">
      <div className="container">
        <span className="eyebrow">Error 404</span>
        <h1 className="h2 mb-3">Página no encontrada</h1>
        <p className="mb-5">El enlace no existe o fue movido.</p>
        <Link to="/" className="btn btn-primary">Volver al inicio</Link>
      </div>
    </section>
  )
}
