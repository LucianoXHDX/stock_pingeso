import { spawnSync } from 'node:child_process'
import { mkdirSync, writeFileSync } from 'node:fs'

const raiz = new URL('../', import.meta.url)
const contrato = spawnSync('go', ['run', './cmd/contrato'], {
  cwd: new URL('apps/api/', raiz),
  encoding: 'utf8',
  shell: false,
})
if (contrato.status !== 0) {
  console.error(contrato.stderr || contrato.error?.message)
  process.exit(contrato.status ?? 1)
}
mkdirSync(new URL('contracts/', raiz), { recursive: true })
writeFileSync(new URL('contracts/openapi.yaml', raiz), contrato.stdout, 'utf8')
const tipos = spawnSync(
  process.platform === 'win32' ? 'pnpm.cmd' : 'pnpm',
  [
    '--filter',
    '@pingeso-ejecu/stock-ui',
    'exec',
    'openapi-typescript',
    '../../contracts/openapi.yaml',
    '-o',
    'src/generado/api.d.ts',
  ],
  { cwd: raiz, stdio: 'inherit', shell: process.platform === 'win32' },
)
if (tipos.error) console.error(tipos.error.message)
process.exit(tipos.status ?? 1)
