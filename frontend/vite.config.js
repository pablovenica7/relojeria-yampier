import { defineConfig, loadEnv } from 'vite'
import react from '@vitejs/plugin-react'

// En un build de hosting (Render, Vercel o CI) la URL de la API es obligatoria
// y debe ser HTTPS: sin esto el sitio publicado llamaría a localhost o
// tendría "mixed content". Los builds locales y de docker-compose no cambian.
function checkProductionApiUrl(mode) {
  const hosted = process.env.RENDER || process.env.VERCEL || process.env.CI
  if (!hosted || mode !== 'production') return
  const apiUrl = loadEnv(mode, process.cwd(), 'VITE_').VITE_API_URL || ''
  if (!/^https:\/\/.+\/api$/.test(apiUrl)) {
    throw new Error(
      `VITE_API_URL inválida para producción (${apiUrl || 'vacía'}). ` +
        'Debe ser la URL HTTPS del backend terminada en /api, ej. https://api.relojeriayampier.com/api',
    )
  }
}

export default defineConfig(({ mode }) => {
  checkProductionApiUrl(mode)
  return {
    plugins: [react()],
    test: {
      environment: 'jsdom',
      globals: true,
      setupFiles: './src/test/setup.js',
    },
  }
})
