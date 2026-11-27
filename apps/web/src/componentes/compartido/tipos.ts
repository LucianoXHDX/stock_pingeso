import type { paths } from '../../generado/api'

export type PropiedadesStock = {
  apiBaseUrl: string
  permisos: readonly string[]
  usuario?: { id: string; nombre: string }
}

export type Existencia =
  paths['/api/v1/existencias/{producto_id}']['get']['responses'][200]['content']['application/json']
export type Movimiento =
  paths['/api/v1/movimientos']['post']['responses'][201]['content']['application/json']
export type NuevoMovimiento =
  paths['/api/v1/movimientos']['post']['requestBody']['content']['application/json']
export type TipoMovimiento = NuevoMovimiento['tipo']

export function mensajeError(error: unknown): string {
  if (
    error instanceof Error &&
    (error.name === 'TimeoutError' || error.name === 'AbortError')
  )
    return 'El servicio tardó demasiado en responder. Reintenta la operación.'
  if (error instanceof TypeError)
    return 'No se pudo conectar con el servicio de stock. Revisa la conexión y vuelve a intentarlo.'
  if (error instanceof Error) return error.message
  return 'No se pudo completar la operación. Vuelve a intentarlo.'
}

export function exigirRespuesta<T>(resultado: {
  data?: T
  error?: unknown
  response: Response
}): T {
  if (resultado.data !== undefined) return resultado.data
  const problema = resultado.error as { detail?: string; title?: string } | undefined
  if (resultado.response.status === 401)
    throw new Error(
      'Tu sesión no está disponible o expiró. Inicia sesión en tu aplicación e inténtalo otra vez.',
    )
  if (resultado.response.status === 403)
    throw new Error('No tienes permiso para realizar esta operación.')
  throw new Error(
    problema?.detail ||
      problema?.title ||
      'No se pudo contactar al servicio de stock. Vuelve a intentarlo.',
  )
}

export function fechaLegible(fecha: string): string {
  return new Intl.DateTimeFormat('es-CL', {
    dateStyle: 'medium',
    timeStyle: 'short',
    timeZone: 'America/Santiago',
  }).format(new Date(fecha))
}

export function nombreTipo(tipo: TipoMovimiento): string {
  return { entrada: 'Entrada', salida: 'Salida', ajuste: 'Ajuste' }[tipo]
}
