import { readdirSync, readFileSync } from 'node:fs'
import { dirname, join, relative, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const raiz = fileURLToPath(new URL('../apps/web/src/', import.meta.url))
const errores = []
function recorrer(directorio) {
  for (const entrada of readdirSync(directorio, { withFileTypes: true })) {
    const ruta = join(directorio, entrada.name)
    if (entrada.isDirectory()) {
      if (entrada.name !== 'generado') recorrer(ruta)
      continue
    }
    if (!/\.[cm]?[jt]sx?$/.test(ruta)) continue
    const texto = readFileSync(ruta, 'utf8')
    const nombre = relative(raiz, ruta)
    if (/\b(?:localStorage|sessionStorage)\b/.test(texto))
      errores.push(`${nombre}: almacenamiento de sesión prohibido`)
    if (/\bfetch\s*\(/.test(texto) && !ruta.endsWith('.api.ts'))
      errores.push(`${nombre}: fetch solo se permite en *.api.ts`)
    if (ruta.replaceAll('\\', '/').includes('/componentes/')) {
      const normalizada = ruta.replaceAll('\\', '/')
      const propio = normalizada.split('/componentes/')[1]?.split('/')[0]
      for (const coincidencia of texto.matchAll(
        /(?:from\s+|import\s*)['"]([^'"]+)['"]/g,
      )) {
        const importacion = coincidencia[1]
        const destino = importacion.startsWith('.')
          ? resolve(dirname(ruta), importacion)
          : importacion.startsWith('@/')
            ? resolve(raiz, importacion.slice(2))
            : ''
        const ajeno = destino
          .replaceAll('\\', '/')
          .split('/componentes/')[1]
          ?.split('/')[0]
        if (ajeno && ajeno !== propio && ajeno !== 'compartido')
          errores.push(`${nombre}: importa otro microcomponente (${ajeno})`)
      }
    }
  }
}
recorrer(raiz)
if (errores.length) {
  console.error(errores.join('\n'))
  process.exit(1)
}
console.log('Límites de microcomponentes y sesión: correctos')
