import { spawnSync } from 'node:child_process'
import { mkdirSync, mkdtempSync, readdirSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const raiz = fileURLToPath(new URL('../', import.meta.url))
const temporal = mkdtempSync(join(tmpdir(), 'stock-e2e-'))
const cobertura = join(temporal, 'cobertura')
mkdirSync(cobertura)
const binario = join(temporal, process.platform === 'win32' ? 'stock.exe' : 'stock')
const entorno = { ...process.env, STOCK_API_BINARY: binario, GOCOVERDIR: cobertura }
function ejecutar(argumentos, opciones = {}) {
  const resultado = spawnSync('go', argumentos, {
    cwd: resolve(raiz, 'apps/api'),
    stdio: 'inherit',
    env: entorno,
    shell: false,
    ...opciones,
  })
  if (resultado.error) throw resultado.error
  return resultado.status ?? 1
}
let estado = 1
try {
  estado = ejecutar([
    'build',
    '-cover',
    '-coverpkg=./internal/...,./cmd/api',
    '-buildvcs=false',
    '-trimpath',
    '-o',
    binario,
    './cmd/api',
  ])
  if (estado === 0) {
    estado = ejecutar(['test', '-tags=e2e', '-count=1', '-timeout=5m', '-v', './e2e/...'])
    if (readdirSync(cobertura).length === 0) {
      console.error('El binario no emitió cobertura: verifique su cierre limpio.')
      estado = 1
    } else {
      const estadoCobertura = ejecutar([
        'tool',
        'covdata',
        'textfmt',
        '-i',
        cobertura,
        '-o',
        resolve(raiz, 'coverage.out'),
      ])
      if (estadoCobertura !== 0) estado = estadoCobertura
    }
  }
} finally {
  rmSync(temporal, { recursive: true, force: true })
}
process.exitCode = estado
