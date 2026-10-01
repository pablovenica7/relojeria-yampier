import { useState } from 'react'
import { Link, Navigate, useNavigate, useSearchParams } from 'react-router-dom'
import { useCustomerAuth } from '../../context/CustomerAuthContext'
import CustomerFields from '../../components/Customer/CustomerFields'
import Button from '../../components/ui/Button'
import { emptyAddress, formToCustomerPayload, validateCustomerForm } from '../../utils/customerOptions'
import { safeReturnTo } from '../../utils/returnTo'

const initial = {
  firstName: '', lastName: '', email: '', phone: '', password: '', passwordConfirm: '',
  documentType: 'dni', documentNumber: '', taxCondition: 'consumer_final', taxIdType: '', taxId: '',
  billingAddress: { ...emptyAddress },
  // Nunca premarcadas: privacidad (obligatoria) y promociones (opcional).
  privacyAccepted: false, marketingConsent: false,
}

export default function CrearCuenta() {
  const [params] = useSearchParams()
  const returnTo = safeReturnTo(params.get('returnTo'))
  const navigate = useNavigate()
  const { register, isAuthenticated } = useCustomerAuth()
  const [form, setForm] = useState(initial)
  const [errors, setErrors] = useState({})
  const [serverError, setServerError] = useState('')
  const [saving, setSaving] = useState(false)

  if (isAuthenticated && !saving) return <Navigate to={returnTo} replace />

  const submit = async (e) => {
    e.preventDefault()
    setServerError('')
    const next = validateCustomerForm(form, { register: true })
    setErrors(next)
    if (Object.keys(next).length > 0) return
    setSaving(true)
    try {
      await register(formToCustomerPayload(form))
      navigate(returnTo, { replace: true })
    } catch (err) {
      setServerError(err.message)
      setSaving(false)
    }
  }

  return (
    <section className="section">
      <div className="container account-container">
        <span className="eyebrow">Clientes</span>
        <h1 className="h2 mb-3">Crear cuenta</h1>
        <p className="mb-6">
          Con tu cuenta podés solicitar reservas y seguir su estado. No hay pagos
          online: el pago final se realiza en nuestro local.
        </p>
        <form onSubmit={submit} noValidate className="d-grid gap-5">
          <CustomerFields form={form} setForm={setForm} errors={errors} register />
          {serverError && <p className="form-alert" role="alert">{serverError}</p>}
          {Object.keys(errors).length > 0 && <p className="form-alert" role="alert">Revisá los campos marcados.</p>}
          <div className="d-flex flex-wrap gap-3 align-items-center">
            <Button type="submit" variant="primary" loading={saving}>{saving ? 'Creando cuenta' : 'Crear cuenta'}</Button>
            <span>¿Ya tenés cuenta? <Link to={`/ingresar?returnTo=${encodeURIComponent(returnTo)}`} className="link-underline">Ingresar</Link></span>
          </div>
        </form>
      </div>
    </section>
  )
}
