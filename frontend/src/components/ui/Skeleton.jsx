/**
 * Placeholders de carga. Evitan el "salto" de layout y comunican mejor que
 * el estado de la página que un simple texto "Cargando…".
 */
export function SkeletonCardGrid({ count = 4 }) {
  return (
    <div className="card-grid" aria-hidden="true">
      {Array.from({ length: count }).map((_, i) => (
        <div className="skeleton skeleton-card" key={i} />
      ))}
    </div>
  )
}

export function SkeletonLines({ count = 3, width = '100%' }) {
  return (
    <div aria-hidden="true">
      {Array.from({ length: count }).map((_, i) => (
        <div
          className="skeleton skeleton-line"
          key={i}
          style={{ width: i === count - 1 ? '60%' : width }}
        />
      ))}
    </div>
  )
}
