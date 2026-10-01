// Cinta de confianza en movimiento continuo. Es puramente decorativa
// (el mismo contenido ya está disponible en el texto de la página),
// así que se oculta de lectores de pantalla para no duplicar información.
export default function Marquee({ items }) {
  const loop = [...items, ...items]
  return (
    <div className="marquee" aria-hidden="true">
      <div className="marquee-track">
        {loop.map((t, i) => (
          <span className="marquee-item" key={i}>{t}</span>
        ))}
      </div>
    </div>
  )
}
