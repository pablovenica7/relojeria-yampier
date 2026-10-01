import { Link } from 'react-router-dom'
import { business } from '../../utils/siteConfig'
import { PRIVACY_VERSION } from '../../utils/legal'

// BORRADOR: texto base para que Relojería Yampier lo revise con su asesor
// legal antes de publicar en producción. No constituye asesoramiento jurídico.
export default function Privacidad() {
  return (
    <section className="section">
      <div className="container legal-container">
        <p className="legal-draft" role="note">
          Borrador pendiente de revisión legal y comercial. Este texto debe ser revisado
          por un profesional antes de usarse en producción.
        </p>
        <span className="eyebrow">Legal</span>
        <h1 className="h2 mb-2">Política de Privacidad</h1>
        <p className="text-faint-token mb-6">Versión {PRIVACY_VERSION}</p>

        <h2 className="h5">Quién es responsable de tus datos</h2>
        <p>
          {business.name}, con domicilio en {business.address.street}, {business.address.city}.
          [Completar: razón social, CUIT y datos de contacto para consultas de privacidad.]
        </p>

        <h2 className="h5">Qué datos pedimos y para qué</h2>
        <ul>
          <li>Nombre, apellido, email y teléfono: para crear tu cuenta, gestionar tus reservas y contactarte por ellas.</li>
          <li>Tipo y número de documento: para identificarte al retirar un reloj reservado.</li>
          <li>Condición fiscal, CUIT/CUIL/CDI y domicilio de facturación: para emitir la factura cuando corresponda.</li>
          <li>Contraseña: se guarda cifrada (hash); nadie del local puede verla.</li>
        </ul>
        <p>
          No pedimos datos sensibles, fecha de nacimiento ni imágenes de documentos, y
          no procesamos pagos ni datos de tarjeta online.
        </p>

        <h2 className="h5">Comunicaciones comerciales</h2>
        <p>
          Solo te enviamos novedades y promociones si lo aceptaste expresamente. Es
          opcional y podés cambiarlo cuando quieras desde <Link to="/mi-cuenta" className="link-underline">Mi cuenta</Link>.
        </p>

        <h2 className="h5">Con quién se comparten</h2>
        <p>
          [Completar: por ejemplo, organismos fiscales cuando corresponda facturar, o
          proveedores de hosting. No se venden ni ceden datos para otros fines.]
        </p>

        <h2 className="h5">Tus derechos</h2>
        <p>
          Podés acceder, rectificar y actualizar tus datos desde Mi cuenta, y solicitar
          su supresión escribiéndonos desde <Link to="/contacto" className="link-underline">Contacto</Link>.
          [Revisar con asesor legal: plazos, procedimiento y referencias a la Ley 25.326
          de Protección de los Datos Personales y a la autoridad de control.]
        </p>

        <h2 className="h5">Conservación y seguridad</h2>
        <p>
          [Completar: plazo de conservación de los datos de cuentas y reservas, y
          medidas de seguridad aplicadas.]
        </p>
      </div>
    </section>
  )
}
