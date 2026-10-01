// Flujo comercial de Yampier: no hay compra ni pago online. Se usa en el
// detalle de cada reloj y en "Cómo reservar".
export const reservationSteps = [
  ['Elegí tu reloj', 'Revisá el catálogo y tocá “Reservar” en el reloj que te interesa.'],
  ['Solicitá la reserva', 'Ingresá o creá tu cuenta y enviá la solicitud. Queda pendiente: todavía no bloquea la unidad.'],
  ['Confirmamos y coordinamos la seña', 'La relojería revisa la disponibilidad, te contacta y, si corresponde, acuerda una seña.'],
  ['Retiralo en el local', 'Completás el pago de forma presencial en nuestra tienda.'],
]

export default function ReservationSteps({ compact = false }) {
  return (
    <ol className={`reservation-steps ${compact ? 'is-compact' : ''}`}>
      {reservationSteps.map(([title, text], i) => (
        <li key={title}>
          <span className="index" aria-hidden="true">{String(i + 1).padStart(2, '0')}</span>
          <div>
            <h3>{title}</h3>
            <p>{text}</p>
          </div>
        </li>
      ))}
    </ol>
  )
}
