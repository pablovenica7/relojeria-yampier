import { memo } from 'react'
import { Link } from 'react-router-dom'
import WatchClock from '../WatchClock/WatchClock'
import TiltCard from '../TiltCard/TiltCard'
import StockBadge from '../ui/StockBadge'
import { imageUrl } from '../../utils/imageUrl'
import { formatCurrency } from '../../utils/formatCurrency'

// La card muestra solo lo esencial (imagen, nombre, precio y stock); la
// descripción y las especificaciones quedan para la página de detalle.
function WatchCard({ watch }) {
  return (
    <TiltCard>
      <Link to={`/reloj/${watch.id}`} className="watch-card d-block">
        <div className={`ph ${watch.image ? 'has-image' : ''}`}>
          {watch.image
            ? <img src={imageUrl(watch.image)} alt={`Reloj ${watch.name}`} loading="lazy" decoding="async" width="800" height="800" />
            : <WatchClock hour={10} minute={8 + (watch.id?.length || 1) * 3} label={watch.name} />}
        </div>
        <div className="body">
          <h3>{watch.name}</h3>
          <p className="watch-card-price">{formatCurrency(watch.priceARS) || 'Consultar precio'}</p>
          <StockBadge quantity={watch.stockQuantity} />
        </div>
      </Link>
    </TiltCard>
  )
}

// Evita re-render si el objeto watch no cambió (listas de hasta ~12 items).
export default memo(WatchCard)
