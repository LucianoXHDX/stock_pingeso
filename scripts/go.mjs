import { spawnSync } from 'node:child_process'

const resultado = spawnSync('go', process.argv.slice(2), {
  cwd: new URL('../apps/api/', import.meta.url),
  stdio: 'inherit',
  shell: false,
})
if (resultado.error) console.error(resultado.error.message)
process.exit(resultado.status ?? 1)
