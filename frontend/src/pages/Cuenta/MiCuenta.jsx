import { useState } from 'react'
import { Link } from 'react-router-dom'
import { useCustomerAuth } from '../../context/CustomerAuthContext'
import { changePassword, updateMe } from '../../services/customerService'
import CustomerFields from '../../components/Customer/CustomerFields'
import FormField from '../../components/ui/FormField'
import Button from '../../components/ui/Button'
import { customerToForm, formToCustomerPayload, validateCustomerForm } from '../../utils/customerOptions'
import swal from '../../utils/swal'

function PasswordForm() {
  const [current, setCurrent] = useState('')
  const [next, setNext] = useState('')
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)

  const submit = async (e) => {
    e.preventDefault()
    setError('')
    if (next.length < 8 || !/\p{L}/u.test(next) || !/\d/.test(next)) {
      setError('La nueva contraseña necesita 8 caracteres o más, con letras y números')
      return
    }
    setSaving(true)
    try {
      await changePassword(current, next)
      setCurrent('')
      setNext('')
      swal.fire({ icon: 'success', title: 'Contraseña actualizada', text: 'Cerramos las sesiones abiertas en otros dispositivos.' })
    } catch (err) {
      setError(err.message)
    } finally {
      setSaving(false)
    }
  }

  return (
    <form onSubmit={submit} noValidate className="form-section d-grid gap-4">
      <h2 className="detail-subtitle mb-0">Cambiar contraseña</h2>
      <div className="form-grid">
        <FormField label="Contraseña actual" type="password" autoComplete="current-password" value={current} onChange={e => setCurrent(e.target.value)} />
        <FormField label="Nueva contraseña" type="password" autoComplete="new-password" value={next} onChange={e => setNext(e.target.value)} error={error} />
      </div>
      <div><Button type="submit" variant="secondary" size="sm" loading={saving}>Cambiar contraseña</Button></div>
    </form>
  )
}

export default function MiCuenta() {
  const { customer, setCustomer } = useCustomerAuth()
  const [form, setForm] = useState(() => customerToForm(customer))
  const [errors, setErrors] = useState({})
  const [serverError, setServerError] = useState('')
  const [saving, setSaving] = useState(false)

  const submit = async (e) => {
    e.preventDefault()
    setServerError('')
    const next = validateCustomerForm(form)
    setErrors(next)
    if (Object.keys(next).length > 0) return
    setSaving(true)
    try {
      const updated = await updateMe(formToCustomerPayload(form))
      setCustomer(updated)
      setForm(customerToForm(updated))
      swal.fire({ icon: 'success', title: 'Datos actualizados', timer: 1400, showConfirmButton: false })
    } catch (err) {
      setServerError(err.message)
    } finally {
      setSaving(false)
    }
  }

  return (
    <section className="section">
      <div className="container account-container">
        <span className="eyebrow">Mi cuenta</span>
        <h1 className="h2 mb-3">Hola, {customer.firstName}</h1>
        <p className="mb-6">
          Revisá y corregí tus datos cuando lo necesites. <Link to="/mi-cuenta/reservas" className="link-underline">Ver mis reservas</Link>
        </p>
        <form onSubmit={submit} noValidate className="d-grid gap-5">
          <CustomerFields form={form} setForm={setForm} errors={errors} />
          {serverError && <p className="form-alert" role="alert">{serverError}</p>}
          {Object.keys(errors).length > 0 && <p className="form-alert" role="alert">Revisá los campos marcados.</p>}
          <div><Button type="submit" variant="primary" loading={saving}>{saving ? 'Guardando' : 'Guardar cambios'}</Button></div>
        </form>
        <hr className="divider my-6" />
        <PasswordForm />
        <p className="text-faint-token mt-6 mb-0">
          Para dar de baja tu cuenta o solicitar acceso a tus datos, escribinos desde{' '}
          <Link to="/contacto" className="link-underline">Contacto</Link>. Más información en la{' '}
          <Link to="/privacidad" className="link-underline">Política de Privacidad</Link>.
        </p>
      </div>
    </section>
  )
}
