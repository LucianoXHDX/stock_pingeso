import { http, HttpResponse } from 'msw'
import type {
  Existencia,
  Movimiento,
  NuevoMovimiento,
} from '../componentes/compartido/tipos'

const fechaDemo = '2026-11-19T14:00:00Z'
export const productoDemo = '11111111-1111-4111-8111-111111111111'
const existenciasIniciales: Existencia[] = [
  { producto_id: productoDemo, cantidad: 128, version: 3, actualizado_en: fechaDemo },
  {
    producto_id: '22222222-2222-4222-8222-222222222222',
    cantidad: 42,
    version: 1,
    actualizado_en: fechaDemo,
  },
  {
    producto_id: '33333333-3333-4333-8333-333333333333',
    cantidad: 0,
    version: 2,
    actualizado_en: fechaDemo,
  },
  {
    producto_id: '44444444-4444-4444-8444-444444444444',
    cantidad: 16,
    version: 1,
    actualizado_en: fechaDemo,
  },
]
let existencias: Existencia[] = []
let movimientos: Movimiento[] = []
const idempotencia = new Map<string, { huella: string; movimiento: Movimiento }>()

export function reiniciarDemo() {
  existencias = structuredClone(existenciasIniciales)
  movimientos = [
    {
      id: 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa',
      producto_id: productoDemo,
      tipo: 'entrada',
      cantidad: 128,
      saldo_anterior: 0,
      saldo_posterior: 128,
      motivo: 'Recepción inicial de inventario',
      referencia: 'OC-DEMO-001',
      actor_id: '55555555-5555-4555-8555-555555555555',
      creado_en: fechaDemo,
    },
  ]
  idempotencia.clear()
}
reiniciarDemo()

function problema(status: number, detail: string) {
  return HttpResponse.json(
    { type: 'about:blank', title: 'Operación rechazada', status, detail },
    { status, headers: { 'Content-Type': 'application/problem+json' } },
  )
}

export const handlers = [
  http.get('*/api/v1/existencias', ({ request }) => {
    const url = new URL(request.url)
    const desde = Number(url.searchParams.get('desplazamiento') || 0)
    const limite = Number(url.searchParams.get('limite') || 10)
    return HttpResponse.json({
      datos: existencias.slice(desde, desde + limite),
      total: existencias.length,
    })
  }),
  http.get('*/api/v1/existencias/:producto_id', ({ params }) => {
    const existencia = existencias.find((item) => item.producto_id === params.producto_id)
    return existencia
      ? HttpResponse.json(existencia)
      : problema(404, 'No hay existencias registradas para ese producto.')
  }),
  http.get('*/api/v1/movimientos', ({ request }) => {
    const url = new URL(request.url)
    const producto = url.searchParams.get('producto_id')
    const tipo = url.searchParams.get('tipo')
    const filtrados = movimientos.filter(
      (item) =>
        (!producto || item.producto_id === producto) && (!tipo || item.tipo === tipo),
    )
    const desde = Number(url.searchParams.get('desplazamiento') || 0)
    const limite = Number(url.searchParams.get('limite') || 10)
    return HttpResponse.json({
      datos: filtrados.slice(desde, desde + limite),
      total: filtrados.length,
    })
  }),
  http.post('*/api/v1/movimientos', async ({ request }) => {
    const datos = (await request.json()) as NuevoMovimiento
    const clave = request.headers.get('Idempotency-Key')
    if (!clave) return problema(422, 'Falta la clave de idempotencia.')
    const repetido = idempotencia.get(clave)
    if (repetido)
      return repetido.huella === JSON.stringify(datos)
        ? HttpResponse.json(repetido.movimiento, { status: 201 })
        : problema(409, 'La clave ya se utilizó para otro movimiento.')
    if (
      !Number.isInteger(datos.cantidad) ||
      datos.cantidad < (datos.tipo === 'ajuste' ? 0 : 1)
    )
      return problema(422, 'Cantidad inválida.')
    const existencia = existencias.find((item) => item.producto_id === datos.producto_id)
    if (!existencia)
      return problema(404, 'El catálogo de demostración no contiene ese producto.')
    if (datos.tipo === 'ajuste' && datos.version_esperada !== existencia.version)
      return problema(
        409,
        'La existencia cambió. Consulta su versión actual antes de ajustar.',
      )
    const saldo =
      datos.tipo === 'ajuste'
        ? datos.cantidad
        : existencia.cantidad +
          (datos.tipo === 'salida' ? -datos.cantidad : datos.cantidad)
    if (saldo < 0) return problema(409, 'Stock insuficiente para registrar esta salida.')
    const movimiento: Movimiento = {
      id: crypto.randomUUID(),
      producto_id: datos.producto_id,
      tipo: datos.tipo,
      cantidad:
        datos.tipo === 'ajuste' ? Math.abs(saldo - existencia.cantidad) : datos.cantidad,
      saldo_anterior: existencia.cantidad,
      saldo_posterior: saldo,
      motivo: datos.motivo,
      referencia: datos.referencia ?? '',
      actor_id: '55555555-5555-4555-8555-555555555555',
      creado_en: fechaDemo,
    }
    existencia.cantidad = saldo
    existencia.version += 1
    existencia.actualizado_en = fechaDemo
    movimientos.unshift(movimiento)
    idempotencia.set(clave, { huella: JSON.stringify(datos), movimiento })
    return HttpResponse.json(movimiento, { status: 201 })
  }),
]
