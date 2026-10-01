import { Link } from 'react-router-dom'
import { business } from '../../utils/siteConfig'
import { TERMS_VERSION } from '../../utils/legal'

// BORRADOR: condiciones base que Relojería Yampier debe revisar y completar
// (plazos, seña, devoluciones) con su asesor antes de publicar en producción.
export default function TerminosReserva() {
  return (
    <section className="section">
      <div className="container legal-container">
        <p className="legal-draft" role="note">
          Borrador pendiente de revisión legal y comercial. Los plazos y condiciones
          entre corchetes deben ser definidos por la relojería.
        </p>
        <span className="eyebrow">Legal</span>
        <h1 className="h2 mb-2">Términos de Reserva</h1>
        <p className="text-faint-token mb-6">Versión {TERMS_VERSION}</p>

        <h2 className="h5">1. Qué es una reserva</h2>
        <p>
          {business.name} no vende online. Desde el sitio podés solicitar la reserva
          de un reloj. La solicitud queda <strong>pendiente de confirmación</strong> y
          todavía no bloquea la unidad: la relojería revisa la disponibilidad y te
          contacta para confirmarla.
        </p>

        <h2 className="h5">2. Confirmación y seña</h2>
        <p>
          Cuando la relojería confirma la reserva, la unidad queda apartada para vos.
          Si corresponde, se coordina una seña directamente con el local (efectivo,
          transferencia o tarjeta en el local). No se cobran señas ni pagos a través
          de este sitio. [Definir: monto o porcentaje habitual de la seña.]
        </p>

        <h2 className="h5">3. Pago final y retiro</h2>
        <p>
          El pago final se realiza en nuestro local, al retirar el reloj. Para retirarlo
          vas a necesitar presentar el documento informado en tu cuenta.
        </p>

        <h2 className="h5">4. Vencimiento</h2>
        <p>
          Una solicitud pendiente que no se confirma vence automáticamente a los 7 días.
          Una reserva confirmada puede tener una fecha de vencimiento informada por la
          relojería. [Definir: plazo para retirar una reserva confirmada.]
        </p>

        <h2 className="h5">5. Cancelación</h2>
        <p>
          Podés cancelar una solicitud pendiente desde <Link to="/mi-cuenta/reservas" className="link-underline">Mis reservas</Link>.
          Las reservas confirmadas o con seña se cancelan contactando al local.
          [Definir con asesor: política de devolución de la seña según el caso.]
        </p>

        <h2 className="h5">6. Precios</h2>
        <p>
          El precio que figura en tu reserva es el vigente al momento de solicitarla y
          se mantiene aunque luego cambie el precio publicado. [Revisar: condiciones
          ante errores evidentes de precio y vigencia.]
        </p>
      </div>
    </section>
  )
}
