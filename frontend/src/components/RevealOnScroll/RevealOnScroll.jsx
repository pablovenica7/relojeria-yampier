import { motion, useReducedMotion } from 'motion/react'

// Envoltorio reutilizable: aparece con un leve desplazamiento cuando entra en pantalla.
// Se desactiva automáticamente si el usuario prefiere menos movimiento.
export default function RevealOnScroll({ children, delay = 0, y = 18, className = '' }) {
  const reduceMotion = useReducedMotion()

  if (reduceMotion) {
    return <div className={className}>{children}</div>
  }

  return (
    <motion.div
      className={className}
      initial={{ opacity: 0, y }}
      whileInView={{ opacity: 1, y: 0 }}
      viewport={{ once: true, amount: 0.2 }}
      transition={{ duration: 0.6, delay, ease: [0.16, 1, 0.3, 1] }}
    >
      {children}
    </motion.div>
  )
}
