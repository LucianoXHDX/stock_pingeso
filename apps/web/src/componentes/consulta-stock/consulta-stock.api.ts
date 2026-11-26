import crearCliente from 'openapi-fetch'
import type { paths } from '../../generado/api'
import { exigirRespuesta } from '../compartido/tipos'

export function crearApiConsulta(apiBaseUrl: string) {
  const cliente = crearCliente<paths>({ baseUrl: apiBaseUrl, credentials: 'include' })
  return {
    async listar(desplazamiento = 0) {
      return exigirRespuesta(
        await cliente.GET('/api/v1/existencias', {
          params: { query: { limite: 10, desplazamiento } },
          signal: AbortSignal.timeout(5000),
        }),
      )
    },
    async consultar(productoId: string) {
      return exigirRespuesta(
        await cliente.GET('/api/v1/existencias/{producto_id}', {
          params: { path: { producto_id: productoId } },
          signal: AbortSignal.timeout(5000),
        }),
      )
    },
  }
}
