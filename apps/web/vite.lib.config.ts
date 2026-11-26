import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  build: {
    outDir: 'dist-paquete',
    lib: {
      entry: 'src/biblioteca.ts',
      name: 'StockUI',
      formats: ['es'],
      fileName: 'stock-ui',
      cssFileName: 'stock-ui',
    },
    rolldownOptions: { external: ['react', 'react-dom', 'react/jsx-runtime'] },
  },
})
