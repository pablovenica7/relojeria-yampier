import Button from './Button'

/** Paginación simple (Anterior / Siguiente). No se muestra si hay una sola página. */
export default function Pagination({ page, pages, onChange }) {
  if (!pages || pages <= 1) return null
  return (
    <nav className="pagination-bar" aria-label="Paginación">
      <Button variant="secondary" size="sm" disabled={page <= 1} onClick={() => onChange(page - 1)}>
        Anterior
      </Button>
      <span className="pagination-info" aria-live="polite">Página {page} de {pages}</span>
      <Button variant="secondary" size="sm" disabled={page >= pages} onClick={() => onChange(page + 1)}>
        Siguiente
      </Button>
    </nav>
  )
}
