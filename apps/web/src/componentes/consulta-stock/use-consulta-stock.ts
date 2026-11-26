import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { crearApiConsulta } from './consulta-stock.api'
import { mensajeError, type Existencia } from '../compartido/tipos'

export function useConsultaStock(apiBaseUrl: string, autorizado: boolean) {
  const api = useMemo(() => crearApiConsulta(apiBaseUrl), [apiBaseUrl])
  const [existencias, establecerExistencias] = useState<Existencia[]>([])
  const [total, establecerTotal] = useState(0)
  const [desplazamiento, establecerDesplazamiento] = useState(0)
  const [cargando, establecerCargando] = useState(false)
  const [error, establecerError] = useState('')
  const [producto, establecerProducto] = useState('')
  const [consulta, establecerConsulta] = useState('')
  const generacion = useRef(0)

  const cargar = useCallback(async () => {
    if (!autorizado) return
    const solicitud = ++generacion.current
    establecerCargando(true)
    establecerError('')
    try {
      if (consulta) {
        const existencia = await api.consultar(consulta)
        if (solicitud !== generacion.current) return
        establecerExistencias([existencia])
        establecerTotal(1)
      } else {
        const pagina = await api.listar(desplazamiento)
        if (solicitud !== generacion.current) return
        establecerExistencias(pagina.datos ?? [])
        establecerTotal(pagina.total)
      }
    } catch (fallo) {
      if (solicitud !== generacion.current) return
      establecerError(mensajeError(fallo))
      establecerExistencias([])
      establecerTotal(0)
    } finally {
      if (solicitud === generacion.current) establecerCargando(false)
    }
  }, [api, autorizado, consulta, desplazamiento])

  useEffect(() => {
    void cargar()
    return () => {
      generacion.current += 1
    }
  }, [cargar])

  function buscar() {
    establecerDesplazamiento(0)
    establecerConsulta(producto.trim())
    if (consulta === producto.trim()) void cargar()
  }

  return {
    existencias,
    total,
    desplazamiento,
    cargando,
    error,
    producto,
    consulta,
    establecerProducto,
    buscar,
    cargar,
    establecerDesplazamiento,
  }
}
