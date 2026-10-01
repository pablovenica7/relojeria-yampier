// Opciones de cuenta de cliente. Las claves deben coincidir con
// backend/customers.go; las etiquetas son lo que ve el usuario.

export const TAX_CONDITIONS = [
  { value: 'consumer_final', label: 'Consumidor final' },
  { value: 'monotributo', label: 'Monotributista' },
  { value: 'responsable_inscripto', label: 'Responsable inscripto' },
  { value: 'exento', label: 'Exento' },
  { value: 'other', label: 'Otra' },
]

export const DOCUMENT_TYPES = [
  { value: 'dni', label: 'DNI' },
  { value: 'cuit', label: 'CUIT' },
  { value: 'cuil', label: 'CUIL' },
  { value: 'cdi', label: 'CDI' },
  { value: 'passport', label: 'Pasaporte' },
  { value: 'foreign_document', label: 'Documento extranjero' },
]

export const TAX_ID_TYPES = [
  { value: 'cuit', label: 'CUIT' },
  { value: 'cuil', label: 'CUIL' },
  { value: 'cdi', label: 'CDI' },
]

export const PROVINCES = [
  'Buenos Aires', 'Ciudad Autónoma de Buenos Aires', 'Catamarca', 'Chaco', 'Chubut',
  'Córdoba', 'Corrientes', 'Entre Ríos', 'Formosa', 'Jujuy', 'La Pampa', 'La Rioja',
  'Mendoza', 'Misiones', 'Neuquén', 'Río Negro', 'Salta', 'San Juan', 'San Luis',
  'Santa Cruz', 'Santa Fe', 'Santiago del Estero', 'Tierra del Fuego', 'Tucumán',
]

export const labelFrom = (list, value) => list.find(o => o.value === value)?.label || value || '—'

/** Estas condiciones siempre tienen CUIT/CUIL/CDI. */
export function requiresTaxId(taxCondition) {
  return ['monotributo', 'responsable_inscripto', 'exento'].includes(taxCondition)
}

export const onlyDigits = (s) => String(s ?? '').replace(/\D/g, '')

/** Dígito verificador de CUIT/CUIL/CDI (el backend vuelve a validarlo). */
export function isValidCuit(value) {
  const d = onlyDigits(value)
  if (d.length !== 11) return false
  const weights = [5, 4, 3, 2, 7, 6, 5, 4, 3, 2]
  const sum = weights.reduce((acc, w, i) => acc + Number(d[i]) * w, 0)
  let check = 11 - (sum % 11)
  if (check === 11) check = 0
  if (check === 10) return false
  return check === Number(d[10])
}

export const emptyAddress = {
  street: '', number: '', floor: '', apartment: '', postalCode: '', city: '', province: '', country: 'Argentina',
}

export function isAddressEmpty(a) {
  return !a || ['street', 'number', 'floor', 'apartment', 'postalCode', 'city', 'province'].every(k => !String(a[k] || '').trim())
}

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/
const NAME_RE = /^\p{L}[\p{L}'’ .-]*$/u

/**
 * Validación del formulario de cuenta (registro y Mi cuenta). Solo UX: el
 * backend aplica las mismas reglas. `register` exige contraseña y privacidad.
 */
export function validateCustomerForm(f, { register = false } = {}) {
  const e = {}
  if (!f.firstName.trim() || !NAME_RE.test(f.firstName.trim())) e.firstName = 'Ingresá tu nombre'
  if (!f.lastName.trim() || !NAME_RE.test(f.lastName.trim())) e.lastName = 'Ingresá tu apellido'
  if (!EMAIL_RE.test(f.email.trim())) e.email = 'Ingresá un email válido'
  const phoneDigits = onlyDigits(f.phone).length
  if (phoneDigits < 8 || phoneDigits > 15) e.phone = 'Ingresá un teléfono con código de área (ej: 351 123 4567)'
  if (register) {
    if ((f.password || '').length < 8 || !/\p{L}/u.test(f.password) || !/\d/.test(f.password)) {
      e.password = 'Mínimo 8 caracteres, con letras y números'
    } else if (f.password !== f.passwordConfirm) {
      e.passwordConfirm = 'Las contraseñas no coinciden'
    }
    if (!f.privacyAccepted) e.privacyAccepted = 'Tenés que leer y aceptar la Política de Privacidad'
  }

  const doc = f.documentNumber.trim()
  if (!f.documentType) e.documentType = 'Elegí un tipo de documento'
  else if (f.documentType === 'dni' && !/^\d{7,8}$/.test(onlyDigits(doc))) e.documentNumber = 'El DNI debe tener 7 u 8 dígitos'
  else if (['cuit', 'cuil', 'cdi'].includes(f.documentType) && !isValidCuit(doc)) e.documentNumber = 'Número inválido (11 dígitos)'
  else if (['passport', 'foreign_document'].includes(f.documentType) && !/^[A-Za-z0-9 .-]{5,24}$/.test(doc)) e.documentNumber = 'Número de documento inválido'

  if (!f.taxCondition) e.taxCondition = 'Elegí tu condición fiscal'
  if (f.taxId.trim()) {
    if (!f.taxIdType) e.taxIdType = 'Indicá si es CUIT, CUIL o CDI'
    if (!isValidCuit(f.taxId)) e.taxId = 'CUIT / CUIL / CDI inválido (11 dígitos)'
  } else if (requiresTaxId(f.taxCondition)) {
    e.taxId = 'Para esta condición fiscal ingresá tu CUIT / CUIL / CDI'
  }

  const a = f.billingAddress
  if (!isAddressEmpty(a)) {
    if (!a.street.trim()) e.street = 'Ingresá la calle'
    if (!a.number.trim()) e.number = 'Ingresá la altura (o S/N)'
    if (!a.city.trim()) e.city = 'Ingresá la ciudad o localidad'
    if (!a.province) e.province = 'Elegí la provincia'
    if (!/^([A-Za-z]\d{4}[A-Za-z]{3}|\d{4})$/.test(a.postalCode.trim())) e.postalCode = 'Código postal: 4 dígitos o CPA (ej: X5000ABC)'
  }
  return e
}

/** Formulario a partir del perfil que devuelve la API. */
export function customerToForm(c) {
  return {
    firstName: c.firstName || '', lastName: c.lastName || '', email: c.email || '', phone: c.phone || '',
    documentType: c.documentType || 'dni', documentNumber: c.documentNumber || '',
    taxCondition: c.taxCondition || 'consumer_final', taxIdType: c.taxIdType || '', taxId: c.taxId || '',
    billingAddress: { ...emptyAddress, ...(c.billingAddress || {}) },
    marketingConsent: Boolean(c.marketingConsent),
  }
}

/** Cuerpo para la API (registro o edición). Un domicilio vacío se envía como null. */
export function formToCustomerPayload(f) {
  const payload = {
    firstName: f.firstName.trim(), lastName: f.lastName.trim(), email: f.email.trim(), phone: f.phone.trim(),
    documentType: f.documentType, documentNumber: f.documentNumber.trim(),
    taxCondition: f.taxCondition, taxIdType: f.taxId.trim() ? f.taxIdType : '', taxId: f.taxId.trim(),
    billingAddress: isAddressEmpty(f.billingAddress) ? null : f.billingAddress,
    marketingConsent: Boolean(f.marketingConsent),
  }
  if (f.password !== undefined) payload.password = f.password
  if (f.privacyAccepted !== undefined) payload.privacyAccepted = f.privacyAccepted
  return payload
}
