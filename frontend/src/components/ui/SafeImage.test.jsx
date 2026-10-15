import { describe, expect, it } from 'vitest'
import { fireEvent, render, screen } from '@testing-library/react'
import SafeImage from './SafeImage'

describe('SafeImage', () => {
  it('muestra la imagen mientras carga bien', () => {
    render(<SafeImage src="/uploads/1.png" alt="Reloj" fallback={<span>sin foto</span>} />)
    expect(screen.getByAltText('Reloj')).toBeInTheDocument()
  })
  it('muestra el fallback si la imagen está rota', () => {
    render(<SafeImage src="/uploads/borrada.png" alt="Reloj" fallback={<span>sin foto</span>} />)
    fireEvent.error(screen.getByAltText('Reloj'))
    expect(screen.queryByAltText('Reloj')).not.toBeInTheDocument()
    expect(screen.getByText('sin foto')).toBeInTheDocument()
  })
  it('muestra el fallback si no hay imagen', () => {
    render(<SafeImage src="" alt="Reloj" fallback={<span>sin foto</span>} />)
    expect(screen.getByText('sin foto')).toBeInTheDocument()
  })
})
