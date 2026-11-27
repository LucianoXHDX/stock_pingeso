import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: { proxy: { '/api': { target: 'http://127.0.0.1:5121', changeOrigin: false } } },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/pruebas/preparar.ts'],
    coverage: {
      provider: 'v8',
      reporter: ['text', 'lcov'],
      include: ['src/componentes/**/*.{ts,tsx}'],
      exclude: ['**/*.test.*'],
    },
  },
})
