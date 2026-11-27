import crearCliente from 'openapi-fetch'
import type { paths } from '../../generado/api'
import { exigirRespuesta, type NuevoMovimiento } from '../compartido/tipos'

export function crearApiMovimiento(apiBaseUrl: string) {
  const cliente = crearCliente<paths>({ baseUrl: apiBaseUrl, credentials: 'include' })
  return {
    async registrar(datos: NuevoMovimiento, clave: string) {
      return exigirRespuesta(
        await cliente.POST('/api/v1/movimientos', {
          params: { header: { 'Idempotency-Key': clave } },
          body: datos,
          signal: AbortSignal.timeout(5000),
        }),
      )
    },
  }
}
