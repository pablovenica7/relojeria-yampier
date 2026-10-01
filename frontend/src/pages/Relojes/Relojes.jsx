import { useCallback } from 'react'
import { useParams, useSearchParams } from 'react-router-dom'
import { getWatches } from '../../services/watchesService'
import WatchCard from '../../components/WatchCard/WatchCard'
import RevealOnScroll from '../../components/RevealOnScroll/RevealOnScroll'
import EmptyState from '../../components/ui/EmptyState'
import Button from '../../components/ui/Button'
import Pagination from '../../components/ui/Pagination'
import { SkeletonCardGrid } from '../../components/ui/Skeleton'
import { useAsync } from '../../hooks/useAsync'
import { CATALOG_BRAND } from '../../utils/siteConfig'

const titles = { hombre: 'de Hombre', mujer: 'de Mujer' }
const PAGE_SIZE = 24

export default function Relojes() {
  const { genero } = useParams()
  const [params, setParams] = useSearchParams()
  // /relojes muestra todo el catálogo; /relojes/hombre|mujer filtra por género.
  const gender = genero === 'hombre' || genero === 'mujer' ? genero : ''
  const subtitle = gender ? titles[gender] : ''
  const page = Math.max(1, Number(params.get('page')) || 1)

  const fetcher = useCallback(
    () => getWatches({ gender, brand: CATALOG_BRAND, page, limit: PAGE_SIZE }),
    [gender, page]
  )
  const { state, data, reload } = useAsync(fetcher, [gender, page])
  const items = data?.items || []

  const goTo = (p) => {
    setParams(p > 1 ? { page: String(p) } : {})
    window.scrollTo({ top: 0 })
  }

  return (
    <section className="section">
      <div className="container">
        <div className="section-header">
          <span className="eyebrow">Catálogo</span>
          <h2>Relojes {CATALOG_BRAND} {subtitle}</h2>
          <p>
            Consultá disponibilidad y reservá por WhatsApp o email. El pago se
            realiza en nuestro local.
          </p>
        </div>

        {state === 'loading' && <SkeletonCardGrid count={8} />}

        {state === 'error' && (
          <EmptyState
            icon="!"
            title="No pudimos cargar los relojes"
            description="Verificá que el servidor esté activo e intentá de nuevo."
            action={<Button variant="secondary" size="sm" onClick={reload}>Reintentar</Button>}
          />
        )}

        {state === 'success' && items.length === 0 && (
          <EmptyState
            icon="—"
            title="Todavía no hay relojes en esta categoría"
            description="Muy pronto vamos a sumar el catálogo completo."
          />
        )}

        {state === 'success' && items.length > 0 && (
          <>
            <div className="card-grid">
              {items.map((w, i) => (
                <RevealOnScroll delay={Math.min(i * 0.05, 0.3)} key={w.id}><WatchCard watch={w} /></RevealOnScroll>
              ))}
            </div>
            <Pagination page={data.page} pages={data.pages} onChange={goTo} />
          </>
        )}
      </div>
    </section>
  )
}
