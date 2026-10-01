import { Link } from 'react-router-dom'
import FormField from '../ui/FormField'
import {
  DOCUMENT_TYPES, PROVINCES, TAX_CONDITIONS, TAX_ID_TYPES, requiresTaxId,
} from '../../utils/customerOptions'

function SelectField({ id, label, value, onChange, options, error, required, placeholder }) {
  const errorId = `${id}-error`
  return (
    <div className="field">
      <label htmlFor={id} className="field-label">{label}{required && <span aria-hidden="true"> *</span>}</label>
      <select
        id={id} className="field-select" value={value} onChange={onChange}
        aria-invalid={error ? 'true' : 'false'} aria-describedby={errorId} aria-required={required || undefined}
      >
        {placeholder && <option value="">{placeholder}</option>}
        {options.map(o => (typeof o === 'string'
          ? <option key={o} value={o}>{o}</option>
          : <option key={o.value} value={o.value}>{o.label}</option>))}
      </select>
      <span id={errorId} className="field-error" role={error ? 'alert' : undefined}>{error || ''}</span>
    </div>
  )
}

export function CheckField({ id, checked, onChange, error, children }) {
  const errorId = `${id}-error`
  return (
    <div className="field-check-wrap">
      <div className="field-check">
        <input
          id={id} type="checkbox" checked={checked} onChange={onChange}
          aria-invalid={error ? 'true' : 'false'} aria-describedby={errorId}
        />
        <label htmlFor={id}>{children}</label>
      </div>
      <span id={errorId} className="field-error" role={error ? 'alert' : undefined}>{error || ''}</span>
    </div>
  )
}

/**
 * Campos de la cuenta de cliente, compartidos por "Crear cuenta" y "Mi cuenta".
 * Solo se piden datos necesarios para la cuenta, las reservas y una futura
 * factura (sin fecha de nacimiento, género ni imágenes de documentos).
 */
export default function CustomerFields({ form, setForm, errors, register = false }) {
  const set = k => e => setForm(f => ({ ...f, [k]: e.target.value }))
  const setCheck = k => e => setForm(f => ({ ...f, [k]: e.target.checked }))
  const setAddr = k => e => setForm(f => ({ ...f, billingAddress: { ...f.billingAddress, [k]: e.target.value } }))
  const needsTaxId = requiresTaxId(form.taxCondition)
  const a = form.billingAddress

  return (
    <>
      <fieldset className="form-section">
        <legend>Datos personales</legend>
        <div className="form-grid">
          <FormField label="Nombre" required autoComplete="given-name" value={form.firstName} onChange={set('firstName')} error={errors.firstName} />
          <FormField label="Apellido" required autoComplete="family-name" value={form.lastName} onChange={set('lastName')} error={errors.lastName} />
          <FormField label="Email" type="email" required autoComplete="email" value={form.email} onChange={set('email')} error={errors.email} />
          <FormField
            label="Teléfono / WhatsApp" type="tel" required autoComplete="tel" hint="Con código de área, ej: 351 123 4567"
            value={form.phone} onChange={set('phone')} error={errors.phone}
          />
        </div>
      </fieldset>

      {register && (
        <fieldset className="form-section">
          <legend>Contraseña</legend>
          <div className="form-grid">
            <FormField
              label="Contraseña" type="password" required autoComplete="new-password" hint="Mínimo 8 caracteres, con letras y números"
              value={form.password} onChange={set('password')} error={errors.password}
            />
            <FormField
              label="Repetir contraseña" type="password" required autoComplete="new-password"
              value={form.passwordConfirm} onChange={set('passwordConfirm')} error={errors.passwordConfirm}
            />
          </div>
        </fieldset>
      )}

      <fieldset className="form-section">
        <legend>Identificación y datos fiscales</legend>
        <p className="form-section-hint">Los usamos para identificarte al retirar el reloj y, si lo necesitás, para la factura.</p>
        <div className="form-grid">
          <SelectField id="documentType" label="Tipo de documento" required value={form.documentType} onChange={set('documentType')} options={DOCUMENT_TYPES} error={errors.documentType} />
          <FormField label="Número de documento" required autoComplete="off" value={form.documentNumber} onChange={set('documentNumber')} error={errors.documentNumber} />
          <SelectField id="taxCondition" label="Condición fiscal" required value={form.taxCondition} onChange={set('taxCondition')} options={TAX_CONDITIONS} error={errors.taxCondition} />
          <div />
          <SelectField
            id="taxIdType" label={needsTaxId ? 'Tipo de identificación fiscal' : 'Tipo de identificación fiscal (opcional)'}
            required={needsTaxId} value={form.taxIdType} onChange={set('taxIdType')} options={TAX_ID_TYPES}
            placeholder="Elegí…" error={errors.taxIdType}
          />
          <FormField
            label={needsTaxId ? 'CUIT / CUIL / CDI' : 'CUIT / CUIL / CDI (opcional)'} required={needsTaxId}
            inputMode="numeric" autoComplete="off" hint="11 dígitos" value={form.taxId} onChange={set('taxId')} error={errors.taxId}
          />
        </div>
      </fieldset>

      <fieldset className="form-section">
        <legend>Domicilio de facturación <span className="text-faint-token">(opcional)</span></legend>
        <p className="form-section-hint">Completalo solo si vas a necesitar factura con domicilio.</p>
        <div className="form-grid">
          <FormField label="Calle" autoComplete="address-line1" value={a.street} onChange={setAddr('street')} error={errors.street} />
          <FormField label="Número" value={a.number} onChange={setAddr('number')} error={errors.number} />
          <FormField label="Piso (opcional)" value={a.floor} onChange={setAddr('floor')} />
          <FormField label="Departamento (opcional)" value={a.apartment} onChange={setAddr('apartment')} />
          <FormField label="Código postal" autoComplete="postal-code" value={a.postalCode} onChange={setAddr('postalCode')} error={errors.postalCode} />
          <FormField label="Ciudad / Localidad" autoComplete="address-level2" value={a.city} onChange={setAddr('city')} error={errors.city} />
          <SelectField id="province" label="Provincia" value={a.province} onChange={setAddr('province')} options={PROVINCES} placeholder="Elegí…" error={errors.province} />
          <FormField label="País" value={a.country} readOnly hint="Por ahora solo domicilios en Argentina" />
        </div>
      </fieldset>

      <fieldset className="form-section">
        <legend>Privacidad</legend>
        {register && (
          <CheckField id="privacyAccepted" checked={form.privacyAccepted} onChange={setCheck('privacyAccepted')} error={errors.privacyAccepted}>
            He leído la <Link to="/privacidad" target="_blank" rel="noopener">Política de Privacidad</Link> *
          </CheckField>
        )}
        <CheckField id="marketingConsent" checked={form.marketingConsent} onChange={setCheck('marketingConsent')}>
          Quiero recibir novedades y promociones (opcional, lo podés cambiar cuando quieras)
        </CheckField>
      </fieldset>
    </>
  )
}
