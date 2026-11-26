import { spawnSync } from 'node:child_process'
import {
  cpSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'

// La mutación se aplica a una copia temporal: el código del proyecto permanece intacto.
const api = fileURLToPath(new URL('../apps/api/', import.meta.url))
const temporal = mkdtempSync(join(tmpdir(), 'stock-sensibilidad-'))
const copia = join(temporal, 'api')
const cobertura = join(temporal, 'cobertura')
mkdirSync(cobertura)
const binario = join(temporal, process.platform === 'win32' ? 'stock.exe' : 'stock')
try {
  cpSync(api, copia, { recursive: true })
  const archivo = join(copia, 'internal', 'service', 'stock.go')
  const fuente = readFileSync(archivo, 'utf8')
  const proteccion = 'if *solicitud.VersionEsperada != actual.Version {'
  if (!fuente.includes(proteccion))
    throw new Error('No se encontró la protección de versión.')
  writeFileSync(archivo, fuente.replace(proteccion, 'if false {'), 'utf8')
  const compilacion = spawnSync('go', ['build', '-cover', '-o', binario, './cmd/api'], {
    cwd: copia,
    stdio: 'inherit',
    shell: false,
  })
  if (compilacion.error) throw compilacion.error
  if (compilacion.status !== 0) throw new Error('No fue posible compilar la mutación.')
  const prueba = spawnSync(
    'go',
    [
      'test',
      '-tags=e2e',
      '-count=1',
      '-timeout=3m',
      '-run=^TestFlujoStock$',
      './e2e/...',
    ],
    {
      cwd: api,
      encoding: 'utf8',
      shell: false,
      env: { ...process.env, STOCK_API_BINARY: binario, GOCOVERDIR: cobertura },
    },
  )
  if (prueba.error) throw prueba.error
  const salida = (prueba.stdout ?? '') + (prueba.stderr ?? '')
  if (prueba.status === 0 || !salida.includes('estado recibido 201; esperado 409')) {
    console.error(salida)
    throw new Error(
      'El E2E no detectó la protección de versión eliminada como se esperaba.',
    )
  }
  console.log(
    'Sensibilidad confirmada: sin control de versión, el ajuste obsoleto devuelve 201 y el E2E falla esperando 409.',
  )
  console.log(
    'La mutación se realizó en una copia temporal; el código original conserva su protección.',
  )
} finally {
  rmSync(temporal, { recursive: true, force: true })
}
