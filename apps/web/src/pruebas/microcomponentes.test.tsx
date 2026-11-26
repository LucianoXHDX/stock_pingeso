import { act, type ReactNode } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, describe, expect, it } from 'vitest'
import { http, HttpResponse } from 'msw'
import { ConsultaStock } from '../componentes/consulta-stock/consulta-stock'
import { MovimientoStock } from '../componentes/movimiento-stock/movimiento-stock'
import { HistorialStock } from '../componentes/historial-stock/historial-stock'
import { productoDemo } from '../mocks/handlers'
import { servidor } from './preparar'

let raiz: Root | undefined
let contenedor: HTMLDivElement
const propiedades = {
  apiBaseUrl: 'http://localhost',
  permisos: ['stock:consultar', 'stock:operar', 'stock:administrar'],
}

async function montar(vista: ReactNode) {
  contenedor = document.createElement('div')
  document.body.append(contenedor)
  raiz = createRoot(contenedor)
  await act(async () => {
    raiz?.render(vista)
  })
}

async function esperar(condicion: () => boolean) {
  const limite = Date.now() + 4000
  while (!condicion()) {
    if (Date.now() > limite)
      throw new Error(
        `La interfaz no llegó al estado esperado: ${contenedor.textContent}`,
      )
    await act(async () => {
      await new Promise((resolver) => setTimeout(resolver, 10))
    })
  }
}

async function completar(etiqueta: string, valor: string) {
  const label = [...contenedor.querySelectorAll('label')].find((item) =>
    item.textContent?.includes(etiqueta),
  )
  const control =
    label?.querySelector('input, textarea, select') ||
    (label?.htmlFor ? document.getElementById(label.htmlFor) : null)
  if (!(
    control instanceof HTMLInputElement ||
    control instanceof HTMLTextAreaElement ||
    control instanceof HTMLSelectElement
  ))
    throw new Error(`Control no encontrado: ${etiqueta}`)
  const prototipo =
    control instanceof HTMLTextAreaElement
      ? HTMLTextAreaElement.prototype
      : control instanceof HTMLSelectElement
        ? HTMLSelectElement.prototype
        : HTMLInputElement.prototype
  await act(async () => {
    Object.getOwnPropertyDescriptor(prototipo, 'value')?.set?.call(control, valor)
    control.dispatchEvent(
      new Event(control instanceof HTMLSelectElement ? 'change' : 'input', {
        bubbles: true,
      }),
    )
  })
}

async function enviar() {
  const formulario = contenedor.querySelector('form')
  if (!formulario) throw new Error('Formulario no encontrado')
  await act(async () => {
    formulario.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))
  })
}

async function pulsar(texto: string) {
  const boton = [...contenedor.querySelectorAll('button')].find((item) =>
    item.textContent?.includes(texto),
  )
  if (!boton) throw new Error(`Botón no encontrado: ${texto}`)
  await act(async () => {
    boton.click()
  })
}

afterEach(async () => {
  await act(async () => {
    raiz?.unmount()
  })
  contenedor?.remove()
  raiz = undefined
})

describe('microcomponentes autocontenidos de STOCK', () => {
  it('muestra el fallo del proveedor y permite recuperar la consulta sin datos ficticios', async () => {
    servidor.use(
      http.get(
        '*/api/v1/existencias',
        () =>
          HttpResponse.json(
            { detail: 'Stock temporalmente no disponible.' },
            { status: 503 },
          ),
        { once: true },
      ),
    )
    await montar(<ConsultaStock {...propiedades} />)
    await esperar(() => !!contenedor.querySelector('[role="alert"]'))
    expect(contenedor.textContent).toContain('Stock temporalmente no disponible.')
    expect(contenedor.querySelectorAll('tbody tr')).toHaveLength(0)
    await pulsar('Reintentar consulta')
    await esperar(() => contenedor.querySelectorAll('tbody tr').length === 4)
    expect(contenedor.textContent).toContain(productoDemo)
    expect(contenedor.textContent).toContain('128')
  })

  it('rechaza una salida sin saldo y permite corregirla conservando el inventario', async () => {
    await montar(<MovimientoStock {...propiedades} />)
    await act(async () => {
      ;(contenedor.querySelector('input[value="salida"]') as HTMLInputElement).click()
    })
    await completar('Identificador de producto', productoDemo)
    await completar('Cantidad', '129')
    await completar('Motivo', 'Despacho de pedido')
    await enviar()
    await esperar(() => !!contenedor.querySelector('[role="alert"]'))
    expect(contenedor.textContent).toContain('Stock insuficiente')
    await completar('Cantidad', '10')
    await enviar()
    await esperar(
      () => contenedor.textContent?.includes('Movimiento registrado') === true,
    )
    expect(contenedor.textContent).toContain('128 → 118')
  })

  it('reutiliza Idempotency-Key tras perder la respuesta y envía la cookie de sesión', async () => {
    const claves: (string | null)[] = []
    let escrituras = 0
    servidor.use(
      http.post('*/api/v1/movimientos', async ({ request }) => {
        claves.push(request.headers.get('Idempotency-Key'))
        expect(request.credentials).toBe('include')
        if (claves.length === 1) {
          escrituras += 1
          return HttpResponse.error()
        }
        return HttpResponse.json(
          {
            id: 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa',
            producto_id: productoDemo,
            tipo: 'entrada',
            cantidad: 5,
            saldo_anterior: 128,
            saldo_posterior: 133,
            motivo: 'Recepción compra',
            referencia: '',
            actor_id: '55555555-5555-4555-8555-555555555555',
            creado_en: '2026-11-19T14:00:00Z',
          },
          { status: 201 },
        )
      }),
    )
    await montar(<MovimientoStock {...propiedades} />)
    await completar('Identificador de producto', productoDemo)
    await completar('Cantidad', '5')
    await completar('Motivo', 'Recepción compra')
    await enviar()
    await esperar(() => !!contenedor.querySelector('[role="alert"]'))
    await enviar()
    await esperar(
      () => contenedor.textContent?.includes('Movimiento registrado') === true,
    )
    expect(claves).toHaveLength(2)
    expect(claves[0]).toBeTruthy()
    expect(claves[1]).toBe(claves[0])
    expect(escrituras).toBe(1)
  })

  it('explica el conflicto de versión de un ajuste y acepta la versión vigente', async () => {
    await montar(<MovimientoStock {...propiedades} />)
    await act(async () => {
      ;(contenedor.querySelector('input[value="ajuste"]') as HTMLInputElement).click()
    })
    await completar('Identificador de producto', productoDemo)
    await completar('Saldo final', '100')
    await completar('Versión actual', '2')
    await completar('Motivo', 'Conteo físico de bodega')
    await enviar()
    await esperar(() => !!contenedor.querySelector('[role="alert"]'))
    expect(contenedor.textContent).toContain('La existencia cambió')
    await completar('Versión actual', '3')
    await enviar()
    await esperar(
      () => contenedor.textContent?.includes('Movimiento registrado') === true,
    )
    expect(contenedor.textContent).toContain('128 → 100')
  })

  it('filtra el historial por producto y tipo y presenta un estado vacío explícito', async () => {
    await montar(<HistorialStock {...propiedades} />)
    await esperar(() => contenedor.querySelectorAll('tbody tr').length === 1)
    await completar('Producto', productoDemo)
    await completar('Tipo', 'salida')
    await enviar()
    await esperar(
      () => contenedor.textContent?.includes('Sin movimientos para mostrar') === true,
    )
    expect(contenedor.textContent).toContain('0 movimientos registrados')
  })

  it('no consulta el servicio sin el permiso necesario', async () => {
    let solicitudes = 0
    servidor.use(
      http.get('*/api/v1/existencias', () => {
        solicitudes += 1
        return HttpResponse.json({ datos: [], total: 0 })
      }),
    )
    await montar(<ConsultaStock apiBaseUrl="http://localhost" permisos={[]} />)
    expect(contenedor.textContent).toContain('Necesitas el permiso stock:consultar')
    expect(solicitudes).toBe(0)
  })
})
