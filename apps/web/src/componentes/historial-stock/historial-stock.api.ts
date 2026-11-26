import crearCliente from 'openapi-fetch'
import type { paths } from '../../generado/api'
import { exigirRespuesta, type TipoMovimiento } from '../compartido/tipos'

export function crearApiHistorial(apiBaseUrl: string) {
  const cliente = crearCliente<paths>({ baseUrl: apiBaseUrl, credentials: 'include' })
  return {
    async listar(productoId: string, tipo: TipoMovimiento | '', desplazamiento: number) {
      return exigirRespuesta(
        await cliente.GET('/api/v1/movimientos', {
          params: {
            query: {
              limite: 10,
              desplazamiento,
              ...(productoId ? { producto_id: productoId } : {}),
              ...(tipo ? { tipo } : {}),
            },
          },
          signal: AbortSignal.timeout(5000),
        }),
      )
    },
  }
}
