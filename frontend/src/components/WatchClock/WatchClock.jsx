import { memo, useEffect, useState } from 'react'

// Esfera analógica en SVG, monocromática. live = hora real; si no, marca la hora fija recibida.
function WatchClock({ live = false, hour = 10, minute = 10, label = 'Reloj' }) {
  const [now, setNow] = useState(() => new Date())
  useEffect(() => {
    if (!live) return
    const t = setInterval(() => setNow(new Date()), 1000)
    return () => clearInterval(t)
  }, [live])

  const h = live ? now.getHours() % 12 : hour % 12
  const m = live ? now.getMinutes() : minute
  const s = live ? now.getSeconds() : null
  const hourAngle = h * 30 + m * 0.5
  const minAngle = m * 6 + (s ?? 0) * 0.1

  return (
    <svg viewBox="0 0 200 200" role="img" aria-label={label} className="watch-svg">
      <defs>
        <radialGradient id="dial" cx="50%" cy="38%" r="72%">
          <stop offset="0%" stopColor="#262626" />
          <stop offset="100%" stopColor="#0a0a0a" />
        </radialGradient>
      </defs>
      <circle cx="100" cy="100" r="97" fill="none" stroke="#404040" strokeWidth="2.5" />
      <circle cx="100" cy="100" r="93" fill="url(#dial)" stroke="#f2f2f2" strokeWidth="1" />
      {Array.from({ length: 60 }, (_, i) => (
        <line key={i} x1="100" x2="100" y1="10" y2={i % 5 === 0 ? 20 : 15}
          stroke="#f2f2f2" strokeWidth={i % 5 === 0 ? 1.6 : 0.5}
          opacity={i % 5 === 0 ? 1 : 0.4}
          transform={`rotate(${i * 6} 100 100)`} />
      ))}
      <text x="100" y="58" textAnchor="middle" fill="#f2f2f2" fontSize="7" letterSpacing="2.5" fontFamily="Jost, sans-serif">YAMPIER</text>
      <line x1="100" y1="100" x2="100" y2="54" stroke="#f2f2f2" strokeWidth="3" strokeLinecap="round" transform={`rotate(${hourAngle} 100 100)`} />
      <line x1="100" y1="100" x2="100" y2="32" stroke="#f2f2f2" strokeWidth="1.8" strokeLinecap="round" transform={`rotate(${minAngle} 100 100)`} />
      {live && <line x1="100" y1="112" x2="100" y2="24" stroke="#a3a3a3" strokeWidth="0.8" transform={`rotate(${s * 6} 100 100)`} />}
      <circle cx="100" cy="100" r="3" fill="#f2f2f2" />
    </svg>
  )
}

export default memo(WatchClock)
