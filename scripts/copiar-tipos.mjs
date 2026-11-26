import { copyFileSync, mkdirSync } from 'node:fs'

const destino = new URL('../apps/web/dist-paquete/generado/', import.meta.url)
mkdirSync(destino, { recursive: true })
copyFileSync(
  new URL('../apps/web/src/generado/api.d.ts', import.meta.url),
  new URL('api.d.ts', destino),
)
