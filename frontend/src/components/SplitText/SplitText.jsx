import { motion, useReducedMotion } from 'motion/react'

// Título que revela palabra por palabra. Si el usuario prefiere menos
// movimiento, se muestra directamente sin animar.
export default function SplitText({ text, as = 'h1', className = '', delay = 0 }) {
  const reduceMotion = useReducedMotion()
  const Tag = motion[as]
  const words = text.split(' ')

  if (reduceMotion) {
    const Plain = as
    return <Plain className={className}>{text}</Plain>
  }

  const container = {
    hidden: {},
    show: { transition: { staggerChildren: 0.06, delayChildren: delay } },
  }
  const word = {
    hidden: { opacity: 0, y: '0.5em' },
    show: { opacity: 1, y: 0, transition: { duration: 0.55, ease: [0.16, 1, 0.3, 1] } },
  }

  return (
    <Tag className={className} variants={container} initial="hidden" animate="show" style={{ overflow: 'hidden' }}>
      {words.map((w, i) => (
        <span key={i} style={{ display: 'inline-block', overflow: 'hidden', marginRight: '0.28em' }}>
          <motion.span variants={word} style={{ display: 'inline-block' }}>{w}</motion.span>
        </span>
      ))}
    </Tag>
  )
}
