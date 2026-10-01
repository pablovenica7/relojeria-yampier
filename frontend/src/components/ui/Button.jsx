import { forwardRef } from 'react'

/**
 * Botón base del sitio. Variantes: primary (blanco sólido), secondary (outline),
 * ghost (texto) y danger (acciones destructivas). Maneja su propio estado de
 * carga para que las páginas no dupliquen el spinner en cada formulario.
 */
const Button = forwardRef(function Button(
  { variant = 'primary', size = 'md', loading = false, disabled, children, className = '', as: Comp = 'button', ...props },
  ref
) {
  const classes = [
    'btn',
    `btn-${variant}`,
    size === 'sm' ? 'btn-sm' : '',
    className,
  ].filter(Boolean).join(' ')

  return (
    <Comp ref={ref} className={classes} disabled={disabled || loading} aria-busy={loading} {...props}>
      {loading && <span className="spinner" aria-hidden="true" />}
      {children}
    </Comp>
  )
})

export default Button
