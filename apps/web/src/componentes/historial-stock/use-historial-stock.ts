import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { crearApiHistorial } from './historial-stock.api'
import { mensajeError, type Movimiento, type TipoMovimiento } from '../compartido/tipos'

export function useHistorialStock(apiBaseUrl: string, autorizado: boolean) {
  const api = useMemo(() => crearApiHistorial(apiBaseUrl), [apiBaseUrl])
  const [movimientos, establecerMovimientos] = useState<Movimiento[]>([])
  const [total, establecerTotal] = useState(0)
  const [producto, establecerProducto] = useState('')
  const [tipo, establecerTipo] = useState<TipoMovimiento | ''>('')
  const [filtros, establecerFiltros] = useState({
    producto: '',
    tipo: '' as TipoMovimiento | '',
  })
  const [desplazamiento, establecerDesplazamiento] = useState(0)
  const [cargando, establecerCargando] = useState(false)
  const [error, establecerError] = useState('')
  const generacion = useRef(0)
  const cargar = useCallback(async () => {
    if (!autorizado) return
    const solicitud = ++generacion.current
    establecerCargando(true)
    establecerError('')
    try {
      const pagina = await api.listar(filtros.producto, filtros.tipo, desplazamiento)
      if (solicitud !== generacion.current) return
      establecerMovimientos(pagina.datos ?? [])
      establecerTotal(pagina.total)
    } catch (fallo) {
      if (solicitud !== generacion.current) return
      establecerError(mensajeError(fallo))
      establecerMovimientos([])
      establecerTotal(0)
    } finally {
      if (solicitud === generacion.current) establecerCargando(false)
    }
  }, [api, autorizado, filtros, desplazamiento])
  useEffect(() => {
    void cargar()
    return () => {
      generacion.current += 1
    }
  }, [cargar])
  function filtrar() {
    establecerDesplazamiento(0)
    establecerFiltros({ producto: producto.trim(), tipo })
  }
  return {
    movimientos,
    total,
    producto,
    tipo,
    desplazamiento,
    cargando,
    error,
    cargar,
    filtrar,
    establecerProducto,
    establecerTipo,
    establecerDesplazamiento,
  }
}
