import { useId } from 'react'

/**
 * Campo de formulario reutilizable: label + input/textarea + mensaje de error.
 * Conecta label, input y error por id/aria-describedby para lectores de pantalla,
 * y marca aria-invalid cuando corresponde, sin depender de estilos de navegador.
 */
export default function FormField({
  label,
  type = 'text',
  as = 'input',
  error,
  hint,
  required,
  rows = 4,
  className = '',
  ...props
}) {
  const autoId = useId()
  const id = props.id || autoId
  const errorId = `${id}-error`
  const hintId = `${id}-hint`
  const describedBy = [error ? errorId : null, hint ? hintId : null].filter(Boolean).join(' ') || undefined

  const Field = as === 'textarea' ? 'textarea' : as === 'select' ? 'select' : 'input'
  const fieldClass = as === 'textarea' ? 'field-textarea' : as === 'select' ? 'field-select' : 'field-input'

  return (
    <div className={`field ${className}`}>
      <label htmlFor={id} className="field-label">
        {label}{required && <span aria-hidden="true"> *</span>}
      </label>
      <Field
        id={id}
        type={as === 'input' ? type : undefined}
        rows={as === 'textarea' ? rows : undefined}
        className={fieldClass}
        aria-invalid={error ? 'true' : 'false'}
        aria-describedby={describedBy}
        aria-required={required || undefined}
        {...props}
      />
      {hint && !error && <span id={hintId} className="field-hint">{hint}</span>}
      <span id={errorId} className="field-error" role={error ? 'alert' : undefined}>{error || ''}</span>
    </div>
  )
}
