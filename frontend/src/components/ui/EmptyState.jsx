/**
 * Estado vacío reutilizable: se usa cuando una lista no tiene resultados
 * (catálogo sin relojes, consultas sin datos, errores de carga).
 */
export default function EmptyState({ icon = '—', title, description, action }) {
  return (
    <div className="empty-state" role="status">
      <div className="empty-state-icon" aria-hidden="true">{icon}</div>
      <h3>{title}</h3>
      {description && <p>{description}</p>}
      {action && <div className="mt-4">{action}</div>}
    </div>
  )
}
