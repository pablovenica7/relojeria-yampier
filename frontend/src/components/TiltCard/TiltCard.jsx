import { useRef } from 'react'
import { motion, useMotionValue, useSpring, useTransform, useReducedMotion } from 'motion/react'

// Inclinación 3D sutil siguiendo el mouse. Se desactiva si el usuario
// prefiere menos movimiento, y solo aplica en dispositivos con puntero fino.
export default function TiltCard({ children, className = '' }) {
  const ref = useRef(null)
  const reduceMotion = useReducedMotion()
  const x = useMotionValue(0.5)
  const y = useMotionValue(0.5)
  const rotateX = useSpring(useTransform(y, [0, 1], [4, -4]), { stiffness: 220, damping: 24 })
  const rotateY = useSpring(useTransform(x, [0, 1], [-4, 4]), { stiffness: 220, damping: 24 })

  if (reduceMotion) {
    return <div className={className}>{children}</div>
  }

  const onMouseMove = (e) => {
    const rect = ref.current.getBoundingClientRect()
    x.set((e.clientX - rect.left) / rect.width)
    y.set((e.clientY - rect.top) / rect.height)
  }
  const reset = () => { x.set(0.5); y.set(0.5) }

  return (
    <motion.div
      ref={ref}
      className={className}
      onMouseMove={onMouseMove}
      onMouseLeave={reset}
      style={{ rotateX, rotateY, transformPerspective: 800 }}
    >
      {children}
    </motion.div>
  )
}
