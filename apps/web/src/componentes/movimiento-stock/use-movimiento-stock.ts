import { useMemo, useRef, useState } from 'react'
import { crearApiMovimiento } from './movimiento-stock.api'
import {
  mensajeError,
  type Movimiento,
  type NuevoMovimiento,
  type TipoMovimiento,
} from '../compartido/tipos'

export function useMovimientoStock(apiBaseUrl: string) {
  const api = useMemo(() => crearApiMovimiento(apiBaseUrl), [apiBaseUrl])
  const [tipo, establecerTipo] = useState<TipoMovimiento>('entrada')
  const [producto, establecerProducto] = useState('')
  const [cantidad, establecerCantidad] = useState('')
  const [version, establecerVersion] = useState('')
  const [motivo, establecerMotivo] = useState('')
  const [referencia, establecerReferencia] = useState('')
  const [enviando, establecerEnviando] = useState(false)
  const [error, establecerError] = useState('')
  const [resultado, establecerResultado] = useState<Movimiento | null>(null)
  const intento = useRef<{ huella: string; clave: string } | null>(null)
  const bloqueado = useRef(false)

  async function registrar() {
    if (bloqueado.current) return
    establecerError('')
    establecerResultado(null)
    const unidades = Number(cantidad)
    if (
      !cantidad ||
      !Number.isSafeInteger(unidades) ||
      unidades < (tipo === 'ajuste' ? 0 : 1) ||
      unidades > 1_000_000_000
    ) {
      establecerError('Ingresa una cantidad entera válida dentro del límite permitido.')
      return
    }
    if (
      tipo === 'ajuste' &&
      (!version || !Number.isSafeInteger(Number(version)) || Number(version) < 0)
    ) {
      establecerError('El ajuste necesita la versión actual de la existencia.')
      return
    }
    const datos: NuevoMovimiento = {
      producto_id: producto.trim(),
      tipo,
      cantidad: unidades,
      motivo: motivo.trim(),
      ...(referencia.trim() ? { referencia: referencia.trim() } : {}),
      ...(tipo === 'ajuste' ? { version_esperada: Number(version) } : {}),
    }
    const huella = JSON.stringify(datos)
    if (!intento.current || intento.current.huella !== huella)
      intento.current = { huella, clave: crypto.randomUUID() }
    bloqueado.current = true
    establecerEnviando(true)
    try {
      const movimiento = await api.registrar(datos, intento.current.clave)
      establecerResultado(movimiento)
      intento.current = null
      establecerCantidad('')
      establecerMotivo('')
      establecerReferencia('')
      establecerVersion('')
    } catch (fallo) {
      establecerError(mensajeError(fallo))
    } finally {
      bloqueado.current = false
      establecerEnviando(false)
    }
  }

  return {
    tipo,
    producto,
    cantidad,
    version,
    motivo,
    referencia,
    enviando,
    error,
    resultado,
    establecerTipo,
    establecerProducto,
    establecerCantidad,
    establecerVersion,
    establecerMotivo,
    establecerReferencia,
    registrar,
  }
}
