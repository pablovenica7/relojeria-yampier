/**
 * Iconos SVG inline, monocromos (heredan currentColor). Se evita sumar una
 * librería de iconos para unos pocos glifos. Son decorativos: el texto
 * accesible lo pone siempre el elemento que los contiene (aria-label o texto).
 */
const base = {
  width: 20,
  height: 20,
  viewBox: '0 0 24 24',
  fill: 'none',
  stroke: 'currentColor',
  strokeWidth: 1.5,
  strokeLinecap: 'round',
  strokeLinejoin: 'round',
  'aria-hidden': true,
  focusable: 'false',
}

export function UserIcon(props) {
  return (
    <svg {...base} {...props}>
      <circle cx="12" cy="8" r="4" />
      <path d="M4 21c0-4.2 3.6-7 8-7s8 2.8 8 7" />
    </svg>
  )
}

export function WhatsAppIcon(props) {
  return (
    <svg {...base} {...props}>
      <path d="M3.5 20.5l1.3-4.2A8.5 8.5 0 1 1 8 19.3z" />
      <path d="M9 8.5c0 3.3 3.2 6.5 6.5 6.5l1-1.6-2.2-1-1 .9c-1-.4-2.2-1.6-2.6-2.6l.9-1-1-2.2z" />
    </svg>
  )
}

export function InstagramIcon(props) {
  return (
    <svg {...base} {...props}>
      <rect x="3.5" y="3.5" width="17" height="17" rx="5" />
      <circle cx="12" cy="12" r="4" />
      <circle cx="17.2" cy="6.8" r="0.6" fill="currentColor" />
    </svg>
  )
}

export function FacebookIcon(props) {
  return (
    <svg {...base} {...props}>
      <path d="M14 21v-7.5h2.6l.4-3H14V8.7c0-.9.3-1.5 1.6-1.5H17V4.5c-.3 0-1.2-.1-2.3-.1-2.3 0-3.7 1.4-3.7 3.9v2.2H8.5v3H11V21" />
    </svg>
  )
}
