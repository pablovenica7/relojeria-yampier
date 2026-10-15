import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { CustomerAuthProvider } from '../../context/CustomerAuthContext'
import Navbar from './Navbar'

// matchMedia no existe en jsdom: se simula para poder disparar el cambio a desktop.
let mqListeners = []
beforeEach(() => {
  mqListeners = []
  window.matchMedia = vi.fn(() => ({
    matches: false,
    addEventListener: (_, fn) => mqListeners.push(fn),
    removeEventListener: (_, fn) => { mqListeners = mqListeners.filter(l => l !== fn) },
  }))
  window.scrollTo = vi.fn()
  Object.defineProperty(window, 'scrollY', { value: 400, configurable: true })
})
afterEach(() => {
  document.body.removeAttribute('style')
})

const renderNavbar = () => render(
  <MemoryRouter><CustomerAuthProvider><Navbar /></CustomerAuthProvider></MemoryRouter>
)
const toggle = () => screen.getByRole('button', { name: /menú/i })

describe('Navbar: bloqueo de scroll del menú mobile', () => {
  it('fija el body al abrir y vuelve a la misma posición al cerrar', () => {
    renderNavbar()
    fireEvent.click(toggle())
    expect(document.body.style.position).toBe('fixed')
    expect(document.body.style.top).toBe('-400px')
    expect(toggle()).toHaveAttribute('aria-expanded', 'true')

    fireEvent.click(toggle())
    expect(document.body.getAttribute('style')).toBe('')
    expect(window.scrollTo).toHaveBeenCalledWith(0, 400)
  })

  it('libera el body al cerrar con Escape', () => {
    renderNavbar()
    fireEvent.click(toggle())
    fireEvent.keyDown(document, { key: 'Escape' })
    expect(document.body.getAttribute('style')).toBe('')
  })

  it('libera el body si el componente se desmonta con el menú abierto', () => {
    const { unmount } = renderNavbar()
    fireEvent.click(toggle())
    unmount()
    expect(document.body.getAttribute('style')).toBe('')
  })

  it('cierra el menú y libera el body si la pantalla pasa a desktop', () => {
    renderNavbar()
    fireEvent.click(toggle())
    act(() => mqListeners.forEach(fn => fn({ matches: true })))
    expect(toggle()).toHaveAttribute('aria-expanded', 'false')
    expect(document.body.getAttribute('style')).toBe('')
  })
})
