import './Loader.css'

// Puramente presentacional: quién decide cuándo mostrarse y por cuánto
// tiempo es SiteLayout (una sola fuente de verdad para el timing, en vez de
// tener un timer acá Y otro en el layout controlando lo mismo).
export default function Loader({ visible }) {
  return (
    <div className={`loader-overlay ${visible ? '' : 'hide'}`} aria-hidden={!visible}>
      <div>
        <div className="loader-mark">YAMPIER</div>
        <div className="loader-line" />
      </div>
    </div>
  )
}
